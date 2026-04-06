package core

import (
	"context"
	"errors"
	"testing"
)

func TestConfigValidateRejectsNegativeTailLines(t *testing.T) {
	t.Parallel()

	err := Config{TailLines: -1}.Validate()
	if err == nil {
		t.Fatal("expected negative tail-lines to be rejected")
	}
}

func TestConfigValidateAcceptsExecMode(t *testing.T) {
	t.Parallel()

	for _, format := range []OutputFormat{OutputFormatJSON, OutputFormatRawJSON} {
		format := format
		t.Run(string(format), func(t *testing.T) {
			t.Parallel()

			err := Config{
				Mode:            ModeExec,
				IDE:             IDECodex,
				OutputFormat:    format,
				PromptText:      "Summarize the repo state",
				BatchSize:       1,
				MaxRetries:      1,
				AccessMode:      AccessModeFull,
				ReasoningEffort: "medium",
			}.Validate()
			if err != nil {
				t.Fatalf("expected exec config to validate: %v", err)
			}
		})
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

func TestConfigValidateRejectsAddDirsForUnsupportedIDE(t *testing.T) {
	t.Parallel()

	err := Config{
		IDE:     IDECursor,
		AddDirs: []string{"../shared"},
	}.Validate()
	if err == nil {
		t.Fatal("expected unsupported add-dir runtime to fail validation")
	}
}

func TestConfigValidateAcceptsAddDirsForSupportedIDE(t *testing.T) {
	t.Parallel()

	err := Config{
		IDE:     IDEClaude,
		AddDirs: []string{"../shared"},
	}.Validate()
	if err != nil {
		t.Fatalf("expected supported add-dir runtime to validate: %v", err)
	}
}

func TestRunDelegatesToRegisteredDispatcherAdapter(t *testing.T) {
	previous := registeredDispatcherAdapters
	t.Cleanup(func() {
		registeredDispatcherAdapters = previous
	})

	wantErr := errors.New("adapter boom")
	called := false
	RegisterDispatcherAdapters(DispatcherAdapters{
		Run: func(_ context.Context, cfg Config) error {
			called = true
			if cfg.Name != "demo" {
				t.Fatalf("unexpected config name: %q", cfg.Name)
			}
			return wantErr
		},
	})

	err := Run(context.Background(), Config{Name: "demo"})
	if !errors.Is(err, wantErr) {
		t.Fatalf("Run() error = %v, want %v", err, wantErr)
	}
	if !called {
		t.Fatal("expected registered run adapter to be called")
	}
}

func TestPrepareDelegatesToRegisteredDispatcherAdapter(t *testing.T) {
	previous := registeredDispatcherAdapters
	t.Cleanup(func() {
		registeredDispatcherAdapters = previous
	})

	wantPrep := &Preparation{ResolvedName: "demo"}
	called := false
	RegisterDispatcherAdapters(DispatcherAdapters{
		Prepare: func(_ context.Context, cfg Config) (*Preparation, error) {
			called = true
			if cfg.Name != "demo" {
				t.Fatalf("unexpected config name: %q", cfg.Name)
			}
			return wantPrep, nil
		},
	})

	prep, err := Prepare(context.Background(), Config{Name: "demo"})
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	if prep != wantPrep {
		t.Fatalf("Prepare() = %#v, want %#v", prep, wantPrep)
	}
	if !called {
		t.Fatal("expected registered prepare adapter to be called")
	}
}
