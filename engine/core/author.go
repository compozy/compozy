package core

// Contributor represents an individual who contributed to the project.
//
// Contributors are team members, collaborators, or external developers
// who helped build, maintain, or improve the project. This enables
// proper attribution and team recognition in project documentation.
//
// Example contributor entry:
//
// ```yaml
// contributors:
//   - name: "John Doe"
//     email: "john@company.com"
//     url: "https://github.com/johndoe"
//     organization: "Engineering Team"
//
// ```
type Contributor struct {
	// Full name of the contributor.
	Name string `json:"name" yaml:"name" mapstructure:"name"`

	// Email address for contributor contact.
	Email string `json:"email,omitempty" yaml:"email,omitempty" mapstructure:"email,omitempty"`

	// URL to contributor's profile or portfolio.
	//
	// Examples: `"https://github.com/username"`, `"https://linkedin.com/in/name"`
	URL string `json:"url,omitempty" yaml:"url,omitempty" mapstructure:"url,omitempty"`

	// Organization or team the contributor belongs to.
	Organization string `json:"organization,omitempty" yaml:"organization,omitempty" mapstructure:"organization,omitempty"`
}

// Author represents project authorship and contributor information.
//
// **Project authors** define ownership, contact information, and attribution
// for Compozy projects. This enables:
//
// - **Clear ownership** for project maintenance and support
// - **Contact information** for collaboration and issues
// - **Attribution** for proper crediting in documentation
// - **Team management** through contributor tracking
//
// ## Usage Examples
//
// **Individual Author:**
//
// ```yaml
// author:
//
//	name: "Jane Smith"
//	email: "jane@company.com"
//	organization: "ACME AI Division"
//
// ```
//
// **Team with Contributors:**
//
// ```yaml
// author:
//
//	name: "AI Platform Team"
//	email: "ai-team@company.com"
//	url: "https://github.com/company/ai-team"
//	organization: "ACME Corporation"
//	contributors:
//	  - name: "John Doe"
//	    email: "john@company.com"
//	    organization: "Engineering"
//	  - name: "Alice Chen"
//	    email: "alice@company.com"
//	    url: "https://github.com/alice-chen"
//
// ```
//
// ## Best Practices
//
// - **Use team emails** for shared project ownership
// - **Include GitHub URLs** for easy contributor identification
// - **Specify organizations** for enterprise project attribution
// - **Keep contributor lists** up-to-date for active projects
type Author struct {
	// Name of the author or team responsible for the project.
	//
	// Examples: `"Jane Smith"`, `"AI Platform Team"`, `"Data Science Division"`
	Name string `json:"name" yaml:"name" mapstructure:"name"`

	// Email contact for project-related communication.
	//
	// Use team emails for shared ownership: `"ai-team@company.com"`
	Email string `json:"email,omitempty" yaml:"email,omitempty" mapstructure:"email,omitempty"`

	// URL to author's profile, repository, or team page.
	//
	// Examples: `"https://github.com/username"`, `"https://company.com/team/ai"`
	URL string `json:"url,omitempty" yaml:"url,omitempty" mapstructure:"url,omitempty"`

	// Organization or company affiliation.
	//
	// Examples: `"ACME Corporation"`, `"AI Research Lab"`, `"Engineering Division"`
	Organization string `json:"organization,omitempty" yaml:"organization,omitempty" mapstructure:"organization,omitempty"`

	// Additional contributors who helped develop the project.
	//
	// Use this to acknowledge team members, collaborators, or external contributors.
	Contributors []Contributor `json:"contributors,omitempty" yaml:"contributors,omitempty" mapstructure:"contributors,omitempty"`
}
