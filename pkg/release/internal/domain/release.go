package domain

// Release holds all metadata related to a release.

type Release struct {
	Version    *Version
	Changelog  string
	BranchName string
	TagName    string
	PRBody     string
}
