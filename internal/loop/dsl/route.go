package dsl

// RouteSpec is one ordered predicate-to-destination mapping on a route node.
type RouteSpec struct {
	When string `json:"when" yaml:"when"`
	To   NodeID `json:"to"   yaml:"to"`
}
