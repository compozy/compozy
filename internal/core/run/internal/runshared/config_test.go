package runshared

import "testing"

func TestJobCodeFileLabel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		codeFiles []string
		want      string
	}{
		{
			name:      "empty",
			codeFiles: nil,
			want:      "",
		},
		{
			name:      "single file",
			codeFiles: []string{"task_01"},
			want:      "task_01",
		},
		{
			name:      "multiple files",
			codeFiles: []string{"task_01", "task_02", "task_03"},
			want:      "task_01, task_02, task_03",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			j := Job{CodeFiles: append([]string(nil), tt.codeFiles...)}
			if got := j.CodeFileLabel(); got != tt.want {
				t.Fatalf("codeFileLabel() = %q, want %q", got, tt.want)
			}
		})
	}
}
