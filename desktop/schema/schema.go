// Package appschema embeds the canonical cross-language desktop app state schema.
package appschema

import _ "embed"

// Definition is the canonical schema shared by the Rust writer and Go reader.
//
//go:embed app-state.schema.json
var Definition []byte
