// Suite: Telegram outbound markdown dialect
// Invariant: Standard markdown becomes lossless MarkdownV2 or an explicit plain-text fallback.
// Boundary IN: Pure Telegram outbound formatting.
// Boundary OUT: Delivery sequencing and Bot API serialization in provider_test.go.
package main

import (
	"testing"

	"github.com/compozy/agh/internal/testutil/bridgeformat"
)

func TestFormatOutbound(t *testing.T) {
	t.Parallel()

	for _, testCase := range bridgeformat.Cases() {
		t.Run(testCase.Name, func(t *testing.T) {
			t.Parallel()

			got := formatOutbound(testCase.Input)
			if got.Text != testCase.Telegram {
				t.Fatalf("formatOutbound().Text = %q, want %q", got.Text, testCase.Telegram)
			}
			if got.PlainText != testCase.TelegramPlain {
				t.Fatalf("formatOutbound().PlainText = %q, want %q", got.PlainText, testCase.TelegramPlain)
			}
			if got.ParseMode != testCase.TelegramParseMode {
				t.Fatalf("formatOutbound().ParseMode = %q, want %q", got.ParseMode, testCase.TelegramParseMode)
			}
		})
	}
}
