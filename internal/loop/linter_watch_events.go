package loop

import (
	"fmt"
	"slices"
	"strings"

	"github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/loop/dsl"
	"github.com/compozy/agh/internal/loop/dsl/refs"
)

const (
	watchEventsOutputEventsKey  = "events"
	watchEventsOutputCursorsKey = "cursors"

	watchEventsFieldSeq         = "seq"
	watchEventsFieldStream      = "stream"
	watchEventsFieldAt          = "at"
	watchEventsFieldWorkspaceID = "workspace_id"
	watchEventsFieldTaskID      = "task_id"
	watchEventsFieldRunID       = "run_id"
	watchEventsFieldLoopRunID   = "loop_run_id"
	watchEventsFieldLoopName    = "loop_name"
	watchEventsFieldSessionID   = "session_id"
	watchEventsFieldChannel     = "channel"
	watchEventsFieldWorkID      = "work_id"
	watchEventsFieldPayload     = "payload"

	watchEventsSchemaArrayType = "array"
	watchEventsSchemaMapType   = "map"
)

func (c *lintContext) lintWatchEventsEnvelopeShape(node dsl.Node) {
	isWatchEvents := node.Class == dsl.NodeClassSource &&
		dsl.SourceKind(node.Kind) == dsl.SourceWatchEvents
	if len(node.Events) > 0 && !isWatchEvents {
		c.add(
			node.ID,
			CodeWatchEventsShapeInvalid,
			"events block is only valid on source kind %q",
			dsl.SourceWatchEvents,
		)
	}
	if isWatchEvents && len(node.WatchSpec) > 0 {
		c.add(
			node.ID,
			CodeWatchEventsShapeInvalid,
			"watch-events nodes use events, not watch",
		)
	}
}

func (c *lintContext) lintWatchEventsNode(node dsl.Node) {
	if len(node.Events) == 0 {
		c.add(
			node.ID,
			CodeWatchEventsSubscriptionRequired,
			"watch-events must declare at least one events subscription",
		)
	}
	c.lintWatchEventsProduces(node)
	supported := SupportedWatchEvents()
	for idx, subscription := range node.Events {
		kind := hooks.HookEvent(strings.TrimSpace(subscription.Kind))
		if strings.TrimSpace(string(kind)) == "" || kind.Validate() != nil {
			c.add(
				node.ID,
				CodeWatchEventsKindUnknown,
				"watch-events events[%d].kind %q is not in the hook catalog",
				idx,
				subscription.Kind,
			)
			continue
		}
		if _, ok := supported[kind]; !ok {
			c.add(
				node.ID,
				CodeWatchEventsKindUnsupported,
				"watch-events kind %q is not supported in this phase; supported kinds: %s",
				kind,
				strings.Join(sortedSupportedWatchEventKinds(), ", "),
			)
		}
	}
}

func (c *lintContext) lintWatchEventsProduces(node dsl.Node) {
	if len(node.Produces) == 0 {
		return
	}
	for key, value := range node.Produces {
		switch key {
		case watchEventsOutputEventsKey:
			if !schemaDeclaresArray(value) {
				c.add(
					node.ID,
					CodeWatchEventsShapeInvalid,
					"watch-events produces.events must describe an array of matched events",
				)
			}
		case watchEventsOutputCursorsKey:
			if !schemaDeclaresObject(value) {
				c.add(
					node.ID,
					CodeWatchEventsShapeInvalid,
					"watch-events produces.cursors must describe an object of stream cursors",
				)
			}
		default:
			c.add(
				node.ID,
				CodeWatchEventsShapeInvalid,
				"watch-events produces may only declare events and cursors",
			)
		}
	}
}

func (c *lintContext) lintWatchEventsFilterReferences(node dsl.Node, namespace refs.Namespace) {
	if node.Class != dsl.NodeClassSource || dsl.SourceKind(node.Kind) != dsl.SourceWatchEvents {
		return
	}
	eventNamespace := namespaceWithEvent(namespace)
	supported := SupportedWatchEvents()
	for idx, subscription := range node.Events {
		kind := hooks.HookEvent(strings.TrimSpace(subscription.Kind))
		contract, ok := supported[kind]
		if !ok {
			continue
		}
		if strings.TrimSpace(subscription.Filter) == "" {
			c.lintWatchEventsRequiredVars(node, idx, subscription, contract)
			continue
		}
		condition, err := c.compileCondition(subscription.Filter, eventNamespace)
		if err != nil {
			c.add(
				node.ID,
				CodeWatchEventsFilterInvalid,
				"watch-events events[%d].filter for kind %q is invalid: %v",
				idx,
				subscription.Kind,
				err,
			)
			continue
		}
		if condition != nil {
			c.warnUnknownOutputReferences(node.ID, condition.References, eventNamespace)
		}
		c.lintWatchEventsRequiredVars(node, idx, subscription, contract)
	}
}

func (c *lintContext) lintWatchEventsRequiredVars(
	node dsl.Node,
	idx int,
	subscription dsl.EventSubscription,
	contract WatchEventsContract,
) {
	if !slices.Contains(contract.RequiredVars, watchEventsFieldSessionID) {
		return
	}
	if watchEventsFilterHasSessionConstraint(subscription.Filter) {
		return
	}
	c.add(
		node.ID,
		CodeWatchEventsFilterTooBroad,
		"watch-events events[%d].filter for kind %q must constrain event.session_id with equality",
		idx,
		subscription.Kind,
	)
}

func watchEventsOutputSchema() refs.Schema {
	return refs.Schema{
		watchEventsOutputEventsKey: []any{map[string]any{
			actionKindMetaKey:           jsonSchemaStringType,
			watchEventsFieldSeq:         "integer",
			watchEventsFieldStream:      jsonSchemaStringType,
			watchEventsFieldAt:          jsonSchemaStringType,
			watchEventsFieldWorkspaceID: jsonSchemaStringType,
			watchEventsFieldTaskID:      jsonSchemaStringType,
			watchEventsFieldRunID:       jsonSchemaStringType,
			watchEventsFieldLoopRunID:   jsonSchemaStringType,
			watchEventsFieldLoopName:    jsonSchemaStringType,
			watchEventsFieldSessionID:   jsonSchemaStringType,
			watchEventsFieldChannel:     jsonSchemaStringType,
			watchEventsFieldWorkID:      jsonSchemaStringType,
			watchEventsFieldPayload:     map[string]any{},
		}},
		watchEventsOutputCursorsKey: map[string]any{},
	}
}

func schemaDeclaresArray(value any) bool {
	normalized := normalizeSchemaValue(value)
	switch typed := normalized.(type) {
	case []any:
		return true
	case string:
		return typed == watchEventsSchemaArrayType
	case map[string]any:
		return schemaTypeIs(typed, watchEventsSchemaArrayType)
	default:
		return false
	}
}

func schemaDeclaresObject(value any) bool {
	normalized := normalizeSchemaValue(value)
	switch typed := normalized.(type) {
	case map[string]any:
		return len(typed) == 0 || schemaTypeIs(typed, jsonSchemaObjectType)
	case string:
		return typed == jsonSchemaObjectType || typed == watchEventsSchemaMapType
	default:
		return false
	}
}

func schemaTypeIs(schema map[string]any, want string) bool {
	raw, ok := schema[jsonSchemaTypeKey]
	if !ok {
		return false
	}
	switch typed := raw.(type) {
	case string:
		return typed == want
	case []any:
		return slices.ContainsFunc(typed, func(item any) bool {
			return fmt.Sprint(item) == want
		})
	default:
		return false
	}
}
