package logger

import (
	"os"
	"testing"
)

func BenchmarkNewFileOnly(b *testing.B) {
	b.ReportAllocs()

	for b.Loop() {
		log, closeFn, err := New(WithLevel("info"), WithFile(os.DevNull), WithMirrorToStderr(false))
		if err != nil {
			b.Fatalf("New() error = %v", err)
		}
		if log == nil {
			b.Fatal("New() logger = nil")
		}
		if err := closeFn(); err != nil {
			b.Fatalf("closeFn() error = %v", err)
		}
	}
}

func BenchmarkLogFileOnly(b *testing.B) {
	b.ReportAllocs()

	log, closeFn, err := New(WithLevel("info"), WithFile(os.DevNull), WithMirrorToStderr(false))
	if err != nil {
		b.Fatalf("New() error = %v", err)
	}
	b.Cleanup(func() {
		if err := closeFn(); err != nil {
			b.Errorf("closeFn() error = %v", err)
		}
	})

	for b.Loop() {
		log.Info("bench", "component", "logger")
	}
}
