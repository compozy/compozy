package controller

import (
	"context"

	"errors"
	"fmt"

	"regexp"

	"strings"
	"time"

	memcontract "github.com/compozy/agh/internal/memory/contract"
	"github.com/compozy/agh/internal/memory/prompts"
	"github.com/compozy/agh/internal/memory/scan"
)

const (
	defaultPromptVersion       = prompts.VersionV1
	metadataOperationKey       = "operation"
	metadataOpKey              = "op"
	metadataFilenameKey        = "filename"
	metadataTargetFilenameKey  = "target_filename"
	metadataRawContentKey      = "raw_content"
	metadataReasonKey          = "reason"
	metadataSourceKey          = "source"
	metadataTargetEntityKey    = "target_entity"
	metadataTargetAttributeKey = "target_attribute"
	minSurfaceOverlap          = 2
	maxDecisionReasonBytes     = 240
)

var (
	filenameUnsafePattern = regexp.MustCompile(`[^a-z0-9]+`)
	errRawContentMetadata = errors.New("memory controller: raw_content metadata is not allowed")
)

// Target is one existing curated memory entry visible to the controller.
type Target struct {
	ID             string
	WorkspaceID    string
	Scope          memcontract.Scope
	AgentName      string
	AgentTier      memcontract.AgentTier
	TargetFilename string
	Frontmatter    memcontract.Header
	Entity         string
	Attribute      string
	Content        string
	RawContent     string
	ContentHash    string
	LastUpdatedAt  time.Time
}

// TargetIndex supplies current curated memory candidates for rule decisions.
type TargetIndex interface {
	ListTargets(ctx context.Context, candidate memcontract.Candidate) ([]Target, error)
}

// Controller decides Memory v2 write outcomes with deterministic Slice 1 rules.
type Controller struct {
	index           TargetIndex
	now             func() time.Time
	promptVersion   string
	tiebreaker      Tiebreaker
	defaultOpOnFail memcontract.Op
}

// Option customizes a Controller.
type Option func(*Controller)

// WithClock injects a deterministic clock for tests.
func WithClock(now func() time.Time) Option {
	return func(c *Controller) {
		if now != nil {
			c.now = now
		}
	}
}

// WithPromptVersion pins the decision prompt version recorded in WAL rows.
func WithPromptVersion(version string) Option {
	return func(c *Controller) {
		if strings.TrimSpace(version) != "" {
			c.promptVersion = strings.TrimSpace(version)
		}
	}
}

// WithTiebreaker installs the bounded model call used only for genuine ambiguity.
func WithTiebreaker(tiebreaker Tiebreaker) Option {
	return func(c *Controller) {
		c.tiebreaker = tiebreaker
	}
}

// WithDefaultOpOnFail selects the deterministic operation used when the
// configured tiebreaker fails. Only noop and reject are safe failure outcomes.
func WithDefaultOpOnFail(op memcontract.Op) Option {
	return func(c *Controller) {
		switch normalized := op.Normalize(); normalized {
		case memcontract.OpNoop, memcontract.OpReject:
			c.defaultOpOnFail = normalized
		}
	}
}

// New constructs a rule-first controller over the provided target index.
func New(index TargetIndex, opts ...Option) *Controller {
	c := &Controller{
		index:           index,
		defaultOpOnFail: memcontract.OpNoop,
		now: func() time.Time {
			return time.Now().UTC()
		},
		promptVersion: defaultPromptVersion,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(c)
		}
	}
	return c
}

// Decide returns a deterministic Decision. Mutation and WAL persistence are owned by Store.ApplyDecision.
func (c *Controller) Decide(ctx context.Context, candidate memcontract.Candidate) (memcontract.Decision, error) {
	normalized, targets, err := c.prepareDecision(ctx, candidate)
	if err != nil {
		return memcontract.Decision{}, err
	}

	scanResult := scan.Candidate(normalized)
	trace := scanRuleHits(scanResult, normalized)
	if scanResult.Rejected() {
		return c.rejectDecision(normalized, scanResult, trace)
	}
	if opFromCandidate(normalized) == memcontract.OpDelete {
		return c.deleteDecision(ctx, normalized, targets, trace)
	}
	if exact := exactContentTarget(normalized, targets); exact != nil {
		return c.decision(
			normalized,
			memcontract.OpNoop,
			[]Target{*exact},
			exact.TargetFilename,
			"",
			append(trace, passedRule("exact_hash", "candidate content already exists", exact.ID)),
			"exact duplicate memory content",
			nil,
		)
	}
	return c.writeDecision(ctx, normalized, targets, trace)
}

func (c *Controller) prepareDecision(
	ctx context.Context,
	candidate memcontract.Candidate,
) (memcontract.Candidate, []Target, error) {
	if ctx == nil {
		return memcontract.Candidate{}, nil, errors.New("memory controller: context is required")
	}
	if c == nil {
		return memcontract.Candidate{}, nil, errors.New("memory controller: controller is required")
	}
	normalized, err := normalizeCandidate(candidate, c.now())
	if err != nil {
		return memcontract.Candidate{}, nil, err
	}
	if err := ctx.Err(); err != nil {
		return memcontract.Candidate{}, nil, fmt.Errorf("memory controller: decide canceled: %w", err)
	}

	targets, err := c.targets(ctx, normalized)
	if err != nil {
		return memcontract.Candidate{}, nil, err
	}
	return normalized, targets, nil
}

func (c *Controller) rejectDecision(
	candidate memcontract.Candidate,
	scanResult scan.Result,
	trace []memcontract.RuleHit,
) (memcontract.Decision, error) {
	return c.decision(
		candidate,
		memcontract.OpReject,
		nil,
		"",
		"",
		trace,
		scanReason(scanResult, candidate),
		nil,
	)
}

func (c *Controller) writeDecision(
	ctx context.Context,
	normalized memcontract.Candidate,
	targets []Target,
	trace []memcontract.RuleHit,
) (memcontract.Decision, error) {
	if collision := exactFilenameTarget(normalized, targets); collision != nil {
		return c.updateDecision(normalized, *collision, append(
			trace,
			passedRule("exact_slug_collision", "target filename already exists", collision.ID),
		))
	}
	slotMatches := entitySlotTargets(normalized, targets)
	switch len(slotMatches) {
	case 0:
		if surface := surfaceTargets(normalized, targets); len(surface) > 1 &&
			normalized.Origin.Normalize() != memcontract.OriginDreaming {
			if !isAutonomousWriteOrigin(normalized.Origin) &&
				directTargetIdentifiesNewMemory(normalized, surface) {
				return c.addDecision(normalized, append(
					trace,
					passedRule(
						"explicit_target_filename",
						"direct write supplied a distinct target filename",
						targetFilename(normalized),
					),
				))
			}
			return c.ambiguousDecision(ctx, normalized, surface, trace)
		}
		return c.addDecision(normalized, trace)
	case 1:
		target := slotMatches[0]
		if equalMemoryBody(normalized.Content, target.Content) {
			return c.decision(
				normalized,
				memcontract.OpNoop,
				[]Target{target},
				target.TargetFilename,
				"",
				append(
					trace,
					passedRule("entity_slot_no_change", "slot target content is unchanged", target.ID),
				),
				"entity slot already has this content",
				nil,
			)
		}
		return c.updateDecision(normalized, target, append(
			trace,
			passedRule("entity_slot_update", "single entity slot target differs", target.ID),
		))
	default:
		if !isAutonomousWriteOrigin(normalized.Origin) &&
			directTargetIdentifiesNewMemory(normalized, slotMatches) {
			return c.addDecision(normalized, append(
				trace,
				passedRule(
					"explicit_target_filename",
					"direct write supplied a distinct target filename",
					targetFilename(normalized),
				),
			))
		}
		return c.ambiguousDecision(ctx, normalized, slotMatches, trace)
	}
}
