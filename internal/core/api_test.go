package core

import "testing"

func TestConfigValidateRejectsNegativeTailLines(t *testing.T) {
	t.Parallel()

	err := Config{TailLines: -1}.Validate()
	if err == nil {
		t.Fatal("expected negative tail-lines to be rejected")
	}
}
