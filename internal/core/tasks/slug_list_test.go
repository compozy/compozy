package tasks

import (
	"slices"
	"strings"
	"testing"
)

func TestParseCommaSeparatedSlugsPreservesOrder(t *testing.T) {
	t.Parallel()

	got, err := ParseCommaSeparatedSlugs("alpha,beta,gamma")
	if err != nil {
		t.Fatalf("parse slugs: %v", err)
	}
	want := []string{"alpha", "beta", "gamma"}
	if !slices.Equal(got, want) {
		t.Fatalf("unexpected slugs\nwant: %#v\ngot:  %#v", want, got)
	}
}

func TestParseCommaSeparatedSlugsTrimsEntries(t *testing.T) {
	t.Parallel()

	got, err := ParseCommaSeparatedSlugs("alpha, beta ,gamma")
	if err != nil {
		t.Fatalf("parse slugs: %v", err)
	}
	want := []string{"alpha", "beta", "gamma"}
	if !slices.Equal(got, want) {
		t.Fatalf("unexpected slugs\nwant: %#v\ngot:  %#v", want, got)
	}
}

func TestParseCommaSeparatedSlugsRejectsEmptyEntries(t *testing.T) {
	t.Parallel()

	_, err := ParseCommaSeparatedSlugs("alpha,,beta")
	if err == nil {
		t.Fatal("expected empty slug error")
	}
	if !strings.Contains(err.Error(), "position 2 cannot be empty") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseCommaSeparatedSlugsRejectsDuplicates(t *testing.T) {
	t.Parallel()

	_, err := ParseCommaSeparatedSlugs("alpha, beta ,alpha")
	if err == nil {
		t.Fatal("expected duplicate slug error")
	}
	if !strings.Contains(err.Error(), `duplicate task slug "alpha"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}
