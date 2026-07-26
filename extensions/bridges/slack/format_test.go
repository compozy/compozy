// Suite: Slack outbound markdown dialect
// Invariant: Standard markdown renders as valid Slack mrkdwn without changing protected code.
// Boundary IN: Pure Slack outbound formatting.
// Boundary OUT: Delivery sequencing and Slack HTTP serialization in provider_test.go.
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

			if got := formatOutbound(testCase.Input); got != testCase.Slack {
				t.Fatalf("formatOutbound() = %q, want %q", got, testCase.Slack)
			}
		})
	}
}
