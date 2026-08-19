package loop

import (
	"fmt"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/loop/dsl/refs"
)

func (c *lintContext) lintContractReferences(namespace refs.Namespace) {
	narrativeNamespace := contractNarrativeNamespace(namespace)
	for _, item := range contractNarrativeStringFields(c.def.Contract) {
		template, err := c.compileStringField(item, narrativeNamespace)
		if err != nil {
			c.addRefsError("", err)
			continue
		}
		if template != nil {
			c.warnUnknownOutputReferences("", template.References, narrativeNamespace)
		}
	}
	if strings.TrimSpace(c.def.Contract.StopWhen.Expr) != "" {
		condition, err := c.compileCondition(c.def.Contract.StopWhen.Expr, namespace)
		if err != nil {
			c.addRefsError("", err)
		} else if condition != nil {
			c.warnUnknownOutputReferences("", condition.References, namespace)
		}
	}
	for idx, criterion := range c.def.Contract.Verification {
		prefix := fmt.Sprintf("verification[%d]", idx)
		for _, item := range criterionStringFields(prefix, criterion) {
			template, err := c.compileStringField(item, namespace)
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
		template, err := c.compileStringField(item, namespace)
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
	name         string
	value        string
	shellCommand bool
}

func nodeStringFields(node dsl.Node) []namedString {
	fields := []namedString{
		{name: "collection", value: node.Collection},
		{name: "pattern", value: node.Pattern},
	}
	if node.Review != nil {
		fields = append(fields, namedString{name: "review.prompt", value: node.Review.Prompt})
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
	if node.Class == dsl.NodeClassControl && dsl.ControlKind(node.Kind) == dsl.ControlWait {
		var params dsl.WaitParams
		if err := node.Params.Decode(&params); err != nil || strings.TrimSpace(params.Until) == "" {
			return nil
		}
		return []namedString{{name: "params.until", value: params.Until}}
	}
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
		{
			name: prefix + ".check", value: criterion.Check,
			shellCommand: criterion.Type == dsl.CriterionCommand,
		},
		{name: prefix + ".contains", value: criterion.Contains},
		{name: prefix + ".agent", value: criterion.Agent},
		{name: prefix + ".rubric", value: criterion.Rubric},
		{name: prefix + ".prompt", value: criterion.Prompt},
		{name: prefix + ".tool", value: criterion.Tool},
	}
	return append(fields, paramsStringFields(prefix+".inputs", criterion.Inputs)...)
}

func compileNamedStringTemplate(item namedString, namespace refs.Namespace) (*refs.Template, error) {
	if item.shellCommand {
		return refs.CompileCommandTemplate(item.name, item.value, namespace)
	}
	return refs.CompileTemplate(item.name, item.value, namespace)
}

func (c *lintContext) compileStringField(
	item namedString,
	namespace refs.Namespace,
) (*refs.Template, error) {
	if strings.TrimSpace(item.value) == "" {
		return nil, nil
	}
	template, err := compileNamedStringTemplate(item, namespace)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", item.name, err)
	}
	return template, nil
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
		fields = append(fields, paramValueStringFields(name, value, skip)...)
	}
	return fields
}

func paramValueStringFields(
	name string,
	value any,
	skip func(prefix string, key string) bool,
) []namedString {
	switch typed := value.(type) {
	case string:
		return []namedString{{name: name, value: typed}}
	case dsl.NodeParams:
		return paramsStringFieldsWithSkip(name, map[string]any(typed), skip)
	case dsl.Schema:
		return paramsStringFieldsWithSkip(name, map[string]any(typed), skip)
	case map[string]any:
		return paramsStringFieldsWithSkip(name, typed, skip)
	case map[any]any:
		converted := make(map[string]any, len(typed))
		for rawKey, rawValue := range typed {
			converted[fmt.Sprint(rawKey)] = rawValue
		}
		return paramsStringFieldsWithSkip(name, converted, skip)
	case []any:
		return paramListStringFields(name, typed, skip)
	case []dsl.Schema:
		values := make([]any, len(typed))
		for idx := range typed {
			values[idx] = typed[idx]
		}
		return paramListStringFields(name, values, skip)
	case []refs.Schema:
		values := make([]any, len(typed))
		for idx := range typed {
			values[idx] = typed[idx]
		}
		return paramListStringFields(name, values, skip)
	default:
		return nil
	}
}

func paramListStringFields(
	name string,
	values []any,
	skip func(prefix string, key string) bool,
) []namedString {
	fields := []namedString{}
	for idx, value := range values {
		fields = append(
			fields,
			paramValueStringFields(fmt.Sprintf("%s[%d]", name, idx), value, skip)...,
		)
	}
	return fields
}

func nodeConditionFields(node dsl.Node) []namedString {
	fields := []namedString{
		{name: "condition", value: node.Condition},
		{name: "filter", value: node.Filter},
	}
	if node.Review != nil {
		fields = append(fields, namedString{name: "review.when", value: node.Review.When})
	}
	for index, route := range node.Routes {
		fields = append(fields, namedString{
			name:  fmt.Sprintf("routes.%d.when", index),
			value: route.When,
		})
	}
	return fields
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
	itemNames := make([]string, 0, len(namespace.FanoutItems)+len(namespace.FanoutIndexes))
	for name := range namespace.FanoutItems {
		itemNames = append(itemNames, "item:"+name)
	}
	for name := range namespace.FanoutIndexes {
		itemNames = append(itemNames, "index:"+name)
	}
	slices.Sort(itemNames)
	key := conditionCompilerKey{
		allowFanout:    namespace.AllowFanout,
		allowTrigger:   namespace.AllowTrigger,
		allowEvent:     namespace.AllowEvent,
		allowProgress:  namespace.AllowProgress,
		iterationNames: strings.Join(itemNames, ","),
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
		nodes[string(node.ID)] = refs.NodeSchema{
			Output: schema, HasOutput: ok,
			HasProgress: isControlKind(node, dsl.ControlFanOut),
		}
	}
	return refs.Namespace{
		Inputs:       inputs,
		Nodes:        nodes,
		AllowFanout:  allowFanout,
		AllowTrigger: allowTrigger,
	}
}

func (c *lintContext) namespaceForNode(nodeID dsl.NodeID, allowTrigger bool) refs.Namespace {
	ancestors := c.fanOutAncestors(nodeID)
	namespace := c.namespace(len(ancestors) > 0, allowTrigger)
	namespace.AllowProgress = len(ancestors) > 0
	namespace.FanoutItems = map[string]struct{}{}
	namespace.FanoutIndexes = map[string]struct{}{}
	for _, fanOut := range ancestors {
		if name := strings.TrimSpace(fanOut.BindAs); name != "" {
			namespace.FanoutItems[name] = struct{}{}
		}
		if name := strings.TrimSpace(fanOut.IndexAs); name != "" {
			namespace.FanoutIndexes[name] = struct{}{}
		}
	}
	return namespace
}

func namespaceWithEvent(namespace refs.Namespace) refs.Namespace {
	namespace.AllowEvent = true
	return namespace
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
			reference.Path[2] != namespaceOutputKey {
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
