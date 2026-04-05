package core

import "testing"

func TestConfigValidateRejectsNegativeTailLines(t *testing.T) {
	t.Parallel()

	err := Config{TailLines: -1}.Validate()
	if err == nil {
		t.Fatal("expected negative tail-lines to be rejected")
	}
}

func TestConfigValidateAcceptsExecMode(t *testing.T) {
	t.Parallel()

	err := Config{
		Mode:            ModeExec,
		IDE:             IDECodex,
		OutputFormat:    OutputFormatJSON,
		PromptText:      "Summarize the repo state",
		BatchSize:       1,
		MaxRetries:      1,
		AccessMode:      AccessModeFull,
		ReasoningEffort: "medium",
	}.Validate()
	if err != nil {
		t.Fatalf("expected exec config to validate: %v", err)
	}
}

func TestConfigValidateRejectsExecModeWithoutPromptSource(t *testing.T) {
	t.Parallel()

	err := Config{
		Mode:         ModeExec,
		IDE:          IDECodex,
		OutputFormat: OutputFormatText,
	}.Validate()
	if err == nil {
		t.Fatal("expected exec config without prompt source to fail validation")
	}
}
