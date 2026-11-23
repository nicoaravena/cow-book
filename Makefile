.PHONY: test test-integration lint generate docs docs-serve fetch-catppuccin db-up db-down db-psql migrate-platform migrate-check migrate-tenant create-tenant serve

# .env is optional: test, lint and docs don't need it. Its variables are
# exported to every recipe, so goose sees DATABASE_URL.
-include .env
export

GOOSE_PLATFORM = go tool goose -dir migrations/platform postgres "$$DATABASE_URL"

test:
	go test -race ./...

# Needs the compose database with the platform migration: make db-up migrate-platform.
test-integration:
	go test -race -tags integration ./...

lint:
	go vet -tags integration ./...
	@unformatted="$$(gofmt -l .)"; \
	if [ -n "$$unformatted" ]; then \
		echo "gofmt needed on:"; echo "$$unformatted"; exit 1; \
	fi
	go tool staticcheck -tags integration ./...
	go tool sqlc diff

# Regenerates internal/postgres/tenantdb from sqlc.yaml, the queries and the
# tenant migrations. The output is committed; never edit it by hand.
generate:
	go tool sqlc generate

docs:
	mdbook build docs

docs-serve:
	mdbook serve --open docs

fetch-catppuccin:
	wget -O ./docs/theme/catppuccin.css https://github.com/catppuccin/mdBook/releases/latest/download/catppuccin.css

db-up:
	docker compose up -d --wait

db-down:
	docker compose down

db-psql:
	docker compose exec db psql -U cowbook -d cowbook

migrate-platform:
	$(GOOSE_PLATFORM) up
	$(GOOSE_PLATFORM) status

# Up, all the way down, and up again: proves every Down migration works. It
# runs in a throwaway database: in the dev one, reset would drop the tenants
# table, and tenant schemas using btree_gist refuse it. The database URL
# parameter overrides the database named in DATABASE_URL.
# Tenant migrations get the same check in TestTenantMigrationsDownAndUp.
MIGRATE_CHECK_DB = cowbook_migrate_check
GOOSE_CHECK = go tool goose -dir migrations/platform postgres "$$DATABASE_URL&database=$(MIGRATE_CHECK_DB)"

migrate-check:
	docker compose exec -T db dropdb -U cowbook --if-exists $(MIGRATE_CHECK_DB)
	docker compose exec -T db createdb -U cowbook $(MIGRATE_CHECK_DB)
	$(GOOSE_CHECK) up
	$(GOOSE_CHECK) reset
	$(GOOSE_CHECK) up
	docker compose exec -T db dropdb -U cowbook $(MIGRATE_CHECK_DB)

# Brings an existing tenant schema up to the latest tenant migration. New
# tenants are migrated by create-tenant; migrating them all is 5.5.
# pgx passes the unknown search_path URL parameter on to Postgres.
# make migrate-tenant TENANT=tenant_1
migrate-tenant:
	@case "$(TENANT)" in tenant_[0-9]*) ;; *) echo "usage: make migrate-tenant TENANT=tenant_<id>"; exit 2;; esac
	go tool goose -dir migrations/tenant postgres "$$DATABASE_URL&search_path=$(TENANT),shared" up
	go tool goose -dir migrations/tenant postgres "$$DATABASE_URL&search_path=$(TENANT),shared" status

# make create-tenant SLUG=north-hub NAME="North Hub"
create-tenant:
	go run ./cmd/cowbook create-tenant "$(SLUG)" "$(NAME)"

# The HTTP API on COWBOOK_ADDR (default localhost:8080) until Ctrl-C. Needs the
# compose database: make db-up migrate-platform.
serve:
	go run ./cmd/cowbook serve
