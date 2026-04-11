package extensions

import (
	"embed"
	"fmt"
	"io/fs"
)

const bundledExtensionsDir = "builtin"

//go:embed builtin
var bundledExtensionsEmbedFS embed.FS

func defaultBundledExtensionsFS() fs.FS {
	root, err := fs.Sub(bundledExtensionsEmbedFS, bundledExtensionsDir)
	if err != nil {
		panic(fmt.Sprintf("load bundled extension filesystem: %v", err))
	}
	return root
}
