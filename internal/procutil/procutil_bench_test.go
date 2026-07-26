package procutil

import (
	"os"
	"syscall"
	"testing"
)

func BenchmarkAliveCurrentProcess(b *testing.B) {
	pid := os.Getpid()
	b.ReportAllocs()

	for b.Loop() {
		if !Alive(pid) {
			b.Fatalf("Alive(%d) = false, want true", pid)
		}
	}
}

func BenchmarkSignalCurrentProcessZero(b *testing.B) {
	pid := os.Getpid()
	sig := syscall.Signal(0)
	b.ReportAllocs()

	for b.Loop() {
		if err := Signal(pid, sig); err != nil {
			b.Fatalf("Signal(%d, %d) error = %v, want nil", pid, sig, err)
		}
	}
}
