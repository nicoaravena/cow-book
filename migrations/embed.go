// Package migrations holds the goose SQL migrations, embedded in the binary so
// it can provision tenants without the files on disk.
package migrations

import (
	"embed"
	"io/fs"
)

//go:embed platform/*.sql tenant/*.sql
var files embed.FS

// Platform holds the migrations run once per database.
var Platform = sub("platform")

// Tenant holds the migrations run in every tenant schema.
var Tenant = sub("tenant")

// sub returns the files under dir, named without the dir prefix, which is how
// goose expects to find them.
func sub(dir string) fs.FS {
	f, err := fs.Sub(files, dir)
	if err != nil {
		panic(err) // dir is a constant above, so this can't happen at run time
	}
	return f
}
