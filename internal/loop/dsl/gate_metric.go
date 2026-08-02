package dsl

// MetricDirection determines whether a metric score should increase or decrease.
type MetricDirection string

const (
	// MetricMaximize accepts scores that increase by the configured delta.
	MetricMaximize MetricDirection = "maximize"
	// MetricMinimize accepts scores that decrease by the configured delta.
	MetricMinimize MetricDirection = "minimize"
)

// MetricSpec declares the score contract for one machine criterion.
type MetricSpec struct {
	Direction MetricDirection `json:"direction" yaml:"direction"`

	// MinDelta is the optional non-negative improvement required beyond the current best score.
	// Nil accepts any strict directional improvement; zero preserves strict comparison semantics.
	MinDelta *float64 `json:"min_delta,omitempty" yaml:"min_delta,omitempty"`
}
