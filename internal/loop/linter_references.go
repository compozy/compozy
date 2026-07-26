package loop

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/loop/dsl"
	"github.com/compozy/agh/internal/loop/dsl/refs"
)

func (c *lintContext) lintContractReferences(namespace refs.Namespace) {
	if strings.TrimSpace(c.def.Contract.StopWhen) != "" {
		condition, err := c.compileCondition(c.def.Contract.StopWhen, namespace)
		if err != nil {
			c.addRefsError("", err)
		} else if condition != nil {
			c.warnUnknownOutputReferences("", condition.References, namespace)
		}
	}
	for idx, criterion := range c.def.Contract.Verification {
		prefix := fmt.Sprintf("verification[%d]", idx)
		for _, item := range criterionStringFields(prefix, criterion) {
			template, err := c.compileTemplate(item.name, item.value, namespace)
			if err != nil {
				c.addRefsError("", err)
				continue
			}
			if template != nil {
				c.warnUnknownOutputReferences("", template.References, namespace)
			}
		}
	}
}

func (c *lintContext) lintStartReferences() {
	for _, start := range c.def.Start {
		namespace := c.namespace(false, startAllowsTrigger(start.Kind))
		for input, raw := range start.InputMapping {
			if _, ok := c.def.Inputs[input]; !ok {
				c.add(
					"",
					refs.CodeUnknownReference,
					"start input_mapping key %q does not name a declared input",
					input,
				)
			}
			if _, err := c.compileTemplate("start.input_mapping."+input, raw, namespace); err != nil {
				c.addRefsError("", err)
			}
		}
	}
}

func startAllowsTrigger(kind dsl.StartKind) bool {
	switch kind {
	case dsl.StartTrigger, dsl.StartSchedule, dsl.StartWebhook:
		return true
	default:
		return false
	}
}

func (c *lintContext) lintNodeReferences(node dsl.Node, namespace refs.Namespace) {
	c.lintTransformReferences(node, namespace)
	for _, item := range nodeStringFields(node) {
		template, err := c.compileTemplate(item.name, item.value, namespace)
		if err != nil {
			c.addRefsError(node.ID, err)
			continue
		}
		if template != nil {
			c.warnUnknownOutputReferences(node.ID, template.References, namespace)
		}
		if isFanOutCollection(node, item.name) && template != nil && len(template.References) == 0 {
			c.add(
				node.ID,
				CodeFanOutUnbounded,
				"fan-out collection must reference a materialized namespace value",
			)
		}
	}
	for _, item := range nodeConditionFields(node) {
		condition, err := c.compileCondition(item.value, namespace)
		if err != nil {
			c.addRefsError(node.ID, err)
			continue
		}
		if condition != nil {
			c.warnUnknownOutputReferences(node.ID, condition.References, namespace)
		}
	}
	c.lintWatchEventsFilterReferences(node, namespace)
}

func (c *lintContext) lintTransformReferences(node dsl.Node, namespace refs.Namespace) {
	if node.Class != dsl.NodeClassAction || dsl.ActionKind(node.Kind) != dsl.ActionTransform {
		return
	}
	var params dsl.TransformParams
	if err := node.Params.Decode(&params); err != nil {
		return
	}
	for key, mapping := range params.Map {
		from := strings.TrimSpace(mapping.From)
		if from == "" {
			continue
		}
		path := strings.Split(from, ".")
		if err := namespace.ValidatePath(path); err != nil {
			c.addRefsError(node.ID, fmt.Errorf("params.map.%s.from: %w", key, err))
			continue
		}
		c.warnUnknownOutputReferences(
			node.ID,
			[]refs.Reference{{Raw: from, Path: append([]string(nil), path...)}},
			namespace,
		)
	}
}

func isFanOutCollection(node dsl.Node, name string) bool {
	return node.Class == dsl.NodeClassControl && dsl.ControlKind(node.Kind) == dsl.ControlFanOut &&
		name == "collection"
}

type namedString struct {
	name  string
	value string
}

func nodeStringFields(node dsl.Node) []namedString {
	fields := []namedString{
		{name: "collection", value: node.Collection},
		{name: "pattern", value: node.Pattern},
	}
	fields = append(fields, nodeParamStringFields(node)...)
	for idx, criterion := range node.Criteria {
		fields = append(
			fields,
			criterionStringFields(fmt.Sprintf("criteria[%d]", idx), criterion)...)
	}
	if node.Harvest != nil {
		fields = append(fields,
			namedString{name: "harvest.responder", value: node.Harvest.Responder},
			namedString{name: "harvest.content_rule", value: node.Harvest.ContentRule},
		)
	}
	return fields
}

func nodeParamStringFields(node dsl.Node) []namedString {
	if node.Class == dsl.NodeClassAction && dsl.ActionKind(node.Kind) == dsl.ActionGoal {
		return goalParamStringFields(node)
	}
	if node.Class == dsl.NodeClassAction && dsl.ActionKind(node.Kind) == dsl.ActionRunAgent {
		return paramsStringFieldsWithSkip(
			"params",
			map[string]any(node.Params),
			func(prefix string, key string) bool {
				return prefix == "params" && key == outputSchemaParamKey
			},
		)
	}
	if node.Class == dsl.NodeClassAction && dsl.ActionKind(node.Kind) == dsl.ActionTransform {
		return paramsStringFieldsWithSkip(
			"params",
			map[string]any(node.Params),
			func(prefix string, key string) bool {
				return strings.HasPrefix(prefix, "params.map.") && key == transformValueKey
			},
		)
	}
	return paramsStringFields("params", map[string]any(node.Params))
}

func criterionStringFields(prefix string, criterion dsl.GateCriterion) []namedString {
	fields := []namedString{
		{name: prefix + ".check", value: criterion.Check},
		{name: prefix + ".agent", value: criterion.Agent},
		{name: prefix + ".rubric", value: criterion.Rubric},
		{name: prefix + ".prompt", value: criterion.Prompt},
		{name: prefix + ".tool", value: criterion.Tool},
	}
	return append(fields, paramsStringFields(prefix+".inputs", criterion.Inputs)...)
}

func paramsStringFields(prefix string, values map[string]any) []namedString {
	return paramsStringFieldsWithSkip(prefix, values, nil)
}

func paramsStringFieldsWithSkip(
	prefix string,
	values map[string]any,
	skip func(prefix string, key string) bool,
) []namedString {
	fields := []namedString{}
	for key, value := range values {
		if skip != nil && skip(prefix, key) {
			continue
		}
		name := prefix + "." + key
		switch typed := value.(type) {
		case string:
			fields = append(fields, namedString{name: name, value: typed})
		case map[string]any:
			fields = append(fields, paramsStringFieldsWithSkip(name, typed, skip)...)
		case map[any]any:
			converted := map[string]any{}
			for rawKey, rawValue := range typed {
				converted[fmt.Sprint(rawKey)] = rawValue
			}
			fields = append(fields, paramsStringFieldsWithSkip(name, converted, skip)...)
		case []any:
			for idx, item := range typed {
				if nested, ok := item.(map[string]any); ok {
					fields = append(
						fields,
						paramsStringFieldsWithSkip(fmt.Sprintf("%s[%d]", name, idx), nested, skip)...,
					)
				}
			}
		}
	}
	return fields
}

func nodeConditionFields(node dsl.Node) []namedString {
	return []namedString{
		{name: "condition", value: node.Condition},
		{name: "filter", value: node.Filter},
	}
}

func (c *lintContext) compileTemplate(
	name string,
	raw string,
	namespace refs.Namespace,
) (*refs.Template, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	template, err := refs.CompileTemplate(name, raw, namespace)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	return template, nil
}

func (c *lintContext) compileCondition(
	raw string,
	namespace refs.Namespace,
) (*refs.Condition, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	compiler, err := c.conditionCompiler(namespace)
	if err != nil {
		return nil, err
	}
	condition, err := compiler.Compile(raw)
	if err != nil {
		return nil, err
	}
	return condition, nil
}

func (c *lintContext) conditionCompiler(namespace refs.Namespace) (*refs.ConditionCompiler, error) {
	key := conditionCompilerKey{
		allowFanout:  namespace.AllowFanout,
		allowTrigger: namespace.AllowTrigger,
		allowEvent:   namespace.AllowEvent,
	}
	if compiler, ok := c.conditionCompilers[key]; ok {
		return compiler, nil
	}
	compiler, err := refs.NewConditionCompiler(namespace)
	if err != nil {
		return nil, err
	}
	c.conditionCompilers[key] = compiler
	return compiler, nil
}

func (c *lintContext) namespace(allowFanout bool, allowTrigger bool) refs.Namespace {
	inputs := map[string]refs.Schema{}
	for name, input := range c.def.Inputs {
		inputs[name] = inputSchema(input)
	}
	nodes := map[string]refs.NodeSchema{}
	for _, node := range c.def.Graph.Nodes {
		schema, ok := c.outputSchema(node)
		nodes[string(node.ID)] = refs.NodeSchema{Output: schema, HasOutput: ok}
	}
	return refs.Namespace{
		Inputs:       inputs,
		Nodes:        nodes,
		AllowFanout:  allowFanout,
		AllowTrigger: allowTrigger,
	}
}

func namespaceWithEvent(namespace refs.Namespace) refs.Namespace {
	namespace.AllowEvent = true
	return namespace
}

func inputSchema(input dsl.Input) refs.Schema {
	return refs.Schema{jsonSchemaTypeKey: string(input.Type)}
}

func (c *lintContext) outputSchema(node dsl.Node) (refs.Schema, bool) {
	if len(node.Produces) > 0 {
		return convertSchema(node.Produces), true
	}
	switch node.Class {
	case dsl.NodeClassSource:
		return c.sourceOutputSchema(node)
	case dsl.NodeClassAction:
		return c.actionOutputSchema(node)
	default:
		return nil, false
	}
}

func (c *lintContext) sourceOutputSchema(node dsl.Node) (refs.Schema, bool) {
	switch dsl.SourceKind(node.Kind) {
	case dsl.SourceInput:
		input, ok := c.def.Inputs[node.InputRef]
		if !ok {
			return nil, false
		}
		return inputSchema(input), true
	case dsl.SourceWatchEvents:
		return watchEventsOutputSchema(), true
	default:
		return nil, false
	}
}

func (c *lintContext) actionOutputSchema(node dsl.Node) (refs.Schema, bool) {
	if !dsl.IsReservedActionKind(node.Kind) {
		return c.toolOutputSchema(node.Kind)
	}
	switch dsl.ActionKind(node.Kind) {
	case dsl.ActionGoal:
		var params dsl.GoalParams
		if err := node.Params.Decode(&params); err != nil || params.OutputSchema == nil {
			return nil, false
		}
		return convertSchema(*params.OutputSchema), true
	case dsl.ActionRunAgent:
		var params dsl.RunAgentParams
		if err := node.Params.Decode(&params); err != nil || len(params.OutputSchema) == 0 {
			return nil, false
		}
		return convertSchema(params.OutputSchema), true
	case dsl.ActionRunLoop:
		return refs.Schema{reasonMetaStatus: jsonSchemaStringType, "outputs": map[string]any{}}, true
	case dsl.ActionTransform:
		var params dsl.TransformParams
		if err := node.Params.Decode(&params); err != nil || len(params.Map) == 0 {
			return nil, false
		}
		schema := refs.Schema{}
		for key := range params.Map {
			schema[key] = "any"
		}
		return schema, true
	default:
		return nil, false
	}
}

func (c *lintContext) toolOutputSchema(kind string) (refs.Schema, bool) {
	if c.linter.tools == nil {
		return nil, false
	}
	snapshot, ok := c.linter.tools.Snapshot(kind)
	if !ok || len(snapshot.OutputSchema) == 0 {
		return nil, false
	}
	schema, err := schemaFromJSON(snapshot.OutputSchema)
	if err != nil {
		return nil, false
	}
	return schema, true
}

func convertSchema(schema dsl.Schema) refs.Schema {
	out := refs.Schema{}
	for key, value := range schema {
		out[key] = normalizeSchemaValue(value)
	}
	return out
}

func normalizeSchemaValue(value any) any {
	switch typed := value.(type) {
	case dsl.Schema:
		normalized := map[string]any{}
		for key, child := range typed {
			normalized[key] = normalizeSchemaValue(child)
		}
		return normalized
	case map[string]any:
		normalized := map[string]any{}
		for key, child := range typed {
			normalized[key] = normalizeSchemaValue(child)
		}
		return normalized
	case map[any]any:
		normalized := map[string]any{}
		for key, child := range typed {
			normalized[fmt.Sprint(key)] = normalizeSchemaValue(child)
		}
		return normalized
	case []any:
		normalized := make([]any, 0, len(typed))
		for _, child := range typed {
			normalized = append(normalized, normalizeSchemaValue(child))
		}
		return normalized
	default:
		return typed
	}
}

func schemaFromJSON(raw json.RawMessage) (refs.Schema, error) {
	decoded := map[string]any{}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return nil, fmt.Errorf("decode JSON schema: %w", err)
	}
	return refs.Schema(decoded), nil
}

func (c *lintContext) inFanoutScope(id dsl.NodeID) bool {
	seen := map[dsl.NodeID]struct{}{}
	var visit func(dsl.NodeID) bool
	visit = func(current dsl.NodeID) bool {
		if _, ok := seen[current]; ok {
			return false
		}
		seen[current] = struct{}{}
		if node, ok := c.nodeByID[current]; ok && isCollectNode(node) {
			return false
		}
		for _, previous := range c.reverse[current] {
			node, ok := c.nodeByID[previous]
			if ok && node.Class == dsl.NodeClassControl &&
				dsl.ControlKind(node.Kind) == dsl.ControlFanOut {
				return true
			}
			if visit(previous) {
				return true
			}
		}
		return false
	}
	return visit(id)
}

func isCollectNode(node dsl.Node) bool {
	return node.Class == dsl.NodeClassControl && dsl.ControlKind(node.Kind) == dsl.ControlCollect
}

func (c *lintContext) warnUnknownOutputReferences(
	nodeID dsl.NodeID,
	references []refs.Reference,
	namespace refs.Namespace,
) {
	for _, reference := range references {
		if len(reference.Path) < 3 || reference.Path[0] != namespaceNodesKey ||
			reference.Path[2] != "output" {
			continue
		}
		node, ok := namespace.Nodes[reference.Path[1]]
		if !ok || node.HasOutput {
			continue
		}
		c.warn(
			nodeID,
			refs.CodeUnresolvablePath,
			"node %q has no declared output schema; path %q cannot be proven",
			reference.Path[1],
			reference.Raw,
		)
	}
}
