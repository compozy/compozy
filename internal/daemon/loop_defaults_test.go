package daemon

import (
	"testing"

	compozyconfig "github.com/compozy/compozy/internal/config"
	loopdsl "github.com/compozy/compozy/internal/loop/dsl"
)

func TestLoopDefaultsFromConfigShouldMapDeliveryAndWatchDefaults(t *testing.T) {
	t.Parallel()

	t.Run("Should map delivery and watch defaults", func(t *testing.T) {
		t.Parallel()

		defaults := loopDefaultsFromConfig(compozyconfig.DefaultLoopsConfig())

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

		cfg := compozyconfig.DefaultLoopsConfig()
		cfg.Defaults.Delivery.RuntimeDefaults.Worker.Model = "delivery-worker"
		cfg.Defaults.Delivery.RuntimeDefaults.Judge.Model = "delivery-judge"
		cfg.Defaults.Watch.RuntimeDefaults.Judge.Model = "watch-judge"
		defaults = loopDefaultsFromConfig(cfg)

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
	})
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
