package author

// Contributor represents a contributor to the project
type Contributor struct {
	Name         string `json:"name" yaml:"name"`
	Email        string `json:"email,omitempty" yaml:"email,omitempty"`
	URL          string `json:"url,omitempty" yaml:"url,omitempty"`
	Organization string `json:"organization,omitempty" yaml:"organization,omitempty"`
}

// Author represents an author of the project
type Author struct {
	Name         string        `json:"name" yaml:"name"`
	Email        string        `json:"email,omitempty" yaml:"email,omitempty"`
	URL          string        `json:"url,omitempty" yaml:"url,omitempty"`
	Organization string        `json:"organization,omitempty" yaml:"organization,omitempty"`
	Contributors []Contributor `json:"contributors,omitempty" yaml:"contributors,omitempty"`
}
