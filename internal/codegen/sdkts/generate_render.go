package sdkts

import (
	"strconv"
	"strings"

	"unicode/utf8"

	extensioncontract "github.com/compozy/agh/internal/extension/contract"
)

func estimatedOutputSize(blocks []string) int {
	size := len(generatedHeader) + len(generatedImport)
	for idx, block := range blocks {
		size += len(block)
		if !strings.HasSuffix(block, "\n") {
			size++
		}
		if idx < len(blocks)-1 {
			size++
		}
	}
	return size
}

func renderTypeAlias(name string, value string) string {
	var out strings.Builder
	out.Grow(len("export type  = ;\n") + len(name) + len(value))
	out.WriteString("export type ")
	out.WriteString(name)
	out.WriteString(" = ")
	out.WriteString(value)
	out.WriteString(";\n")
	return out.String()
}

func renderInterface(name string, fields []fieldSpec) string {
	var out strings.Builder
	out.Grow(estimatedInterfaceSize(name, fields))
	out.WriteString("export interface ")
	out.WriteString(name)
	out.WriteString(" {\n")
	for _, field := range fields {
		writeFieldSpec(&out, "  ", field, ";\n")
	}
	out.WriteString("}\n")
	return out.String()
}

func renderInlineObject(fields []fieldSpec) string {
	var out strings.Builder
	out.Grow(estimatedInlineObjectSize(fields))
	out.WriteString("{ ")
	for idx, field := range fields {
		if idx > 0 {
			out.WriteString("; ")
		}
		writeFieldSpec(&out, "", field, "")
	}
	out.WriteString(" }")
	return out.String()
}

func writeFieldSpec(out *strings.Builder, prefix string, field fieldSpec, suffix string) {
	out.WriteString(prefix)
	out.WriteString(field.Name)
	if field.Optional {
		out.WriteByte('?')
	}
	out.WriteString(": ")
	out.WriteString(field.Type)
	out.WriteString(suffix)
}

func writeQuotedString(out *strings.Builder, value string) {
	if isSimpleQuotedString(value) {
		out.WriteByte('"')
		out.WriteString(value)
		out.WriteByte('"')
		return
	}
	out.WriteString(strconv.Quote(value))
}

func isSimpleQuotedString(value string) bool {
	if !utf8.ValidString(value) {
		return false
	}
	for _, char := range value {
		if char < ' ' || char == '"' || char == '\\' {
			return false
		}
	}
	return true
}

func estimatedInterfaceSize(name string, fields []fieldSpec) int {
	size := len("export interface  {\n}\n") + len(name)
	for _, field := range fields {
		size += fieldSpecSize("  ", field, ";\n")
	}
	return size
}

func estimatedInlineObjectSize(fields []fieldSpec) int {
	size := len("{  }")
	for idx, field := range fields {
		if idx > 0 {
			size += len("; ")
		}
		size += fieldSpecSize("", field, "")
	}
	return size
}

func fieldSpecSize(prefix string, field fieldSpec, suffix string) int {
	size := len(prefix) + len(field.Name) + len(": ") + len(field.Type) + len(suffix)
	if field.Optional {
		size++
	}
	return size
}

func estimatedEnumUnionSize(values []string) int {
	size := 0
	for idx, value := range values {
		if idx > 0 {
			size += len(" | ")
		}
		size += estimatedQuotedStringSize(value)
	}
	return size
}

func estimatedHookMapSize(header string, specs []extensioncontract.HookContractSpec, payload bool) int {
	size := len(header) + len("}\n")
	for _, spec := range specs {
		typeName := spec.Patch.Name
		if payload {
			typeName = spec.Payload.Name
		}
		size += len("  : ;\n") + estimatedQuotedStringSize(string(spec.Event)) + len(typeName)
	}
	return size
}

func estimatedHostMethodMapSize(specs []extensioncontract.HostAPIMethodSpec) int {
	size := len("export interface HostAPIMethodMap {\n}\n")
	for _, spec := range specs {
		size += len("  : {\n    params: ;\n    result: ;\n  };\n") +
			estimatedQuotedStringSize(string(spec.Method)) +
			estimatedHostParamsTypeSize(spec) +
			estimatedHostResultTypeSize(spec)
	}
	return size
}

func estimatedQuotedStringSize(value string) int {
	return len(value) + len(`""`)
}

func estimatedHostParamsTypeSize(spec extensioncontract.HostAPIMethodSpec) int {
	if !spec.OptionalParams {
		return len(spec.Params.Name)
	}
	if spec.Params.Name == "EmptyResult" {
		return len("undefined")
	}
	return len(spec.Params.Name) + len(" | undefined")
}

func estimatedHostResultTypeSize(spec extensioncontract.HostAPIMethodSpec) int {
	size := len(spec.Result.Name)
	if resultIsList(spec.Result.Value) {
		size += len("[]")
	}
	return size
}
