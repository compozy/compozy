package refs

import "strings"

func (n Namespace) validatePreviousPath(path []string) *Error {
	if len(path) < 2 {
		return newPathError(CodeUnknownReference, path, "previous reference must name a field")
	}
	switch path[1] {
	case "generation":
		return validateHistoryScalar(path, "previous.generation")
	case "nodes":
		return n.validatePreviousNodePath(path)
	case "verdicts":
		return validatePreviousVerdictPath(path)
	case "route_causes":
		return validateHistoryScalar(path, "previous.route_causes")
	default:
		return newPathError(CodeUnknownReference, path, "unknown previous field %q", path[1])
	}
}

func (n Namespace) validateBestPath(path []string) *Error {
	if len(path) < 2 {
		return newPathError(CodeUnknownReference, path, "best reference must name a field")
	}
	switch path[1] {
	case "generation", "score":
		return validateHistoryScalar(path, "best."+path[1])
	case "nodes":
		return n.validateBestNodePath(path)
	default:
		return newPathError(CodeUnknownReference, path, "unknown best field %q", path[1])
	}
}

func (n Namespace) validatePreviousNodePath(path []string) *Error {
	if len(path) < 4 {
		return newPathError(CodeUnknownReference, path, "previous node reference must name a node and field")
	}
	node, err := n.historyNode(path, "previous")
	if err != nil {
		return err
	}
	switch path[3] {
	case "status":
		return validateHistoryScalarTail(path[3:], "previous node status")
	case namespaceOutput:
		if !node.HasOutput {
			return nil
		}
		return validateSchemaPath(node.Output, path[4:], path)
	default:
		return newPathError(CodeUnknownReference, path, "unknown previous node field %q", path[3])
	}
}

func (n Namespace) validateBestNodePath(path []string) *Error {
	if len(path) < 4 {
		return newPathError(CodeUnknownReference, path, "best node reference must name a node and output")
	}
	node, err := n.historyNode(path, "best")
	if err != nil {
		return err
	}
	if path[3] != namespaceOutput {
		return newPathError(CodeUnknownReference, path, "best node field %q is not available", path[3])
	}
	if !node.HasOutput {
		return nil
	}
	return validateSchemaPath(node.Output, path[4:], path)
}

func (n Namespace) historyNode(path []string, root string) (NodeSchema, *Error) {
	node, ok := n.Nodes[path[2]]
	if !ok {
		return NodeSchema{}, newPathError(CodeUnknownReference, path, "unknown %s node %q", root, path[2])
	}
	return node, nil
}

func validatePreviousVerdictPath(path []string) *Error {
	if len(path) < 4 || strings.TrimSpace(path[2]) == "" {
		return newPathError(
			CodeUnknownReference,
			path,
			"previous verdict reference must name a gate and field",
		)
	}
	switch path[3] {
	case "outcome", "score":
		return validateHistoryScalarTail(path[3:], "previous verdict "+path[3])
	case "blocking_issues", "criteria":
		return nil
	default:
		return newPathError(CodeUnknownReference, path, "unknown previous verdict field %q", path[3])
	}
}

func validateHistoryScalar(path []string, name string) *Error {
	if len(path) > 2 {
		return newPathError(CodeUnresolvablePath, path, "%s is a scalar", name)
	}
	return nil
}

func validateHistoryScalarTail(path []string, name string) *Error {
	if len(path) > 1 {
		return newPathError(CodeUnresolvablePath, path, "%s is a scalar", name)
	}
	return nil
}
