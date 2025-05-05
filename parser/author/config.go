package author

// Contributor represents a contributor to the project
type Contributor struct {
	Name         AuthorName          `json:"name" yaml:"name"`
	Email        *AuthorEmail        `json:"email,omitempty" yaml:"email,omitempty"`
	URL          *AuthorURL          `json:"url,omitempty" yaml:"url,omitempty"`
	Organization *AuthorOrganization `json:"organization,omitempty" yaml:"organization,omitempty"`
}

// Author represents an author of the project
type Author struct {
	Name         AuthorName          `json:"name" yaml:"name"`
	Email        *AuthorEmail        `json:"email,omitempty" yaml:"email,omitempty"`
	URL          *AuthorURL          `json:"url,omitempty" yaml:"url,omitempty"`
	Organization *AuthorOrganization `json:"organization,omitempty" yaml:"organization,omitempty"`
	Contributors []Contributor       `json:"contributors,omitempty" yaml:"contributors,omitempty"`
}
