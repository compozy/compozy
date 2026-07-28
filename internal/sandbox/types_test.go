package sandbox

import "testing"

func TestBackendValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		backend Backend
		want    bool
	}{
		{backend: BackendLocal, want: true},
		{backend: BackendDaytona, want: true},
		{backend: BackendE2B, want: true},
		{backend: Backend("docker"), want: false},
		{backend: Backend(""), want: false},
	}

	for _, tt := range tests {
		t.Run(string(tt.backend), func(t *testing.T) {
			t.Parallel()

			if got := tt.backend.Valid(); got != tt.want {
				t.Fatalf("Backend.Valid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSyncModeValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		mode SyncMode
		want bool
	}{
		{mode: SyncModeNone, want: true},
		{mode: SyncModeSessionBidirectional, want: true},
		{mode: SyncModeTurnBidirectional, want: true},
		{mode: SyncMode("always"), want: false},
	}

	for _, tt := range tests {
		t.Run(string(tt.mode), func(t *testing.T) {
			t.Parallel()

			if got := tt.mode.Valid(); got != tt.want {
				t.Fatalf("SyncMode.Valid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPersistenceModeValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		mode PersistenceMode
		want bool
	}{
		{mode: PersistenceTransient, want: true},
		{mode: PersistenceReuse, want: true},
		{mode: PersistenceArchive, want: true},
		{mode: PersistenceMode("forever"), want: false},
	}

	for _, tt := range tests {
		t.Run(string(tt.mode), func(t *testing.T) {
			t.Parallel()

			if got := tt.mode.Valid(); got != tt.want {
				t.Fatalf("PersistenceMode.Valid() = %v, want %v", got, tt.want)
			}
		})
	}
}
