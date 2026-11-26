package acp

import (
	"errors"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/testutil"
)

func TestAwaitSessionReasoningOption(t *testing.T) {
	t.Parallel()

	t.Run("Should wait for a late config_option_update that publishes reasoning effort", func(t *testing.T) {
		t.Parallel()

		driver := New(WithConfigSettleBudget(500 * time.Millisecond))
		proc := &AgentProcess{}
		proc.setCaps(Caps{
			ConfigOptions: []SessionConfigOption{
				{
					ID:             "model",
					Kind:           SessionConfigOptionKindSelect,
					CurrentValueID: "m1",
					Values:         []SessionConfigOptionValue{{Value: "m1", Label: "m1"}},
				},
			},
		})

		done := make(chan struct{})
		go func() {
			defer close(done)
			time.Sleep(15 * time.Millisecond)
			proc.setConfigOptions(sessionConfigOptionsFromSDK(helperConfigOptions("m1", "high")))
		}()

		option, err := driver.awaitSessionReasoningOption(testutil.Context(t), proc, "high", true)
		<-done
		if err != nil {
			t.Fatalf("awaitSessionReasoningOption() error = %v", err)
		}
		if !configOptionAllowsValue(option, "high") {
			t.Fatalf("returned option %#v does not allow high", option)
		}
	})

	t.Run("Should return reasoning_option_missing when no reasoning update arrives within budget", func(t *testing.T) {
		t.Parallel()

		driver := New(WithConfigSettleBudget(30 * time.Millisecond))
		proc := &AgentProcess{}
		proc.setCaps(Caps{
			ConfigOptions: []SessionConfigOption{
				{
					ID:             "model",
					Kind:           SessionConfigOptionKindSelect,
					CurrentValueID: "m1",
					Values:         []SessionConfigOptionValue{{Value: "m1", Label: "m1"}},
				},
			},
		})

		_, err := driver.awaitSessionReasoningOption(testutil.Context(t), proc, "high", true)
		negotiationErr, ok := errors.AsType[*NegotiationError](err)
		if !ok || negotiationErr.Code != NegotiationCodeReasoningOptionMissing {
			t.Fatalf("awaitSessionReasoningOption() error = %v, want %s", err, NegotiationCodeReasoningOptionMissing)
		}
	})
}
