package brand

import "embed"

// CloudAssets carries the same Atmosphere crops and wordmark as the sign-in prototypes.
//
//go:embed public/brand/visuals/swarm-1280.jpg public/brand/visuals/swarm-portrait-1280.jpg public/brand/svg/compozy-wordmark-white.svg
var CloudAssets embed.FS
