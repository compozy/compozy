// Package schema embeds the workspace database schema and migration directory.
package schema

import "embed"

// Files contains the declarative schema, SQL migrations, and atlas.sum.
//
//go:embed definitions/*.sql migrations/*.sql migrations/atlas.sum
var Files embed.FS
