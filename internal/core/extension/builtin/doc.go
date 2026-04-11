// Package builtin anchors the embedded extension discovery root.
//
// The directory intentionally contains no first-party extensions in v1. The
// discovery pipeline still embeds this directory so later releases can ship
// bundled extensions without changing the loading mechanism.
package builtin
