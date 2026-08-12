package command

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"strings"
)

const (
	goalCommandName     = "goal"
	worktreeCommandName = "worktree"
	runCommandName      = "run"
)

var errCanonicalNameRequired = errors.New("command: canonical name is required")

// BuildCatalog merges daemon, ACP, and skill commands using the accepted precedence rules.
func BuildCatalog(builtins []BuiltinSpec, agents []AgentSpec, skills []SkillSpec) (Catalog, error) {
	descriptors := make([]Descriptor, 0, len(builtins)+len(agents)+len(skills))
	claimedBare := map[string]struct{}{runCommandName: {}}
	unavailable := map[string]struct{}{slashToken(runCommandName): {}}

	for _, builtin := range builtins {
		descriptor, err := builtinDescriptor(builtin)
		if err != nil {
			return Catalog{}, err
		}
		claimedBare[strings.TrimPrefix(descriptor.CanonicalToken, "/")] = struct{}{}
		if !builtin.Available {
			unavailable[descriptor.CanonicalToken] = struct{}{}
			if strings.TrimSpace(builtin.UnavailableReason) == "" {
				continue
			}
		}
		descriptors = append(descriptors, descriptor)
	}

	for _, agent := range agents {
		descriptor, err := agentDescriptor(agent)
		if err != nil {
			return Catalog{}, err
		}
		name := strings.TrimPrefix(descriptor.CanonicalToken, "/")
		if _, exists := claimedBare[name]; exists {
			continue
		}
		claimedBare[name] = struct{}{}
		descriptors = append(descriptors, descriptor)
	}

	seenSkillIDs := make(map[string]struct{}, len(skills))
	seenSkillTokens := make(map[string]struct{}, len(skills))
	for _, skill := range skills {
		descriptor, err := skillDescriptor(skill)
		if err != nil {
			return Catalog{}, err
		}
		if _, duplicate := seenSkillIDs[descriptor.ID]; duplicate {
			return Catalog{}, fmt.Errorf("command: duplicate skill command %q", descriptor.ID)
		}
		seenSkillIDs[descriptor.ID] = struct{}{}
		if _, duplicate := seenSkillTokens[descriptor.CanonicalToken]; duplicate {
			return Catalog{}, fmt.Errorf("command: duplicate skill token %q", descriptor.CanonicalToken)
		}
		seenSkillTokens[descriptor.CanonicalToken] = struct{}{}
		if !skill.Qualified {
			name := strings.TrimPrefix(descriptor.CanonicalToken, "/")
			if _, exists := claimedBare[name]; exists {
				continue
			}
			claimedBare[name] = struct{}{}
		}
		if !skill.Available {
			unavailable[descriptor.CanonicalToken] = struct{}{}
			continue
		}
		descriptors = append(descriptors, descriptor)
	}

	slices.SortStableFunc(descriptors, compareDescriptors)
	return Catalog{
		Commands:    descriptors,
		Revision:    catalogRevision(descriptors),
		unavailable: unavailable,
	}, nil
}

// DefaultBuiltins returns the complete reserved daemon command set.
func DefaultBuiltins() []BuiltinSpec {
	return []BuiltinSpec{
		{
			Name:        goalCommandName,
			Description: "Set, inspect, pause, resume, replace, or clear the session Goal.",
			InputHint:   "objective or Goal command",
			Available:   true,
		},
		{
			Name:        worktreeCommandName,
			Description: "Create a new session in a worktree.",
			Available:   true,
		},
		{Name: runCommandName, Available: false},
	}
}

func builtinDescriptor(spec BuiltinSpec) (Descriptor, error) {
	name := Slug(spec.Name)
	if name == "" {
		return Descriptor{}, errCanonicalNameRequired
	}
	return Descriptor{
		ID:                "builtin:" + name,
		CanonicalToken:    slashToken(name),
		DisplayName:       "/" + name,
		Description:       strings.TrimSpace(spec.Description),
		Lane:              LaneBuiltin,
		Source:            Source{Kind: "builtin", Scope: "session"},
		Placements:        []Placement{PlacementStandalone},
		InputHint:         strings.TrimSpace(spec.InputHint),
		Available:         spec.Available,
		UnavailableReason: strings.TrimSpace(spec.UnavailableReason),
	}, nil
}

func agentDescriptor(spec AgentSpec) (Descriptor, error) {
	name := Slug(spec.Name)
	if name == "" {
		return Descriptor{}, errCanonicalNameRequired
	}
	return Descriptor{
		ID:             "agent:" + name,
		CanonicalToken: slashToken(name),
		DisplayName:    "/" + name,
		Description:    strings.TrimSpace(spec.Description),
		Lane:           LaneAgent,
		Source:         Source{Kind: "acp", ID: Slug(spec.SourceID), Scope: "session"},
		Placements:     []Placement{PlacementStandalone},
		InputHint:      strings.TrimSpace(spec.InputHint),
		Available:      true,
	}, nil
}

func skillDescriptor(spec SkillSpec) (Descriptor, error) {
	name := Slug(spec.Name)
	if name == "" {
		return Descriptor{}, errCanonicalNameRequired
	}
	sourceKind := Slug(spec.Source.Kind)
	if sourceKind == "" {
		return Descriptor{}, errors.New("command: skill source kind is required")
	}
	sourceID := Slug(spec.Source.ID)
	sourceKey := strings.TrimSpace(spec.Source.Key)
	canonicalName := name
	if spec.Qualified {
		if sourceID == "" {
			return Descriptor{}, errors.New("command: qualified skill source id is required")
		}
		canonicalName = sourceID + ":" + name
	}
	identity := sourceID
	if sourceKey != "" {
		hash := sha256.Sum256([]byte(sourceKey))
		identity = sourceID + "-" + hex.EncodeToString(hash[:8])
	}
	commandID := strings.Join([]string{"skill", sourceKind, identity, name}, ":")
	source := spec.Source
	source.Kind = sourceKind
	source.ID = sourceID
	source.Key = sourceKey
	ref := SkillRef{CommandID: commandID, Name: strings.TrimSpace(spec.Name), Source: source}
	return Descriptor{
		ID:             commandID,
		CanonicalToken: slashToken(canonicalName),
		DisplayName:    "/" + name,
		Description:    strings.TrimSpace(spec.Description),
		Lane:           LaneSkill,
		Source:         source,
		Placements:     []Placement{PlacementStandalone, PlacementInline},
		Skill:          &ref,
		Available:      true,
	}, nil
}

func compareDescriptors(left Descriptor, right Descriptor) int {
	leftLane := laneOrder(left.Lane)
	rightLane := laneOrder(right.Lane)
	if leftLane != rightLane {
		return leftLane - rightLane
	}
	return strings.Compare(left.CanonicalToken, right.CanonicalToken)
}

func laneOrder(lane Lane) int {
	switch lane {
	case LaneBuiltin:
		return 0
	case LaneAgent:
		return 1
	case LaneSkill:
		return 2
	default:
		return 3
	}
}

func catalogRevision(descriptors []Descriptor) string {
	var material strings.Builder
	for _, descriptor := range descriptors {
		material.WriteString(descriptor.ID)
		material.WriteByte(0)
		material.WriteString(descriptor.CanonicalToken)
		material.WriteByte(0)
		material.WriteString(descriptor.DisplayName)
		material.WriteByte(0)
		material.WriteString(descriptor.Description)
		material.WriteByte(0)
		material.WriteString(string(descriptor.Lane))
		material.WriteByte(0)
		material.WriteString(descriptor.Source.Kind)
		material.WriteByte(0)
		material.WriteString(descriptor.Source.ID)
		material.WriteByte(0)
		material.WriteString(descriptor.Source.Key)
		material.WriteByte(0)
		material.WriteString(descriptor.Source.Scope)
		material.WriteByte(0)
		for _, placement := range descriptor.Placements {
			material.WriteString(string(placement))
			material.WriteByte(0)
		}
		material.WriteString(descriptor.InputHint)
		material.WriteByte(0)
		if descriptor.Available {
			material.WriteByte(1)
		} else {
			material.WriteByte(2)
		}
		material.WriteString(descriptor.UnavailableReason)
		material.WriteByte(0)
	}
	hash := sha256.Sum256([]byte(material.String()))
	return hex.EncodeToString(hash[:])
}

// SetBuiltinAvailability updates one visible daemon-owned command while
// preserving catalog ordering and parser refusal state.
func SetBuiltinAvailability(catalog Catalog, name string, available bool, reason string) Catalog {
	id := "builtin:" + Slug(name)
	commands := append([]Descriptor(nil), catalog.Commands...)
	unavailable := make(map[string]struct{}, len(catalog.unavailable)+1)
	for token := range catalog.unavailable {
		unavailable[token] = struct{}{}
	}
	for index := range commands {
		if commands[index].ID != id {
			continue
		}
		commands[index].Available = available
		commands[index].UnavailableReason = strings.TrimSpace(reason)
		if available {
			delete(unavailable, commands[index].CanonicalToken)
		} else {
			unavailable[commands[index].CanonicalToken] = struct{}{}
		}
		break
	}
	catalog.Commands = commands
	catalog.unavailable = unavailable
	catalog.Revision = catalogRevision(commands)
	return catalog
}

func slashToken(name string) string {
	return "/" + name
}
