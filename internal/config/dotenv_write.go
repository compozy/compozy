package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/compozy/compozy/internal/fileutil"
)

func replaceDotEnvFile(
	directory *fileutil.Directory,
	name string,
	path string,
	contents []byte,
	mode os.FileMode,
) error {
	mode = secureDotEnvWriteMode(mode)
	if err := directory.AtomicWriteFile(name, contents, mode, true); err != nil {
		return fmt.Errorf("replace .env file %q: %w", path, err)
	}
	return nil
}

func secureDotEnvWriteMode(mode os.FileMode) os.FileMode {
	if mode == 0 || mode.Perm()&0o077 != 0 {
		return 0o600
	}
	return mode.Perm()
}

func dotEnvUnsupportedError(path string, diagnostics []DotEnvDiagnostic) error {
	return &DotEnvRepairError{
		Path:        path,
		Diagnostics: append([]DotEnvDiagnostic(nil), diagnostics...),
	}
}

// Error returns a diagnostic summary without including .env values.
func (e *DotEnvRepairError) Error() string {
	if e == nil {
		return ErrDotEnvUnsupported.Error()
	}
	parts := make([]string, 0, len(e.Diagnostics))
	for _, diagnostic := range e.Diagnostics {
		location := ""
		if diagnostic.Line > 0 {
			location = fmt.Sprintf("line %d", diagnostic.Line)
		}
		if diagnostic.Key != "" {
			if location != "" {
				location += " "
			}
			location += "key " + diagnostic.Key
		}
		if location == "" {
			location = string(capabilityCatalogLayoutModeFile)
		}
		parts = append(parts, fmt.Sprintf("%s: %s", location, diagnostic.Message))
	}
	if len(parts) == 0 {
		return fmt.Sprintf("%s in %q", ErrDotEnvUnsupported, e.Path)
	}
	return fmt.Sprintf("%s in %q (%s)", ErrDotEnvUnsupported, e.Path, strings.Join(parts, "; "))
}

// Is matches the unsupported .env sentinel.
func (e *DotEnvRepairError) Is(target error) bool {
	return target == ErrDotEnvUnsupported
}
