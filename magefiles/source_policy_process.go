//go:build mage

package main

import (
	"go/ast"
	"go/token"
)

const windowsImportPath = "golang.org/x/sys/windows"

func inspectProcessControl(
	fileset *token.FileSet,
	file *ast.File,
	imports map[string]string,
	rel string,
) []sourcePolicyViolation {
	violations := make([]sourcePolicyViolation, 0)
	processHandles := osProcessHandleNames(file, imports)
	commandHandles := execCommandHandleNames(file, imports)
	ast.Inspect(file, func(node ast.Node) bool {
		switch typed := node.(type) {
		case *ast.CallExpr:
			message := rawProcessCallMessage(typed, imports)
			if message == "" && callsOSProcessControl(typed, processHandles, commandHandles) {
				message = "os.Process.Signal/Kill may only be called by internal/procutil"
			}
			if message != "" {
				violations = append(violations, sourcePolicyViolation{
					path:    rel,
					line:    fileset.Position(typed.Pos()).Line,
					message: message,
				})
			}
		case *ast.AssignStmt:
			for _, target := range typed.Lhs {
				if selectsTrackedField(target, commandHandles, "SysProcAttr") {
					violations = append(violations, sourcePolicyViolation{
						path:    rel,
						line:    fileset.Position(target.Pos()).Line,
						message: "exec.Cmd.SysProcAttr may only be mutated by internal/procutil",
					})
				}
			}
		case *ast.CompositeLit:
			if isImportedType(typed.Type, imports, "os/exec", "Cmd") && compositeHasField(typed, "SysProcAttr") {
				violations = append(violations, sourcePolicyViolation{
					path:    rel,
					line:    fileset.Position(typed.Pos()).Line,
					message: "exec.Cmd.SysProcAttr may only be configured by internal/procutil",
				})
			}
		}
		return true
	})
	return violations
}

func osProcessHandleNames(file *ast.File, imports map[string]string) map[string]struct{} {
	names := make(map[string]struct{})
	ast.Inspect(file, func(node ast.Node) bool {
		switch typed := node.(type) {
		case *ast.Field:
			if isImportedPointerType(typed.Type, imports, "os", "Process") {
				for _, name := range typed.Names {
					names[name.Name] = struct{}{}
				}
			}
		case *ast.ValueSpec:
			if isImportedPointerType(typed.Type, imports, "os", "Process") {
				for _, name := range typed.Names {
					names[name.Name] = struct{}{}
				}
			}
			for index, value := range typed.Values {
				if callReturnsOSProcess(value, imports) && index < len(typed.Names) {
					names[typed.Names[index].Name] = struct{}{}
				}
			}
		case *ast.AssignStmt:
			for index, value := range typed.Rhs {
				if !callReturnsOSProcess(value, imports) || index >= len(typed.Lhs) {
					continue
				}
				if name, ok := typed.Lhs[index].(*ast.Ident); ok {
					names[name.Name] = struct{}{}
				}
			}
		}
		return true
	})
	return names
}

func execCommandHandleNames(file *ast.File, imports map[string]string) map[string]struct{} {
	names := make(map[string]struct{})
	ast.Inspect(file, func(node ast.Node) bool {
		switch typed := node.(type) {
		case *ast.Field:
			if isExecCommandType(typed.Type, imports) {
				for _, name := range typed.Names {
					names[name.Name] = struct{}{}
				}
			}
		case *ast.ValueSpec:
			if isExecCommandType(typed.Type, imports) {
				for _, name := range typed.Names {
					names[name.Name] = struct{}{}
				}
			}
			for index, value := range typed.Values {
				if callReturnsExecCommand(value, imports) && index < len(typed.Names) {
					names[typed.Names[index].Name] = struct{}{}
				}
			}
		case *ast.AssignStmt:
			for index, value := range typed.Rhs {
				if !callReturnsExecCommand(value, imports) || index >= len(typed.Lhs) {
					continue
				}
				if name, ok := typed.Lhs[index].(*ast.Ident); ok {
					names[name.Name] = struct{}{}
				}
			}
		}
		return true
	})
	return names
}

func isExecCommandType(expr ast.Expr, imports map[string]string) bool {
	return isImportedType(expr, imports, "os/exec", "Cmd") ||
		isImportedPointerType(expr, imports, "os/exec", "Cmd")
}

func callReturnsExecCommand(expr ast.Expr, imports map[string]string) bool {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return false
	}
	return importedCall(call, imports, "os/exec", "Command") ||
		importedCall(call, imports, "os/exec", "CommandContext")
}

func callsOSProcessControl(
	call *ast.CallExpr,
	processHandles map[string]struct{},
	commandHandles map[string]struct{},
) bool {
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || (selector.Sel.Name != "Signal" && selector.Sel.Name != "Kill") {
		return false
	}
	if receiver, ok := selector.X.(*ast.Ident); ok {
		_, exists := processHandles[receiver.Name]
		return exists
	}
	receiver, ok := selector.X.(*ast.SelectorExpr)
	if !ok || receiver.Sel.Name != "Process" {
		return false
	}
	command, ok := receiver.X.(*ast.Ident)
	if !ok {
		return false
	}
	_, exists := commandHandles[command.Name]
	return exists
}

func callReturnsOSProcess(expr ast.Expr, imports map[string]string) bool {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return false
	}
	return importedCall(call, imports, "os", "FindProcess") ||
		importedCall(call, imports, "os", "StartProcess")
}

func rawProcessCallMessage(call *ast.CallExpr, imports map[string]string) string {
	switch {
	case importedCall(call, imports, "syscall", "Kill"):
		return "syscall.Kill may only be called by internal/procutil"
	case importedCall(call, imports, "syscall", "OpenProcess"):
		return "syscall.OpenProcess may only be called by internal/procutil"
	case importedCall(call, imports, "syscall", "TerminateProcess"):
		return "syscall.TerminateProcess may only be called by internal/procutil"
	case importedCall(call, imports, windowsImportPath, "OpenProcess"):
		return "windows.OpenProcess may only be called by internal/procutil"
	case importedCall(call, imports, windowsImportPath, "TerminateProcess"):
		return "windows.TerminateProcess may only be called by internal/procutil"
	case importedCall(call, imports, "os", "FindProcess"):
		return "os.FindProcess may only be called by internal/procutil"
	case importedCall(call, imports, "os", "StartProcess"):
		return "os.StartProcess may only be called by internal/procutil"
	default:
		return ""
	}
}

func selectsTrackedField(expr ast.Expr, handles map[string]struct{}, field string) bool {
	selector, ok := expr.(*ast.SelectorExpr)
	for ok {
		if selector.Sel.Name == field {
			receiver, isIdentifier := selector.X.(*ast.Ident)
			if !isIdentifier {
				return false
			}
			_, exists := handles[receiver.Name]
			return exists
		}
		selector, ok = selector.X.(*ast.SelectorExpr)
	}
	return false
}

func compositeHasField(literal *ast.CompositeLit, name string) bool {
	for _, element := range literal.Elts {
		pair, ok := element.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		ident, ok := pair.Key.(*ast.Ident)
		if ok && ident.Name == name {
			return true
		}
	}
	return false
}
