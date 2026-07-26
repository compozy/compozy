package daemon

import (
	"testing"

	aghconfig "github.com/compozy/agh/internal/config"
	loopdsl "github.com/compozy/agh/internal/loop/dsl"
)

func TestLoopDefaultsFromConfigShouldMapDeliveryAndWatchDefaults(t *testing.T) {
	t.Parallel()

	t.Run("Should map delivery and watch defaults", func(t *testing.T) {
		t.Parallel()

		defaults := loopDefaultsFromConfig(aghconfig.DefaultLoopsConfig())

		assertIntPointer(t, "delivery iteration cap", defaults.Delivery.IterationCap, 50)
		assertIntPointer(t, "delivery no-progress window", defaults.Delivery.NoProgressWindow, 3)
		assertIntPointer(t, "delivery gate max revisions", defaults.Delivery.GateMaxRevisions, 10)
		assertIntPointer(t, "delivery budget tokens", defaults.Delivery.BudgetTokens, 0)
		assertIntPointer(t, "delivery budget wall", defaults.Delivery.BudgetWallSec, 0)
		if defaults.Delivery.BudgetOnExceeded == nil ||
			*defaults.Delivery.BudgetOnExceeded != loopdsl.BudgetExceededHalt {
			t.Fatalf("delivery budget on exceeded = %#v, want halt", defaults.Delivery.BudgetOnExceeded)
		}
		if defaults.Delivery.ModelDefaults != nil {
			t.Fatalf("delivery model defaults = %#v, want nil for empty config", defaults.Delivery.ModelDefaults)
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
		if defaults.Watch.ModelDefaults != nil {
			t.Fatalf("watch model defaults = %#v, want nil for empty config", defaults.Watch.ModelDefaults)
		}
		assertIntPointer(t, "watch fan out width", defaults.Watch.FanOutWidth, 2)

		cfg := aghconfig.DefaultLoopsConfig()
		cfg.Defaults.Delivery.ModelDefaults.Worker = "delivery-worker"
		cfg.Defaults.Delivery.ModelDefaults.Judge = "delivery-judge"
		cfg.Defaults.Watch.ModelDefaults.Judge = "watch-judge"
		defaults = loopDefaultsFromConfig(cfg)

		if defaults.Delivery.ModelDefaults == nil {
			t.Fatal("delivery model defaults = nil, want configured defaults")
		}
		assertStringPointer(t, "delivery worker model", defaults.Delivery.ModelDefaults.Worker, "delivery-worker")
		assertStringPointer(t, "delivery judge model", defaults.Delivery.ModelDefaults.Judge, "delivery-judge")
		if defaults.Watch.ModelDefaults == nil {
			t.Fatal("watch model defaults = nil, want configured defaults")
		}
		if defaults.Watch.ModelDefaults.Worker != nil {
			t.Fatalf("watch worker model = %#v, want nil when unset", defaults.Watch.ModelDefaults.Worker)
		}
		assertStringPointer(t, "watch judge model", defaults.Watch.ModelDefaults.Judge, "watch-judge")
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

func assertStringPointer(t *testing.T, label string, got *string, want string) {
	t.Helper()

	if got == nil {
		t.Fatalf("%s = nil, want %q", label, want)
	}
	if *got != want {
		t.Fatalf("%s = %q, want %q", label, *got, want)
	}
}
