package uc

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateFlushInput(t *testing.T) {
	tests := []struct {
		name      string
		input     *FlushMemoryInput
		expectErr bool
		errMsg    string
	}{
		{
			name:      "nil input",
			input:     nil,
			expectErr: true,
			errMsg:    "invalid payload",
		},
		{
			name: "valid input with max keys",
			input: &FlushMemoryInput{
				MaxKeys: 100,
			},
			expectErr: false,
		},
		{
			name: "valid input with strategy (service layer validates)",
			input: &FlushMemoryInput{
				Strategy: "simple_fifo",
			},
			expectErr: false,
		},
		{
			name: "negative max keys",
			input: &FlushMemoryInput{
				MaxKeys: -1,
			},
			expectErr: true,
			errMsg:    "must be non-negative",
		},
		{
			name: "max keys too large",
			input: &FlushMemoryInput{
				MaxKeys: 10001,
			},
			expectErr: true,
			errMsg:    "too large",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFlushInput(tt.input)

			if tt.expectErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
