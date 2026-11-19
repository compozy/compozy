package agentinstance

import (
	"math"
	"math/big"
	"testing"
)

func TestDailyBudgetLimitsValidate(t *testing.T) {
	for _, tc := range []struct {
		name   string
		limits DailyBudgetLimits
		valid  bool
	}{
		{name: "Should accept disabled limits", valid: true},
		{name: "Should accept a zero token cap", limits: DailyBudgetLimits{Tokens: new(int64)}, valid: true},
		{name: "Should reject negative token cap", limits: DailyBudgetLimits{Tokens: new(int64(-1))}},
		{name: "Should reject negative USD cap", limits: DailyBudgetLimits{USD: new(-0.1)}},
		{name: "Should reject NaN USD cap", limits: DailyBudgetLimits{USD: new(math.NaN())}},
		{name: "Should reject infinite USD cap", limits: DailyBudgetLimits{USD: new(math.Inf(1))}},
		{name: "Should accept tiny positive USD caps", limits: DailyBudgetLimits{USD: new(4e-10)}, valid: true},
		{name: "Should accept finite USD caps beyond nine billion", limits: DailyBudgetLimits{USD: new(1e301)}, valid: true},
		{name: "Should accept a finite USD cap", limits: DailyBudgetLimits{USD: new(1.5)}, valid: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.limits.Validate()
			if (err == nil) != tc.valid {
				t.Fatalf("Validate() error = %v, want valid=%v", err, tc.valid)
			}
		})
	}
}

func TestEvaluateDailyBudget(t *testing.T) {
	t.Run("Should leave uncapped instances eligible", func(t *testing.T) {
		t.Parallel()
		got := mustEvaluateDailyBudget(t, DailyBudgetLimits{}, DailyBudgetUsage{Tokens: 1000}, false)
		if got.Exhausted || got.Warn {
			t.Fatalf("unlimited budget = %+v, want eligible without warning", got)
		}
	})

	t.Run("Should warn at the token threshold only when not previously warned", func(t *testing.T) {
		t.Parallel()
		limit := int64(100)
		limits := DailyBudgetLimits{Tokens: &limit}
		for _, tc := range []struct {
			name   string
			used   int64
			warned bool
			want   bool
		}{
			{name: "Should stay silent below eighty percent", used: 79},
			{name: "Should notify at eighty percent", used: 80, want: true},
			{name: "Should suppress repeated warning", used: 90, warned: true},
		} {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				got := mustEvaluateDailyBudget(t, limits, DailyBudgetUsage{Tokens: tc.used}, tc.warned)
				if got.Warn != tc.want || got.Exhausted {
					t.Fatalf("tokens=%d warned=%v: %+v, want warning=%v and eligible", tc.used, tc.warned, got, tc.want)
				}
			})
		}
	})

	t.Run("Should exhaust tokens at the exact cap even without cost prices", func(t *testing.T) {
		t.Parallel()
		limit := int64(100)
		got := mustEvaluateDailyBudget(t, DailyBudgetLimits{Tokens: &limit}, DailyBudgetUsage{Tokens: 100}, false)
		if !got.Exhausted || !got.TokensExhausted {
			t.Fatalf("token cap = %+v, want exhausted", got)
		}
	})

	t.Run("Should ignore unknown USD costs without disabling token enforcement", func(t *testing.T) {
		t.Parallel()
		usdCap := 2.0
		tokenCap := int64(100)
		got := mustEvaluateDailyBudget(
			t, DailyBudgetLimits{Tokens: &tokenCap, USD: &usdCap}, DailyBudgetUsage{Tokens: 110}, false,
		)
		if !got.TokensExhausted || got.USDExhausted {
			t.Fatalf("unknown USD with exhausted tokens = %+v", got)
		}
	})

	t.Run("Should exhaust estimated USD at the exact cap", func(t *testing.T) {
		t.Parallel()
		usdCap := 2.0
		got := mustEvaluateDailyBudget(t, DailyBudgetLimits{USD: &usdCap}, DailyBudgetUsage{USD: big.NewRat(2, 1)}, false)
		if !got.Exhausted || !got.USDExhausted {
			t.Fatalf("USD cap = %+v, want exhausted", got)
		}
	})

	t.Run("Should retain known USD spend when other receipts have no price", func(t *testing.T) {
		t.Parallel()
		usdCap := 2.0
		got := mustEvaluateDailyBudget(t,
			DailyBudgetLimits{USD: &usdCap},
			DailyBudgetUsage{USD: big.NewRat(2, 1), HasUnpricedUsage: true},
			false,
		)
		if !got.USDExhausted {
			t.Fatalf("mixed priced/unpriced usage = %+v, want USD cap exhausted", got)
		}
	})

	t.Run("Should hold a USD budget when a completed turn has unknown cost", func(t *testing.T) {
		t.Parallel()
		usage := DailyBudgetUsage{Tokens: 50, USD: big.NewRat(1, 10), HasUnpricedUsage: true}
		budget := mustEvaluateDailyBudget(t, DailyBudgetLimits{USD: new(2.0)}, usage, false)
		if !budget.Exhausted || !budget.USDUnpriced || budget.USDExhausted || budget.Warn {
			t.Fatalf("unpriced USD decision=%+v, want admission held without a measured threshold crossing", budget)
		}
		tokens := mustEvaluateDailyBudget(t, DailyBudgetLimits{Tokens: new(int64(100))}, usage, false)
		if tokens.Exhausted || tokens.Warn {
			t.Fatalf("token-only decision=%+v, want eligible at 50 of 100 tokens", tokens)
		}
	})

	t.Run("Should block a zero USD cap even when the provider has no price", func(t *testing.T) {
		t.Parallel()
		usdCap := 0.0
		got := mustEvaluateDailyBudget(t, DailyBudgetLimits{USD: &usdCap}, DailyBudgetUsage{HasUnpricedUsage: true}, false)
		if !got.USDExhausted || !got.Exhausted {
			t.Fatalf("zero USD cap and unpriced usage = %+v, want exhausted", got)
		}
	})

	t.Run("Should warn at exact fractional USD threshold without binary rounding loss", func(t *testing.T) {
		t.Parallel()
		usdCap := 0.10
		got := mustEvaluateDailyBudget(t, DailyBudgetLimits{USD: &usdCap}, DailyBudgetUsage{USD: big.NewRat(8, 100)}, false)
		if !got.Warn || got.Exhausted {
			t.Fatalf("fractional threshold = %+v, want warning only", got)
		}
	})

	t.Run("Should not exhaust a tiny USD cap before its real boundary", func(t *testing.T) {
		t.Parallel()
		usdLimit := 1e-9
		got := mustEvaluateDailyBudget(
			t, DailyBudgetLimits{USD: &usdLimit}, DailyBudgetUsage{USD: big.NewRat(51, 100_000_000_000)}, false,
		)
		if got.Warn || got.Exhausted {
			t.Fatalf("below-threshold fraction = %+v, want eligible without warning", got)
		}
	})

	t.Run("Should reject invalid caps without panicking", func(t *testing.T) {
		t.Parallel()
		invalidCap := math.NaN()
		_, err := EvaluateDailyBudget(DailyBudgetLimits{USD: &invalidCap}, DailyBudgetUsage{USD: big.NewRat(1, 1)}, false)
		if err == nil {
			t.Fatal("EvaluateDailyBudget() accepted NaN cap")
		}
	})

	t.Run("Should reject negative restored usage before admission", func(t *testing.T) {
		t.Parallel()
		tokenLimit := int64(100)
		_, err := EvaluateDailyBudget(DailyBudgetLimits{Tokens: &tokenLimit}, DailyBudgetUsage{Tokens: -1}, false)
		if err == nil {
			t.Fatal("EvaluateDailyBudget() accepted negative restored usage")
		}
	})
}

func TestDailyBudgetUsageAddReceipt(t *testing.T) {
	t.Run("Should retain priced subtotal across unpriced receipts", func(t *testing.T) {
		t.Parallel()
		usage := DailyBudgetUsage{}
		for _, receipt := range []struct {
			tokens int64
			usd    *float64
		}{{25, new(1.25)}, {30, nil}, {15, new(0.75)}} {
			if err := usage.AddReceipt(receipt.tokens, receipt.usd); err != nil {
				t.Fatalf("AddReceipt() error = %v", err)
			}
		}
		if usage.Tokens != 70 || usage.USD == nil || usage.USD.Cmp(big.NewRat(2, 1)) != 0 || !usage.HasUnpricedUsage {
			t.Fatalf("aggregated usage = %+v, want tokens=70 known USD=2 and incomplete pricing", usage)
		}
	})

	t.Run("Should not mutate copied snapshot USD when accumulating new receipt", func(t *testing.T) {
		t.Parallel()
		usage := DailyBudgetUsage{Tokens: 10, USD: big.NewRat(3, 2)}
		snapshot := usage
		if err := snapshot.AddReceipt(20, new(0.5)); err != nil {
			t.Fatalf("AddReceipt() error = %v", err)
		}
		if usage.Tokens != 10 || usage.USD == nil || usage.USD.Cmp(big.NewRat(3, 2)) != 0 {
			t.Fatalf("original usage mutated after speculative copy: %+v", usage)
		}
	})

	t.Run("Should reject a receipt against a negative restored token balance", func(t *testing.T) {
		t.Parallel()
		usage := DailyBudgetUsage{Tokens: -1}
		if err := usage.AddReceipt(100, nil); err == nil || usage.Tokens != -1 {
			t.Fatalf("invalid restored token balance accepted: usage=%+v err=%v", usage, err)
		}
	})

	t.Run("Should reject a receipt against a negative restored USD balance", func(t *testing.T) {
		t.Parallel()
		usage := DailyBudgetUsage{USD: big.NewRat(-1, 1)}
		if err := usage.AddReceipt(0, new(2.0)); err == nil || usage.USD.Cmp(big.NewRat(-1, 1)) != 0 {
			t.Fatalf("invalid restored USD balance accepted: usage=%+v err=%v", usage, err)
		}
	})

	t.Run("Should preserve very small receipts against a large existing subtotal", func(t *testing.T) {
		t.Parallel()
		usage := DailyBudgetUsage{}
		if err := usage.AddReceipt(0, new(math.Nextafter(9e9, 0))); err != nil {
			t.Fatalf("initial AddReceipt() error = %v", err)
		}
		for range 20 {
			if err := usage.AddReceipt(1, new(1e-7)); err != nil {
				t.Fatalf("AddReceipt() error = %v", err)
			}
		}
		usdLimit := 9e9
		assessment := mustEvaluateDailyBudget(t, DailyBudgetLimits{USD: &usdLimit}, usage, false)
		if !assessment.USDExhausted {
			t.Fatalf("lost tiny cost receipts: usage=%+v assessment=%+v", usage, assessment)
		}
	})

	t.Run("Should preserve large cost sums without float overflow", func(t *testing.T) {
		t.Parallel()
		receipt := math.Ldexp(3, 970)
		usage := DailyBudgetUsage{}
		if err := usage.AddReceipt(0, new(math.MaxFloat64-receipt)); err != nil {
			t.Fatalf("initial AddReceipt() error = %v", err)
		}
		prior := new(big.Rat).Set(usage.USD)
		if err := usage.AddReceipt(1, &receipt); err != nil || usage.USD.Cmp(prior) <= 0 {
			t.Fatalf("AddReceipt() lost a large positive receipt: usage=%+v err=%v", usage, err)
		}
	})

	for _, tc := range []struct {
		name   string
		tokens int64
		usd    *float64
	}{
		{name: "Should reject negative token receipts", tokens: -1},
		{name: "Should reject negative cost receipts", usd: new(-0.01)},
		{name: "Should reject nonfinite cost receipts", usd: new(math.Inf(1))},
		{name: "Should reject overflowing token totals", tokens: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			usage := DailyBudgetUsage{}
			if tc.name == "Should reject overflowing token totals" {
				usage.Tokens = math.MaxInt64
			}
			if err := usage.AddReceipt(tc.tokens, tc.usd); err == nil {
				t.Fatalf("AddReceipt(%d, %v) accepted invalid receipt", tc.tokens, tc.usd)
			}
		})
	}

	t.Run("Should keep totals unchanged when a priced receipt is rejected", func(t *testing.T) {
		t.Parallel()
		usage := DailyBudgetUsage{Tokens: 50, USD: big.NewRat(1, 1)}
		if err := usage.AddReceipt(10, new(math.NaN())); err == nil {
			t.Fatal("AddReceipt() accepted NaN cost")
		}
		if usage.Tokens != 50 || usage.USD.Cmp(big.NewRat(1, 1)) != 0 {
			t.Fatalf("rejected receipt mutated totals: %+v", usage)
		}
	})
}

func mustEvaluateDailyBudget(
	t *testing.T, limits DailyBudgetLimits, usage DailyBudgetUsage, warned bool,
) DailyBudgetAssessment {
	t.Helper()
	assessment, err := EvaluateDailyBudget(limits, usage, warned)
	if err != nil {
		t.Fatalf("EvaluateDailyBudget() error = %v", err)
	}
	return assessment
}
