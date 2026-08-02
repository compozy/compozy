package fileutil

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

var portableDOSDeviceNames = map[string]struct{}{
	"AUX":     {},
	"CLOCK$":  {},
	"CON":     {},
	"CONIN$":  {},
	"CONOUT$": {},
	"NUL":     {},
	"PRN":     {},
}

// ValidatePathComponent accepts exactly one portable filename component.
// It rejects names whose interpretation differs across supported platforms.
func ValidatePathComponent(component string) error {
	if component == "" || strings.TrimSpace(component) == "" || component == "." || component == ".." {
		return fmt.Errorf("%w: path component is required", ErrInvalidPath)
	}
	if !utf8.ValidString(component) {
		return fmt.Errorf("%w: path component is not valid UTF-8", ErrInvalidPath)
	}
	if strings.HasSuffix(component, ".") || strings.HasSuffix(component, " ") {
		return fmt.Errorf("%w: path component has a forbidden suffix", ErrInvalidPath)
	}

	for _, character := range component {
		if unicode.IsControl(character) || strings.ContainsRune(`:/\\<>"|?*`, character) {
			return fmt.Errorf("%w: path component contains a forbidden character", ErrInvalidPath)
		}
	}

	stem := strings.ToUpper(component)
	if extension := strings.IndexByte(stem, '.'); extension >= 0 {
		stem = stem[:extension]
	}
	stem = strings.TrimRight(stem, " ")
	if _, reserved := portableDOSDeviceNames[stem]; reserved || isPortableDOSSerialDevice(stem) {
		return fmt.Errorf("%w: path component is a reserved DOS device", ErrInvalidPath)
	}
	return nil
}

func isPortableDOSSerialDevice(stem string) bool {
	runes := []rune(stem)
	if len(runes) != 4 || (string(runes[:3]) != "COM" && string(runes[:3]) != "LPT") {
		return false
	}
	return (runes[3] >= '1' && runes[3] <= '9') || runes[3] == '¹' || runes[3] == '²' || runes[3] == '³'
}
