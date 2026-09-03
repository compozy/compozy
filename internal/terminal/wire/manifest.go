package wire

import _ "embed"

// ProtocolManifest is the authoritative terminal wire manifest embedded for code generation.
//
//go:embed protocol.json
var ProtocolManifest []byte
