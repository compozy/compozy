// Package packaging holds contract tests for the cy-capture-decisions extension's
// packaging surface: the skill-only extension.toml manifest and the extension-root
// README's documented conventions (gitignore durability negations, the @import
// consumption wiring, and the skill install path). The extension ships no runtime
// logic of its own here; these tests guard the documentation and packaging contract
// under `make verify` (go test ./...), alongside the decisionlog format validator.
package packaging
