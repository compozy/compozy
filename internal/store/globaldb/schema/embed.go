// Package schema embeds the global database schema and migration directory.
package schema

import "embed"

// Files contains the declarative schema, fresh seed, SQL migrations, and atlas.sum.
//
//go:embed bootstrap.sql definitions/*.sql migrations/*.sql migrations/atlas.sum
var Files embed.FS
