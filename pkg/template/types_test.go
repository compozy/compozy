package template

import (
	"strings"
	"testing"
)

func TestValidateMode(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		mode    string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "memory mode valid",
			mode:    "memory",
			wantErr: false,
		},
		{
			name:    "persistent mode valid",
			mode:    "persistent",
			wantErr: false,
		},
		{
			name:    "distributed mode valid",
			mode:    "distributed",
			wantErr: false,
		},
		{
			name:    "standalone rejected with hint",
			mode:    "standalone",
			wantErr: true,
			errMsg:  "has been replaced",
		},
		{
			name:    "invalid mode rejected",
			mode:    "invalid",
			wantErr: true,
			errMsg:  "invalid mode",
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateMode(tt.mode)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateMode() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && !strings.Contains(err.Error(), tt.errMsg) {
				t.Fatalf("ValidateMode() error = %v, want message containing %q", err, tt.errMsg)
			}
		})
	}
}
