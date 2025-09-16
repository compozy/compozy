package postgres

import "testing"

func TestDollarList(t *testing.T) {
	if got := dollarList(1, 3); got != "$1,$2,$3" {
		t.Fatalf("unexpected: %s", got)
	}
	if got := dollarList(5, 0); got != "" {
		t.Fatalf("expected empty, got: %s", got)
	}
}

func TestItoa(t *testing.T) {
	if got := itoa(0); got != "0" {
		t.Fatalf("itoa(0)=%s", got)
	}
	if got := itoa(42); got != "42" {
		t.Fatalf("itoa(42)=%s", got)
	}
	if got := itoa(-7); got != "-7" {
		t.Fatalf("itoa(-7)=%s", got)
	}
}
