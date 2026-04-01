package charmtheme

import (
	"image/color"
	"testing"

	"charm.land/lipgloss/v2"
)

func TestCompozyPaletteDefaults(t *testing.T) {
	t.Parallel()

	if !sameColor(ColorBgBase, lipgloss.Color("#0C0A09")) {
		t.Fatalf("expected base background #0C0A09, got %v", ColorBgBase)
	}
	if !sameColor(ColorBgSurface, lipgloss.Color("#1C1917")) {
		t.Fatalf("expected surface background #1C1917, got %v", ColorBgSurface)
	}
	if !sameColor(ColorBgOverlay, lipgloss.Color("#292524")) {
		t.Fatalf("expected overlay background #292524, got %v", ColorBgOverlay)
	}
	if !sameColor(ColorBrand, lipgloss.Color("#CAEA28")) {
		t.Fatalf("expected brand color #CAEA28, got %v", ColorBrand)
	}
	if !sameColor(ColorAccent, lipgloss.Color("#A3E635")) {
		t.Fatalf("expected accent color #A3E635, got %v", ColorAccent)
	}
	if !sameColor(ColorBorder, lipgloss.Color("#44403C")) {
		t.Fatalf("expected border color #44403C, got %v", ColorBorder)
	}
	if got, want := ProgressGradientStart, "#65A30D"; got != want {
		t.Fatalf("expected progress gradient start %s, got %s", want, got)
	}
	if got, want := ProgressGradientEnd, "#CAEA28"; got != want {
		t.Fatalf("expected progress gradient end %s, got %s", want, got)
	}
}

func TestTechBorderUsesSquareChrome(t *testing.T) {
	t.Parallel()

	if got := TechBorder.TopLeft; got != "┌" {
		t.Fatalf("expected top-left border glyph ┌, got %q", got)
	}
	if got := TechBorder.TopRight; got != "┐" {
		t.Fatalf("expected top-right border glyph ┐, got %q", got)
	}
	if got := TechBorder.BottomLeft; got != "└" {
		t.Fatalf("expected bottom-left border glyph └, got %q", got)
	}
	if got := TechBorder.BottomRight; got != "┘" {
		t.Fatalf("expected bottom-right border glyph ┘, got %q", got)
	}
}

func sameColor(left, right color.Color) bool {
	lr, lg, lb, la := left.RGBA()
	rr, rg, rb, ra := right.RGBA()
	return lr == rr && lg == rg && lb == rb && la == ra
}
