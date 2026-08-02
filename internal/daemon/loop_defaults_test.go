package daemon

import (
	"testing"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	looppkg "github.com/compozy/compozy/internal/loop"
	loopdsl "github.com/compozy/compozy/internal/loop/dsl"
)

func TestLoopDefaultsFromConfigShouldMapDeliveryAndWatchDefaults(t *testing.T) {
	t.Parallel()

	t.Run("Should map delivery and watch defaults", func(t *testing.T) {
		t.Parallel()

		cfg := compozyconfig.DefaultLoopsConfig()
		defaults, err := loopDefaultsFromConfig(&cfg)
		if err != nil {
			t.Fatalf("loopDefaultsFromConfig() error = %v", err)
		}

		assertIntPointer(t, "delivery iteration cap", defaults.Delivery.IterationCap, 50)
		assertIntPointer(t, "delivery no-progress window", defaults.Delivery.NoProgressWindow, 3)
		assertIntPointer(t, "delivery gate max revisions", defaults.Delivery.GateMaxRevisions, 10)
		assertIntPointer(t, "delivery budget tokens", defaults.Delivery.BudgetTokens, 0)
		assertIntPointer(t, "delivery budget wall", defaults.Delivery.BudgetWallSec, 0)
		if defaults.Delivery.BudgetOnExceeded == nil ||
			*defaults.Delivery.BudgetOnExceeded != loopdsl.BudgetExceededHalt {
			t.Fatalf("delivery budget on exceeded = %#v, want halt", defaults.Delivery.BudgetOnExceeded)
		}
		if defaults.Delivery.RuntimeDefaults != nil {
			t.Fatalf("delivery runtime defaults = %#v, want nil for empty config", defaults.Delivery.RuntimeDefaults)
		}
		assertIntPointer(t, "delivery fan out width", defaults.Delivery.FanOutWidth, 4)
		assertLifecycleDefaults(t, "delivery", defaults.Delivery.Lifecycle)

		assertIntPointer(t, "watch iteration cap", defaults.Watch.IterationCap, 0)
		assertIntPointer(t, "watch no-progress window", defaults.Watch.NoProgressWindow, 2)
		if defaults.Watch.GateMaxRevisions != nil {
			t.Fatalf(
				"watch gate max revisions = %#v, want nil when config default is zero",
				defaults.Watch.GateMaxRevisions,
			)
		}
		if defaults.Watch.RuntimeDefaults != nil {
			t.Fatalf("watch runtime defaults = %#v, want nil for empty config", defaults.Watch.RuntimeDefaults)
		}
		assertIntPointer(t, "watch fan out width", defaults.Watch.FanOutWidth, 2)
		assertLifecycleDefaults(t, "watch", defaults.Watch.Lifecycle)

		breaker, err := loopBreakerPolicyFromConfig(compozyconfig.DefaultLoopsConfig().Breaker)
		if err != nil {
			t.Fatalf("loopBreakerPolicyFromConfig() error = %v", err)
		}
		if breaker.Threshold != 5 || breaker.ProbeInterval != time.Minute ||
			breaker.Source != looppkg.BreakerGlobalSource {
			t.Fatalf("breaker policy = %#v, want global 5/60s", breaker)
		}

		cfg = compozyconfig.DefaultLoopsConfig()
		cfg.Defaults.Delivery.RuntimeDefaults.Worker.Model = "delivery-worker"
		cfg.Defaults.Delivery.RuntimeDefaults.Judge.Model = "delivery-judge"
		cfg.Defaults.Watch.RuntimeDefaults.Judge.Model = "watch-judge"
		cfg.Defaults.Delivery.Autopause = []compozyconfig.LoopAutopauseRule{
			{Match: "attempt >= 2", Action: "pause"},
		}
		defaults, err = loopDefaultsFromConfig(&cfg)
		if err != nil {
			t.Fatalf("loopDefaultsFromConfig(configured) error = %v", err)
		}

		if defaults.Delivery.RuntimeDefaults == nil {
			t.Fatal("delivery runtime defaults = nil, want configured defaults")
		}
		assertString(t, "delivery worker model", defaults.Delivery.RuntimeDefaults.Worker.Model, "delivery-worker")
		assertString(t, "delivery judge model", defaults.Delivery.RuntimeDefaults.Judge.Model, "delivery-judge")
		if defaults.Watch.RuntimeDefaults == nil {
			t.Fatal("watch runtime defaults = nil, want configured defaults")
		}
		if defaults.Watch.RuntimeDefaults.Worker.Model != "" {
			t.Fatalf("watch worker model = %#v, want empty when unset", defaults.Watch.RuntimeDefaults.Worker)
		}
		assertString(t, "watch judge model", defaults.Watch.RuntimeDefaults.Judge.Model, "watch-judge")
		if got := defaults.Delivery.Lifecycle.Autopause; len(got) != 1 ||
			got[0] != (looppkg.LifecycleAutopauseRule{Match: "attempt >= 2", Action: "pause"}) {
			t.Fatalf("delivery lifecycle autopause = %#v, want mapped rule", got)
		}
	})
}

func assertLifecycleDefaults(t *testing.T, label string, got *looppkg.LifecycleConfig) {
	t.Helper()

	if got == nil {
		t.Fatalf("%s lifecycle = nil", label)
	}
	assertIntPointer(t, label+" retry attempts", got.RetryMaxAttempts, 3)
	assertIntPointer(t, label+" resume death streak", got.ResumeDeathStreakLimit, 3)
	assertIntPointer(t, label+" wait admission attempts", got.WaitAdmissionAttempts, 3)
	if got.RetryBackoffBase == nil || *got.RetryBackoffBase != time.Second ||
		got.RetryBackoffMax == nil || *got.RetryBackoffMax != 30*time.Second ||
		got.LivenessSilenceWindow == nil || *got.LivenessSilenceWindow != 30*time.Minute ||
		got.PredicateCostLimit == nil || *got.PredicateCostLimit != 10000 ||
		got.WaitAdmissionInterval == nil || *got.WaitAdmissionInterval != time.Minute ||
		got.AdmissionHorizon == nil || *got.AdmissionHorizon != 168*time.Hour {
		t.Fatalf("%s lifecycle = %#v, want mapped shipped defaults", label, got)
	}
}

func assertIntPointer(t *testing.T, label string, got *int, want int) {
	t.Helper()

	if got == nil {
		t.Fatalf("%s = nil, want %d", label, want)
	}
	if *got != want {
		t.Fatalf("%s = %d, want %d", label, *got, want)
	}
}

func assertString(t *testing.T, label string, got string, want string) {
	t.Helper()
	if got != want {
		t.Fatalf("%s = %q, want %q", label, got, want)
	}
}
