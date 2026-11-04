// Package schema embeds the telemetry schema and its independent migration history.
package schema

import "embed"

//go:embed schema.sql migrations/*.sql migrations/atlas.sum
var Files embed.FS
