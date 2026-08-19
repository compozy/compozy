package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCmdPaletteConfig(t *testing.T) {
	t.Parallel()

	t.Run("Should provide valid shipped defaults", func(t *testing.T) {
		t.Parallel()

		got := DefaultCmdPaletteConfig()
		if err := got.Validate(); err != nil {
			t.Fatalf("Validate() error = %v", err)
		}
		if len(got.FallbackTargets) != 1 || got.FallbackTargets[0] != CmdPaletteFallbackAgent {
			t.Fatalf("FallbackTargets = %#v, want [agent]", got.FallbackTargets)
		}
		if !got.Personalization {
			t.Fatal("Personalization = false, want true")
		}
		if got.Aliases == nil {
			t.Fatal("Aliases = nil, want initialized map")
		}
	})

	t.Run("Should reject invalid aliases with their config path", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name  string
			alias string
		}{
			{name: "Should reject whitespace", alias: "my alias"},
			{name: "Should reject more than thirty two characters", alias: strings.Repeat("a", 33)},
		}
		for _, testCase := range tests {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				cfg := DefaultCmdPaletteConfig()
				cfg.Aliases["session.new"] = testCase.alias
				err := cfg.Validate()
				var validationErr ValidationError
				if !errors.As(err, &validationErr) {
					t.Fatalf("Validate() error = %T %v, want ValidationError", err, err)
				}
				if validationErr.Path != `cmd_palette.aliases["session.new"]` {
					t.Fatalf("ValidationError.Path = %q, want alias path", validationErr.Path)
				}
			})
		}
	})

	t.Run("Should reject unknown fallback targets", func(t *testing.T) {
		t.Parallel()

		cfg := DefaultCmdPaletteConfig()
		cfg.FallbackTargets = []string{"telegram"}
		err := cfg.Validate()
		var validationErr ValidationError
		if !errors.As(err, &validationErr) {
			t.Fatalf("Validate() error = %T %v, want ValidationError", err, err)
		}
		if validationErr.Path != "cmd_palette.fallback_targets[0]" ||
			!strings.Contains(validationErr.Message, "agent") {
			t.Fatalf("ValidationError = %#v, want allowed target in indexed path", validationErr)
		}
	})

	t.Run("Should report duplicate alias ownership deterministically", func(t *testing.T) {
		t.Parallel()

		cfg := DefaultCmdPaletteConfig()
		cfg.Aliases = map[string]string{
			"session.new":  "go",
			"palette.open": "go",
		}
		err := cfg.Validate()
		var validationErr ValidationError
		if !errors.As(err, &validationErr) {
			t.Fatalf("Validate() error = %T %v, want ValidationError", err, err)
		}
		if validationErr.Path != `cmd_palette.aliases["session.new"]` ||
			validationErr.Message != `alias "go" is already owned by "palette.open"` {
			t.Fatalf("ValidationError = %#v, want stable duplicate owner", validationErr)
		}
	})

	t.Run("Should clone nested values without sharing state", func(t *testing.T) {
		t.Parallel()

		source := DefaultWithHome(HomePaths{HomeDir: t.TempDir()})
		source.CmdPalette.Aliases["session.new"] = "new"
		cloned := CloneConfig(&source)
		cloned.CmdPalette.FallbackTargets[0] = "changed"
		cloned.CmdPalette.Aliases["session.new"] = "changed"
		if source.CmdPalette.FallbackTargets[0] != CmdPaletteFallbackAgent ||
			source.CmdPalette.Aliases["session.new"] != "new" {
			t.Fatalf("CloneConfig() shared command-palette state: %#v", source.CmdPalette)
		}
	})
}

func TestApplyCmdPaletteOverlayFile(t *testing.T) {
	t.Parallel()

	t.Run("Should merge workspace values over defaults", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), ConfigName)
		contents := []byte(
			"[cmd_palette]\npersonalization = false\n\n" +
				"[cmd_palette.aliases]\n\"session.new\" = \"new\"\n",
		)
		if err := os.WriteFile(path, contents, 0o600); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}
		got, err := ApplyCmdPaletteOverlayFile(path, DefaultCmdPaletteConfig())
		if err != nil {
			t.Fatalf("ApplyCmdPaletteOverlayFile() error = %v", err)
		}
		if got.Personalization || got.Aliases["session.new"] != "new" ||
			len(got.FallbackTargets) != 1 || got.FallbackTargets[0] != CmdPaletteFallbackAgent {
			t.Fatalf("ApplyCmdPaletteOverlayFile() = %#v, want merged values", got)
		}
	})
}
