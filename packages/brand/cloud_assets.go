package brand

import "embed"

// CloudAssets carries the Atmosphere crops and wordmark for Cloud sign-in and billing pages.
//
//go:embed public/brand/visuals/swarm-1280.jpg public/brand/visuals/swarm-portrait-1280.jpg
//go:embed public/brand/visuals/cone-1280.jpg public/brand/visuals/cone-portrait-1280.jpg
//go:embed public/brand/visuals/converge-1280.jpg public/brand/visuals/converge-portrait-1280.jpg
//go:embed public/brand/svg/compozy-wordmark-white.svg
var CloudAssets embed.FS
