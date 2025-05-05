package common

// PackageRef represents a reference to a package
type PackageRef struct {
	Name    string `json:"name" yaml:"name"`
	Version string `json:"version" yaml:"version"`
}

// Error represents a parser error
type Error struct {
	Message string `json:"message" yaml:"message"`
	Code    string `json:"code" yaml:"code"`
}

// Transition represents a state transition
type Transition struct {
	To    string `json:"to" yaml:"to"`
	When  string `json:"when" yaml:"when"`
	Error *Error `json:"error,omitempty" yaml:"error,omitempty"`
}

// Metadata represents common metadata fields
type Metadata struct {
	Name        string            `json:"name" yaml:"name"`
	Description string            `json:"description" yaml:"description"`
	Tags        []string          `json:"tags,omitempty" yaml:"tags,omitempty"`
	Labels      map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
}
