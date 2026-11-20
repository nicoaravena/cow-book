// Package postgres stores cowbook data in Postgres, one schema per tenant.
package postgres

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

// ErrTenantNotFound means no tenant has the slug.
var ErrTenantNotFound = errors.New("tenant not found")

// LookupTenant returns the id of the tenant with slug. It returns
// ErrTenantNotFound if there is none, and an error naming the status if the
// tenant isn't active: a provisioning one may have no schema yet, and a
// suspended one must not be used.
func LookupTenant(ctx context.Context, pool *pgxpool.Pool, slug string) (int64, error) {
	var id int64
	var status string
	err := pool.QueryRow(ctx,
		"SELECT id, status FROM platform.tenants WHERE slug = $1", slug).Scan(&id, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, fmt.Errorf("%w: %q", ErrTenantNotFound, slug)
	}
	if err != nil {
		return 0, fmt.Errorf("look up tenant: %w", err)
	}
	if status != "active" {
		return 0, fmt.Errorf("tenant %q is %s, not active", slug, status)
	}
	return id, nil
}

// SchemaName returns the name of tenant id's schema. It is built only from
// the numeric id, never from the slug or any other user input.
func SchemaName(id int64) string {
	return "tenant_" + strconv.FormatInt(id, 10)
}

// Provisioner creates tenants: a row in platform.tenants, a schema, and the
// tenant migrations applied in it.
type Provisioner struct {
	pool       *pgxpool.Pool
	migrations fs.FS
}

// NewProvisioner returns a Provisioner that connects through pool and
// applies the goose migrations in migrations to each new schema.
func NewProvisioner(pool *pgxpool.Pool, migrations fs.FS) *Provisioner {
	return &Provisioner{pool: pool, migrations: migrations}
}

// CreateTenant adds a tenant and returns its id. The row starts as
// provisioning and becomes active once its schema is migrated. If any step
// after the insert fails, the schema and the row are removed, so a failed
// signup leaves nothing behind and its slug can be used again.
func (p *Provisioner) CreateTenant(ctx context.Context, slug, name string) (int64, error) {
	var id int64
	err := p.pool.QueryRow(ctx,
		"INSERT INTO platform.tenants (slug, name) VALUES ($1, $2) RETURNING id",
		slug, name).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("insert tenant: %w", err)
	}

	if err := p.provision(ctx, id); err != nil {
		// Clean up even if ctx was cancelled, which may be why provision failed.
		cleanupErr := p.remove(context.WithoutCancel(ctx), id)
		return 0, errors.Join(err, cleanupErr)
	}
	return id, nil
}

// provision creates and migrates tenant id's schema, then marks it active.
func (p *Provisioner) provision(ctx context.Context, id int64) error {
	schema := pgx.Identifier{SchemaName(id)}.Sanitize()
	if _, err := p.pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		return fmt.Errorf("create schema: %w", err)
	}
	if err := p.migrate(ctx, id); err != nil {
		return fmt.Errorf("migrate %s: %w", SchemaName(id), err)
	}
	_, err := p.pool.Exec(ctx, "UPDATE platform.tenants SET status = 'active' WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("activate tenant: %w", err)
	}
	return nil
}

// migrate applies the tenant migrations in tenant id's schema. goose works
// on a database/sql *sql.DB, so this opens one through pgx's stdlib adapter,
// with the tenant's schema first on its search_path. goose then creates its
// version table, and every unqualified table, in that schema. The schema
// must already exist: Postgres skips missing schemas on the search_path.
func (p *Provisioner) migrate(ctx context.Context, id int64) error {
	cfg := p.pool.Config().ConnConfig.Copy()
	cfg.RuntimeParams["search_path"] = pgx.Identifier{SchemaName(id)}.Sanitize() + ", shared"
	db := stdlib.OpenDB(*cfg)
	defer db.Close()

	provider, err := goose.NewProvider(goose.DialectPostgres, db, p.migrations)
	if err != nil {
		return err
	}
	_, err = provider.Up(ctx)
	return err
}

// remove drops tenant id's schema, with everything in it, and its row.
func (p *Provisioner) remove(ctx context.Context, id int64) error {
	schema := pgx.Identifier{SchemaName(id)}.Sanitize()
	if _, err := p.pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE"); err != nil {
		return fmt.Errorf("clean up: drop schema: %w", err)
	}
	if _, err := p.pool.Exec(ctx, "DELETE FROM platform.tenants WHERE id = $1", id); err != nil {
		return fmt.Errorf("clean up: delete tenant: %w", err)
	}
	return nil
}
