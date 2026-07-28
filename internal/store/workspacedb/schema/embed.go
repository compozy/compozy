// Package schema embeds the workspace database schema and migration directory.
package schema

import "embed"

// Files contains schema.sql, SQL migrations, and atlas.sum.
//
//go:embed schema.sql migrations/*.sql migrations/atlas.sum
var Files embed.FS
