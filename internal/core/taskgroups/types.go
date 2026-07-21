// TaskGroup taskgroups owns the canonical optional Task Group domain.
package taskgroups

const (
	// ManifestFileName is the opt-in marker and canonical Task Group plan.
	ManifestFileName = "_task_groups.md"
	// SchemaVersion is the only supported Task Group manifest schema.
	SchemaVersion = "compozy.task-groups/v1"
)

// Ref identifies either an initiative or a Task Group beneath an initiative.
type Ref struct {
	Initiative  string
	TaskGroupID string
}

// String returns the canonical public reference.
func (r Ref) String() string {
	if r.TaskGroupID == "" {
		return r.Initiative
	}
	return r.Initiative + "/" + r.TaskGroupID
}

// IsTaskGroup reports whether the reference selects a task group.
func (r Ref) IsTaskGroup() bool {
	return r.TaskGroupID != ""
}

// TargetMode describes how an initiative reference resolved.
type TargetMode string

const (
	// TargetModeOrdinary is a workflow without a Task Group marker.
	TargetModeOrdinary TargetMode = "ordinary"
	// TargetModeInitiative is an opted-in initiative without a selected task group.
	TargetModeInitiative TargetMode = "initiative"
	// TargetModeTaskGroup is a selected Task Group.
	TargetModeTaskGroup TargetMode = "task group"
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
	TaskGroupDir  string
	TasksDir      string
	ReviewsDir    string
	MemoryDir     string
	Plan          Plan
	TaskGroup     TaskGroup
}

// TaskGroup is one stable Task Group in a plan.
type TaskGroup struct {
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
	TaskGroupIDs []string
	Edges        []Dependency
}

// Readiness is the current dependency state of one task group.
type Readiness struct {
	Eligible         bool
	DirectUnmet      []Dependency
	TransitiveUnmet  []DependencyPath
	IndependentPeers []string
}

// Plan is a validated canonical Task Group manifest.
type Plan struct {
	SchemaVersion string
	Initiative    string
	TaskGroups    []TaskGroup
	Edges         []Dependency
	Path          string
	Checksum      string

	raw []byte
}

// TaskGroup returns a task group by its stable ID.
func (p Plan) TaskGroup(id string) (TaskGroup, bool) {
	for index := range p.TaskGroups {
		taskGroup := &p.TaskGroups[index]
		if taskGroup.ID == id {
			return *taskGroup, true
		}
	}
	return TaskGroup{}, false
}

// TaskGroupIDs returns stable IDs in deterministic order.
func (p Plan) TaskGroupIDs() []string {
	ids := make([]string, 0, len(p.TaskGroups))
	for index := range p.TaskGroups {
		taskGroup := &p.TaskGroups[index]
		ids = append(ids, taskGroup.ID)
	}
	return ids
}

// IsComplete reports whether a stable task group ID has a checked Markdown heading.
func (p Plan) IsComplete(id string) bool {
	taskGroup, ok := p.TaskGroup(id)
	return ok && taskGroup.Completed
}

// Issue identifies one actionable plan or ownership validation failure.
type Issue struct {
	Path    string
	Field   string
	Message string
}
