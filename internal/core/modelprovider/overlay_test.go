package modelprovider

import "testing"

func TestActivateOverlayResolvesAliasesAndExposesCatalog(t *testing.T) {
	restore, err := ActivateOverlay([]OverlayEntry{
		{
			Name:        "ext-fast",
			DisplayName: "Extension Fast",
			Target:      "openai/codex-fast",
		},
		{
			Name:        "ext-smart",
			DisplayName: "Extension Smart",
			Target:      "openai/gpt-5.5",
		},
	})
	if err != nil {
		t.Fatalf("ActivateOverlay() error = %v", err)
	}
	defer restore()

	if got := ResolveAlias("ext-smart"); got != "openai/gpt-5.5" {
		t.Fatalf("ResolveAlias(ext-smart) = %q, want %q", got, "openai/gpt-5.5")
	}
	if got := ResolveAlias("EXT-FAST"); got != "openai/codex-fast" {
		t.Fatalf("ResolveAlias(EXT-FAST) = %q, want %q", got, "openai/codex-fast")
	}
	if got := ResolveAlias("gpt-5.5"); got != "gpt-5.5" {
		t.Fatalf("ResolveAlias(gpt-5.5) = %q, want passthrough", got)
	}

	catalog := Catalog()
	if len(catalog) != 2 {
		t.Fatalf("Catalog() len = %d, want 2", len(catalog))
	}
	if catalog[0].Name != "ext-fast" || catalog[1].Name != "ext-smart" {
		t.Fatalf("Catalog() order = %#v, want sorted aliases", catalog)
	}
}

func TestActivateOverlayRejectsMissingTarget(t *testing.T) {
	_, err := ActivateOverlay([]OverlayEntry{{
		Name: "ext-smart",
	}})
	if err == nil || err.Error() != `declare model overlay "ext-smart": target model is required` {
		t.Fatalf("expected missing target error, got %v", err)
	}
}
