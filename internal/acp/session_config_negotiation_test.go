package acp

import (
	"errors"
	"testing"
	"testing/synctest"
	"time"
)

func TestAwaitSessionReasoningOption(t *testing.T) {
	t.Parallel()

	t.Run("Should wait for a late config_option_update that publishes reasoning effort", func(t *testing.T) {
		t.Parallel()

		synctest.Test(t, func(t *testing.T) {
			driver := New(WithConfigSettleBudget(time.Second))
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

			var option SessionConfigOption
			var err error
			done := make(chan struct{})
			go func() {
				defer close(done)
				option, err = driver.awaitSessionReasoningOption(t.Context(), proc, "high", true)
			}()
			synctest.Wait()
			select {
			case <-done:
				t.Fatalf("negotiation returned before the provider update: %v", err)
			default:
			}

			proc.setConfigOptions(sessionConfigOptionsFromSDK(helperConfigOptions("m1", "high")))
			synctest.Wait()
			select {
			case <-done:
			default:
				t.Fatal("provider update did not release the reasoning wait")
			}
			if err != nil {
				t.Fatalf("awaitSessionReasoningOption() error = %v", err)
			}
			if !configOptionAllowsValue(option, "high") {
				t.Fatalf("returned option %#v does not allow high", option)
			}
		})
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

		_, err := driver.awaitSessionReasoningOption(t.Context(), proc, "high", true)
		negotiationErr, ok := errors.AsType[*NegotiationError](err)
		if !ok || negotiationErr.Code != NegotiationCodeReasoningOptionMissing {
			t.Fatalf("awaitSessionReasoningOption() error = %v, want %s", err, NegotiationCodeReasoningOptionMissing)
		}
	})
}
