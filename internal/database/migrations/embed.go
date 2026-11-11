package migrations

import "embed"

// Files contains versioned PostgreSQL schema migrations.
//
//go:embed *.sql
var Files embed.FS
