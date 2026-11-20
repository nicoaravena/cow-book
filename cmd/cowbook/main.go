// Command cowbook is the coworking booking app. It grows with each step of
// the roadmap and ends as the full app. Its subcommands:
//
//	cowbook repl [-store=memory|postgres] [-tenant=<slug>]
//	                                     a command prompt over the services and a store
//	cowbook create-tenant <slug> <name>  add an operator with its own schema, in DATABASE_URL
//
// Exit codes: 0 ok, 1 an error, 2 a usage error.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"nicoaravena/cow-book-app/cowbook"
	"nicoaravena/cow-book-app/internal/memstore"
	"nicoaravena/cow-book-app/internal/postgres"
	"nicoaravena/cow-book-app/internal/repl"
	"nicoaravena/cow-book-app/migrations"
)

const usage = `usage: cowbook <command>

commands:
  repl [-store=memory|postgres] [-tenant=<slug>]
                               a command prompt over the services and a store;
                               postgres keeps the tenant's data in DATABASE_URL
  create-tenant <slug> <name>  add an operator with its own schema, in DATABASE_URL`

// errUsage is returned for bad arguments, which exit with code 2. The message
// has been printed already.
var errUsage = errors.New("usage")

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, usage)
		os.Exit(2)
	}

	var err error
	switch os.Args[1] {
	case "repl":
		err = runREPL(os.Args[2:])
	case "create-tenant":
		if len(os.Args) != 4 {
			fmt.Fprintln(os.Stderr, usage)
			os.Exit(2)
		}
		err = runCreateTenant(os.Args[2], os.Args[3])
	default:
		fmt.Fprintf(os.Stderr, "cowbook: unknown command %q\n%s\n", os.Args[1], usage)
		os.Exit(2)
	}
	if errors.Is(err, errUsage) {
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "cowbook:", err)
		os.Exit(1)
	}
}

// replStore is what runREPL builds a session on: the repl package's Store
// plus the three interfaces the services take. Both stores meet it.
type replStore interface {
	repl.Store
	cowbook.BookingStore
	cowbook.SpaceStore
	cowbook.MemberStore
}

// runREPL parses the repl flags, opens the store they name and runs a
// session on it. flag.NewFlagSet gives the subcommand its own flags; 3.2
// moves every subcommand to it.
func runREPL(args []string) error {
	fs := flag.NewFlagSet("repl", flag.ContinueOnError)
	storeName := fs.String("store", "memory", "where data lives: memory, gone when the REPL ends, or postgres")
	tenant := fs.String("tenant", "", "the operator's slug, with -store=postgres")
	// Parse prints the error and the flags itself. -h asks for the flags and
	// isn't an error.
	if err := fs.Parse(args); errors.Is(err, flag.ErrHelp) {
		return nil
	} else if err != nil {
		return errUsage
	}
	if fs.NArg() > 0 {
		return usageError(fs, "unexpected argument %q", fs.Arg(0))
	}

	ctx := context.Background()
	var store replStore
	switch {
	case *storeName == "memory" && *tenant == "":
		store = memstore.New()
	case *storeName == "memory":
		return usageError(fs, "-tenant needs -store=postgres")
	case *storeName == "postgres" && *tenant == "":
		return usageError(fs, "-store=postgres needs -tenant=<slug>")
	case *storeName == "postgres":
		pool, err := openPool(ctx)
		if err != nil {
			return err
		}
		defer pool.Close()
		id, err := postgres.LookupTenant(ctx, pool, *tenant)
		if err != nil {
			return err
		}
		store = postgres.NewStores(pool, id)
	default:
		return usageError(fs, "unknown store %q, want memory or postgres", *storeName)
	}

	// A fixed start, Monday 16 November 2026 08:00 UTC, so a session prints
	// the same every run. With -store=postgres it starts there again after a
	// restart, while the data stays.
	clock := repl.NewClock(time.Date(2026, time.November, 16, 8, 0, 0, 0, time.UTC))
	bookings := cowbook.NewBookingService(store, store, store, clock.Now)
	members := cowbook.NewMemberService(store, store, clock.Now)
	session := repl.NewSession(store, bookings, members, clock)

	// Input from a terminal is a person typing; anything else, such as a
	// file redirected with <, is a script.
	info, err := os.Stdin.Stat()
	script := err == nil && info.Mode()&os.ModeCharDevice == 0

	return session.Run(ctx, os.Stdin, os.Stdout, script)
}

// usageError prints the problem and the flags of fs to its output, stderr,
// and returns errUsage.
func usageError(fs *flag.FlagSet, format string, a ...any) error {
	fmt.Fprintf(fs.Output(), format+"\n", a...)
	fs.Usage()
	return errUsage
}

// openPool connects to DATABASE_URL.
func openPool(ctx context.Context) (*pgxpool.Pool, error) {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		return nil, errors.New("DATABASE_URL is not set")
	}
	return postgres.NewPool(ctx, url)
}

func runCreateTenant(slug, name string) error {
	ctx := context.Background()
	pool, err := openPool(ctx)
	if err != nil {
		return err
	}
	defer pool.Close()

	id, err := postgres.NewProvisioner(pool, migrations.Tenant).CreateTenant(ctx, slug, name)
	if err != nil {
		return err
	}
	fmt.Printf("created tenant %d (%s) in schema %s\n", id, slug, postgres.SchemaName(id))
	return nil
}
