package bridges

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"slices"
	"strings"
	"time"

	"github.com/compozy/agh/internal/resources"
)

// ResourceProjectionStore is the bridge desired-runtime surface updated by resource projection.
type ResourceProjectionStore interface {
	ListBridgeInstances(ctx context.Context) ([]BridgeInstance, error)
	ReplaceBridgeInstances(ctx context.Context, instances []BridgeInstance) error
}

// ResourceProjectionPlan is the validated bridge.instance delta built from canonical resources.
type ResourceProjectionPlan struct {
	revision          int64
	operations        int
	previous          []BridgeInstance
	next              []BridgeInstance
	changedExtensions []string
}

var _ resources.ProjectionPlan = (*ResourceProjectionPlan)(nil)

// Kind returns the projected resource kind.
func (p *ResourceProjectionPlan) Kind() resources.ResourceKind {
	return BridgeInstanceResourceKind
}

// Revision returns the highest source resource version represented by this plan.
func (p *ResourceProjectionPlan) Revision() int64 {
	if p == nil {
		return 0
	}
	return p.revision
}

// OperationCount returns the number of runtime rows that change when this plan applies.
func (p *ResourceProjectionPlan) OperationCount() int {
	if p == nil {
		return 0
	}
	return p.operations
}

// PreviousInstances returns the daemon-visible bridge state before this plan applies.
func (p *ResourceProjectionPlan) PreviousInstances() []BridgeInstance {
	if p == nil {
		return nil
	}
	return cloneBridgeInstances(p.previous)
}

// NextInstances returns the daemon-visible bridge state after this plan applies.
func (p *ResourceProjectionPlan) NextInstances() []BridgeInstance {
	if p == nil {
		return nil
	}
	return cloneBridgeInstances(p.next)
}

// ChangedExtensions returns the provider extensions impacted by this plan.
func (p *ResourceProjectionPlan) ChangedExtensions() []string {
	if p == nil {
		return nil
	}
	return append([]string(nil), p.changedExtensions...)
}

// RollbackPlan returns a plan that restores the prior daemon-visible bridge state.
func (p *ResourceProjectionPlan) RollbackPlan() *ResourceProjectionPlan {
	if p == nil {
		return nil
	}
	currentByID := bridgeInstancesByID(p.next)
	rollbackByID := bridgeInstancesByID(p.previous)
	return &ResourceProjectionPlan{
		revision:          p.revision,
		operations:        bridgeProjectionOperationCountByID(currentByID, rollbackByID),
		previous:          cloneBridgeInstances(p.next),
		next:              cloneBridgeInstances(p.previous),
		changedExtensions: append([]string(nil), p.changedExtensions...),
	}
}

// BuildResourceState computes the next bridge runtime projection without opening live provider connections.
func BuildResourceState(
	ctx context.Context,
	store ResourceProjectionStore,
	records []resources.Record[BridgeInstanceSpec],
	now func() time.Time,
) (*ResourceProjectionPlan, error) {
	if ctx == nil {
		return nil, errors.New("bridges: bridge resource build context is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if store == nil {
		return nil, errors.New("bridges: bridge resource projection store is required")
	}

	previous, err := store.ListBridgeInstances(ctx)
	if err != nil {
		return nil, fmt.Errorf("bridges: build bridge resource state: list existing instances: %w", err)
	}
	sortBridgeInstances(previous)
	previousByID := bridgeInstancesByID(previous)

	next := make([]BridgeInstance, 0, len(records))
	seen := make(map[string]struct{}, len(records))
	var revision int64
	for _, record := range records {
		if record.Version > revision {
			revision = record.Version
		}
		id := strings.TrimSpace(record.ID)
		if id == "" {
			return nil, errors.New("bridges: bridge resource record id is required")
		}
		if _, ok := seen[id]; ok {
			return nil, fmt.Errorf("bridges: duplicate bridge resource record %q", id)
		}
		seen[id] = struct{}{}

		var existing *BridgeInstance
		if previousInstance, ok := previousByID[id]; ok {
			existing = cloneBridgeInstance(previousInstance)
		}
		instance, err := bridgeInstanceFromResourceRecord(record, existing, now)
		if err != nil {
			return nil, fmt.Errorf("bridges: build bridge resource state for %q: %w", id, err)
		}
		next = append(next, instance)
	}
	sortBridgeInstances(next)
	nextByID := bridgeInstancesByID(next)

	return &ResourceProjectionPlan{
		revision:          revision,
		operations:        bridgeProjectionOperationCountByID(previousByID, nextByID),
		previous:          cloneBridgeInstances(previous),
		next:              cloneBridgeInstances(next),
		changedExtensions: changedBridgeProjectionExtensionsByID(previousByID, nextByID),
	}, nil
}

// ApplyResourceState atomically swaps the daemon-visible bridge desired runtime state.
func ApplyResourceState(ctx context.Context, store ResourceProjectionStore, plan resources.ProjectionPlan) error {
	if ctx == nil {
		return errors.New("bridges: bridge resource apply context is required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if store == nil {
		return errors.New("bridges: bridge resource projection store is required")
	}

	typed, ok := plan.(*ResourceProjectionPlan)
	if !ok {
		return fmt.Errorf("bridges: bridge resource plan has type %T", plan)
	}
	if typed == nil {
		return errors.New("bridges: bridge resource plan is required")
	}
	if err := store.ReplaceBridgeInstances(ctx, typed.NextInstances()); err != nil {
		return fmt.Errorf("bridges: apply bridge resource state: replace instances: %w", err)
	}
	return nil
}

func bridgeInstancesByID(instances []BridgeInstance) map[string]BridgeInstance {
	byID := make(map[string]BridgeInstance, len(instances))
	for _, instance := range instances {
		byID[instance.ID] = *cloneBridgeInstance(instance)
	}
	return byID
}

func bridgeProjectionOperationCountByID(
	previousByID map[string]BridgeInstance,
	nextByID map[string]BridgeInstance,
) int {
	operations := 0
	for id, nextInstance := range nextByID {
		previousInstance, exists := previousByID[id]
		if !exists || !sameProjectedBridgeInstance(previousInstance, nextInstance) {
			operations++
		}
	}
	for id := range previousByID {
		if _, exists := nextByID[id]; !exists {
			operations++
		}
	}
	return operations
}

func changedBridgeProjectionExtensionsByID(
	previousByID map[string]BridgeInstance,
	nextByID map[string]BridgeInstance,
) []string {
	changed := make(map[string]struct{})
	for id, nextInstance := range nextByID {
		previousInstance, exists := previousByID[id]
		if exists && sameProjectedBridgeInstance(previousInstance, nextInstance) {
			continue
		}
		if previousInstance.ExtensionName != "" {
			changed[previousInstance.ExtensionName] = struct{}{}
		}
		if nextInstance.ExtensionName != "" {
			changed[nextInstance.ExtensionName] = struct{}{}
		}
	}
	for id, previousInstance := range previousByID {
		if _, exists := nextByID[id]; exists {
			continue
		}
		if previousInstance.ExtensionName != "" {
			changed[previousInstance.ExtensionName] = struct{}{}
		}
	}

	names := make([]string, 0, len(changed))
	for name := range changed {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

func sameProjectedBridgeInstance(left BridgeInstance, right BridgeInstance) bool {
	left = left.normalize()
	right = right.normalize()
	return left.ID == right.ID &&
		left.Scope == right.Scope &&
		left.WorkspaceID == right.WorkspaceID &&
		left.Platform == right.Platform &&
		left.ExtensionName == right.ExtensionName &&
		left.DisplayName == right.DisplayName &&
		left.Source == right.Source &&
		left.Enabled == right.Enabled &&
		left.Status == right.Status &&
		left.DMPolicy == right.DMPolicy &&
		left.RoutingPolicy == right.RoutingPolicy &&
		rawJSONEqual(left.ProviderConfig, right.ProviderConfig) &&
		rawJSONEqual(left.DeliveryDefaults, right.DeliveryDefaults) &&
		sameBridgeDegradation(left.Degradation, right.Degradation)
}

func sameBridgeDegradation(left *BridgeDegradation, right *BridgeDegradation) bool {
	left = cloneBridgeDegradationPointer(left)
	right = cloneBridgeDegradationPointer(right)
	switch {
	case left == nil && right == nil:
		return true
	case left == nil || right == nil:
		return false
	default:
		return *left == *right
	}
}

func rawJSONEqual(left []byte, right []byte) bool {
	return semanticJSONEqual(left, right)
}

func semanticJSONEqual(left []byte, right []byte) bool {
	left = bytes.TrimSpace(left)
	right = bytes.TrimSpace(right)
	if len(left) == 0 || bytes.Equal(left, []byte("null")) {
		left = nil
	}
	if len(right) == 0 || bytes.Equal(right, []byte("null")) {
		right = nil
	}
	switch {
	case len(left) == 0 && len(right) == 0:
		return true
	case len(left) == 0 || len(right) == 0:
		return false
	}
	if bytes.Equal(left, right) {
		return json.Valid(left)
	}

	leftValue, err := decodeSemanticJSON(left)
	if err != nil {
		return false
	}
	rightValue, err := decodeSemanticJSON(right)
	if err != nil {
		return false
	}
	return semanticJSONValuesEqual(leftValue, rightValue)
}

func decodeSemanticJSON(data []byte) (any, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	var extra json.RawMessage
	if err := decoder.Decode(&extra); errors.Is(err, io.EOF) {
		return value, nil
	} else if err != nil {
		return nil, err
	}
	return nil, errors.New("bridges: JSON payload contains multiple values")
}

func semanticJSONValuesEqual(left any, right any) bool {
	switch leftValue := left.(type) {
	case map[string]any:
		rightValue, ok := right.(map[string]any)
		if !ok || len(leftValue) != len(rightValue) {
			return false
		}
		for key, leftItem := range leftValue {
			rightItem, ok := rightValue[key]
			if !ok || !semanticJSONValuesEqual(leftItem, rightItem) {
				return false
			}
		}
		return true
	case []any:
		rightValue, ok := right.([]any)
		if !ok || len(leftValue) != len(rightValue) {
			return false
		}
		for idx, leftItem := range leftValue {
			if !semanticJSONValuesEqual(leftItem, rightValue[idx]) {
				return false
			}
		}
		return true
	case json.Number:
		rightValue, ok := right.(json.Number)
		return ok && semanticJSONNumbersEqual(leftValue, rightValue)
	default:
		return left == right
	}
}

func semanticJSONNumbersEqual(left json.Number, right json.Number) bool {
	leftValue, ok := new(big.Rat).SetString(left.String())
	if !ok {
		return false
	}
	rightValue, ok := new(big.Rat).SetString(right.String())
	if !ok {
		return false
	}
	return leftValue.Cmp(rightValue) == 0
}

func cloneBridgeInstances(instances []BridgeInstance) []BridgeInstance {
	if len(instances) == 0 {
		return nil
	}
	cloned := make([]BridgeInstance, 0, len(instances))
	for _, instance := range instances {
		cloned = append(cloned, *cloneBridgeInstance(instance))
	}
	return cloned
}

func sortBridgeInstances(instances []BridgeInstance) {
	slices.SortFunc(instances, func(left BridgeInstance, right BridgeInstance) int {
		if byDisplay := strings.Compare(left.DisplayName, right.DisplayName); byDisplay != 0 {
			return byDisplay
		}
		if byCreated := left.CreatedAt.Compare(right.CreatedAt); byCreated != 0 {
			return byCreated
		}
		return strings.Compare(left.ID, right.ID)
	})
}
