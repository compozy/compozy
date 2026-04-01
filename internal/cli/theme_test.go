package cli

import (
	"bytes"
	"image/color"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/compozy/compozy/internal/charmtheme"
	"github.com/spf13/cobra"
)

func TestDarkHuhThemeUsesCompozyTokens(t *testing.T) {
	t.Parallel()

	styles := darkHuhTheme().Theme(true)

	if !sameColor(styles.Focused.Title.GetForeground(), charmtheme.ColorBrand) {
		t.Fatalf("expected focused title to use brand color, got %v", styles.Focused.Title.GetForeground())
	}
	if !sameColor(styles.Focused.SelectSelector.GetForeground(), charmtheme.ColorAccent) {
		t.Fatalf("expected selector to use accent color, got %v", styles.Focused.SelectSelector.GetForeground())
	}
	if !sameColor(styles.Focused.Base.GetBorderLeftForeground(), charmtheme.ColorBorderFocus) {
		t.Fatalf(
			"expected focused border to use focus border color, got %v",
			styles.Focused.Base.GetBorderLeftForeground(),
		)
	}
	if !sameColor(styles.Focused.FocusedButton.GetBackground(), charmtheme.ColorBrand) {
		t.Fatalf(
			"expected focused button background to use brand color, got %v",
			styles.Focused.FocusedButton.GetBackground(),
		)
	}
	if !sameColor(styles.Focused.TextInput.Prompt.GetForeground(), charmtheme.ColorAccent) {
		t.Fatalf(
			"expected text input prompt to use accent color, got %v",
			styles.Focused.TextInput.Prompt.GetForeground(),
		)
	}
	if !sameColor(styles.Help.ShortKey.GetForeground(), charmtheme.ColorAccentDeep) {
		t.Fatalf("expected help key style to use deep accent color, got %v", styles.Help.ShortKey.GetForeground())
	}
	if sameColor(styles.Focused.SelectSelector.GetForeground(), lipgloss.Color("#F780E2")) {
		t.Fatalf("expected selector to avoid stock Charm fuchsia accent")
	}

	border := styles.Focused.Base.GetBorderStyle()
	if border.Left != charmtheme.TechBorder.Left {
		t.Fatalf(
			"expected focused border style to use technical border glyph %q, got %q",
			charmtheme.TechBorder.Left,
			border.Left,
		)
	}
}

func TestRenderFormIntroUsesTechnicalChrome(t *testing.T) {
	t.Parallel()

	rendered := renderFormIntro()
	if !strings.Contains(rendered, "COMPOZY // INTERACTIVE INPUT") {
		t.Fatalf("expected interactive input banner, got %q", rendered)
	}
	if !strings.Contains(rendered, "┌") {
		t.Fatalf("expected technical border chrome, got %q", rendered)
	}
	if strings.ContainsAny(rendered, "╭╮╰╯") {
		t.Fatalf("expected to avoid rounded border glyphs, got %q", rendered)
	}
}

func TestSetupWelcomeHeaderUsesTechnicalChrome(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	cmd := &cobra.Command{Use: "setup"}
	cmd.SetOut(&out)

	printWelcomeHeader(cmd)
	rendered := out.String()
	if !strings.Contains(rendered, "COMPOZY // SETUP") {
		t.Fatalf("expected setup header title, got %q", rendered)
	}
	if !strings.Contains(rendered, "┌") {
		t.Fatalf("expected setup header to use technical border chrome, got %q", rendered)
	}
	if strings.ContainsAny(rendered, "╭╮╰╯") {
		t.Fatalf("expected setup header to avoid rounded border glyphs, got %q", rendered)
	}
}

func TestCLIChromeStylesUseCompozyPalette(t *testing.T) {
	t.Parallel()

	styles := newCLIChromeStyles()
	if !sameColor(styles.title.GetForeground(), charmtheme.ColorBrand) {
		t.Fatalf("expected title style to use brand color, got %v", styles.title.GetForeground())
	}
	if !sameColor(styles.box.GetBackground(), charmtheme.ColorBgSurface) {
		t.Fatalf("expected box background to use surface color, got %v", styles.box.GetBackground())
	}
	if !sameColor(styles.box.GetBorderLeftForeground(), charmtheme.ColorAccentDeep) {
		t.Fatalf("expected box border to use deep accent color, got %v", styles.box.GetBorderLeftForeground())
	}
	if !sameColor(styles.warn.GetForeground(), charmtheme.ColorWarning) {
		t.Fatalf("expected warning style to use warning color, got %v", styles.warn.GetForeground())
	}
}

func sameColor(left, right color.Color) bool {
	lr, lg, lb, la := left.RGBA()
	rr, rg, rb, ra := right.RGBA()
	return lr == rr && lg == rg && lb == rb && la == ra
}
