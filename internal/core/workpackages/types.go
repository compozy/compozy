// Package workpackages owns the canonical optional Work Package domain.
package workpackages

const (
	// ManifestFileName is the opt-in marker and canonical Work Package plan.
	ManifestFileName = "_work_packages.md"
	// SchemaVersion is the only supported Work Package manifest schema.
	SchemaVersion = "compozy.work-packages/v1"
)

// Ref identifies either an initiative or a Work Package beneath an initiative.
type Ref struct {
	Initiative string
	PackageID  string
}

// String returns the canonical public reference.
func (r Ref) String() string {
	if r.PackageID == "" {
		return r.Initiative
	}
	return r.Initiative + "/" + r.PackageID
}

// IsPackage reports whether the reference selects a package.
func (r Ref) IsPackage() bool {
	return r.PackageID != ""
}

// TargetMode describes how an initiative reference resolved.
type TargetMode string

const (
	// TargetModeOrdinary is a workflow without a Work Package marker.
	TargetModeOrdinary TargetMode = "ordinary"
	// TargetModeInitiative is an opted-in initiative without a selected package.
	TargetModeInitiative TargetMode = "initiative"
	// TargetModePackage is a selected Work Package.
	TargetModePackage TargetMode = "package"
	// TargetModeInvalidOptIn is a present marker that failed canonical validation.
	TargetModeInvalidOptIn TargetMode = "invalid_opt_in"
)

// Target is a containment-checked execution target.
type Target struct {
	Mode          TargetMode
	Ref           Ref
	DisplayRef    string
	InitiativeDir string
	SpecDir       string
	PackageDir    string
	TasksDir      string
	ReviewsDir    string
	MemoryDir     string
	Plan          Plan
	Package       Package
}

// Package is one stable Work Package in a plan.
type Package struct {
	ID           string
	Title        string
	Outcome      string
	Reference    string
	Directory    string
	Completed    bool
	Dependencies []Dependency
	OwnedScope   []string
}

// Dependency is a directed prerequisite relationship from From to To.
type Dependency struct {
	From      string
	To        string
	Rationale string
}

// DependencyPath describes an unmet transitive prerequisite chain.
type DependencyPath struct {
	PackageIDs []string
	Edges      []Dependency
}

// Readiness is the current dependency state of one package.
type Readiness struct {
	Eligible         bool
	DirectUnmet      []Dependency
	TransitiveUnmet  []DependencyPath
	IndependentPeers []string
}

// Plan is a validated canonical Work Package manifest.
type Plan struct {
	SchemaVersion string
	Initiative    string
	Packages      []Package
	Edges         []Dependency
	Path          string
	Checksum      string

	raw []byte
}

// Package returns a package by its stable ID.
func (p Plan) Package(id string) (Package, bool) {
	for index := range p.Packages {
		pkg := &p.Packages[index]
		if pkg.ID == id {
			return *pkg, true
		}
	}
	return Package{}, false
}

// PackageIDs returns stable IDs in deterministic order.
func (p Plan) PackageIDs() []string {
	ids := make([]string, 0, len(p.Packages))
	for index := range p.Packages {
		pkg := &p.Packages[index]
		ids = append(ids, pkg.ID)
	}
	return ids
}

// IsComplete reports whether a stable package ID has a checked Markdown heading.
func (p Plan) IsComplete(id string) bool {
	pkg, ok := p.Package(id)
	return ok && pkg.Completed
}

// Issue identifies one actionable plan or ownership validation failure.
type Issue struct {
	Path    string
	Field   string
	Message string
}
