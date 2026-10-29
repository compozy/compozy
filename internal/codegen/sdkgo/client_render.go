package sdkgo

import (
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"

	"github.com/dave/jennifer/jen"
)

func renderClientFiles(
	g *typeGenerator,
	ops []ClientOperation,
	opts ClientOptions,
	reserved map[string]string,
) (map[string][]byte, error) {
	operations := make([]*jen.Statement, 0, len(ops)*2)
	owned := make(map[string]string)
	for _, op := range ops {
		name := exportIdentifier(op.ID)
		if len(op.Params) > 0 {
			owned[name+"Params"] = op.ID
			operations = append(operations, renderClientParams(op))
		}
		if len(op.ResponseVariants) > 1 {
			owned[name+"Frame"] = op.ID
			union, err := renderClientUnion(g, op)
			if err != nil {
				return nil, err
			}
			operations = append(operations, union...)
		}
		method, err := renderClientMethod(g, op)
		if err != nil {
			return nil, fmt.Errorf("render operation %s: %w", op.ID, err)
		}
		operations = append(operations, method)
	}
	if err := g.renderAll(); err != nil {
		return nil, err
	}
	names := slices.Sorted(maps.Keys(g.rendered))
	types := make([]*jen.Statement, 0, len(names))
	for _, name := range names {
		if owner, exists := reserved[name]; exists {
			return nil, fmt.Errorf("client DTO %s collides with %s", name, owner)
		}
		if owner, exists := owned[name]; exists {
			return nil, fmt.Errorf("client DTO %s collides with operation %s", name, owner)
		}
		types = append(types, jen.Comment(name+" is a daemon API value.").Line().Add(g.rendered[name]))
	}
	for name, owner := range owned {
		if reservedOwner, exists := reserved[name]; exists {
			return nil, fmt.Errorf("operation %s type %s collides with %s", owner, name, reservedOwner)
		}
	}
	files := make(map[string][]byte)
	for _, group := range []struct {
		prefix string
		blocks []*jen.Statement
	}{{"types", types}, {"operations", operations}} {
		if err := renderClientChunks(files, group.prefix, group.blocks, opts); err != nil {
			return nil, err
		}
	}
	return files, nil
}

func renderClientChunks(files map[string][]byte, prefix string, blocks []*jen.Statement, opts ClientOptions) error {
	fileNumber := 1
	var current []*jen.Statement
	var content []byte
	render := func(statements []*jen.Statement) ([]byte, error) {
		file := jen.NewFile(opts.PackageName)
		file.HeaderComment(opts.Header)
		for _, block := range statements {
			file.Add(block).Line()
		}
		return renderJenniferFile(file)
	}
	for _, block := range blocks {
		candidate := append(slices.Clone(current), block)
		next, err := render(candidate)
		if err != nil {
			return fmt.Errorf("render client %s: %w", prefix, err)
		}
		if strings.Count(string(next), "\n") > 450 && len(current) > 0 {
			files[fmt.Sprintf("%s_%03d_gen.go", prefix, fileNumber)] = content
			fileNumber++
			candidate = []*jen.Statement{block}
			next, err = render(candidate)
			if err != nil {
				return err
			}
		}
		current = candidate
		content = next
	}
	if len(current) > 0 {
		files[fmt.Sprintf("%s_%03d_gen.go", prefix, fileNumber)] = content
	}
	return nil
}

func renderClientParams(op ClientOperation) *jen.Statement {
	fields := make([]jen.Code, 0, len(op.Params))
	for _, param := range op.Params {
		field := jen.Id(clientParameterField(param))
		if !param.Required && param.In != clientParamInPath {
			field.Op("*")
		}
		field.Id(param.GoType).Tag(map[string]string{"url": param.Name})
		fields = append(fields, field)
	}
	name := exportIdentifier(op.ID) + "Params"
	return jen.Comment(name + " carries the path, query, and header parameters.").
		Line().
		Type().
		Id(name).
		Struct(fields...)
}

func renderClientMethod(g *typeGenerator, op ClientOperation) (*jen.Statement, error) {
	name := exportIdentifier(op.ID)
	args := []jen.Code{jen.Id("ctx").Qual("context", "Context")}
	legacyWorkspaceList := op.ID == "listWorkspaces"
	if len(op.Params) > 0 {
		args = append(args, jen.Id("params").Id(name+"Params"))
	}
	if legacyWorkspaceList {
		name += "WithParams"
	}
	body := jen.Nil()
	if op.RequestContentType == "multipart/form-data" {
		args = append(args, jen.Id("body").Op("*").Id("UploadFile"))
		body = jen.Id("body")
	} else if op.RequestBody != nil {
		bodyType, err := g.goType(reflect.TypeOf(op.RequestBody))
		if err != nil {
			return nil, err
		}
		args = append(args, jen.Id("body").Add(bodyType))
		body = jen.Id("body")
	}
	statements, headers := renderClientRequest(op)
	responseType, err := clientResponseType(g, op)
	if err != nil {
		return nil, err
	}
	result := []jen.Code{jen.Error()}
	if op.Kind == ClientOperationSSE {
		result = []jen.Code{jen.Op("*").Id("Stream").Types(responseType), jen.Error()}
		statements = append(statements, jen.Return(jen.Id("newStream").Types(responseType).Call(
			jen.Id("ctx"), jen.Id("c"), jen.Lit(op.Method), jen.Id("path"), headers, body, jen.Lit(op.ID),
		)))
	} else {
		output := jen.Nil()
		if responseType != nil {
			result = []jen.Code{jen.Op("*").Add(responseType), jen.Error()}
			statements = append(statements, jen.Var().Id("out").Add(responseType))
			output = jen.Op("&").Id("out")
		}
		call := jen.Id("c").Dot("doContent").Call(
			jen.Id("ctx"), jen.Lit(op.Method), jen.Id("path"), headers, body, output,
			jen.Lit(op.RequestContentType), jen.Lit(op.ResponseContentType),
		)
		if responseType == nil {
			statements = append(statements, jen.Return(call))
		} else {
			statements = append(
				statements,
				jen.If(jen.Id("err").Op(":=").Add(call), jen.Id("err").Op("!=").Nil()).
					Block(jen.Return(jen.Nil(), jen.Id("err"))),
				jen.Return(output, jen.Nil()),
			)
		}
	}
	summary := strings.Join(strings.Fields(op.Summary), " ")
	if summary == "" {
		summary = "calls " + op.ID + "."
	}
	comment := name + " " + summary
	if op.Stability == "experimental" {
		comment += " Experimental: pin the exact SDK version."
	}
	method := jen.Comment(comment).Line().Func().Params(jen.Id("c").Op("*").Id("Client")).
		Id(name).Params(args...).Params(result...).Block(statements...)
	if legacyWorkspaceList {
		wrapper := jen.Comment("ListWorkspaces lists active workspaces.").Line().Func().
			Params(jen.Id("c").Op("*").Id("Client")).Id("ListWorkspaces").
			Params(jen.Id("ctx").Qual("context", "Context")).Params(result...).
			Block(jen.Return(jen.Id("c").Dot(name).Call(jen.Id("ctx"), jen.Id("ListWorkspacesParams").Values())))
		return wrapper.Line().Line().Add(method), nil
	}
	return method, nil
}

func clientJSONResponse(op ClientOperation) bool {
	return op.ResponseContentType == "" || strings.Contains(op.ResponseContentType, "json")
}

func clientResponseType(g *typeGenerator, op ClientOperation) (*jen.Statement, error) {
	if len(op.ResponseVariants) > 1 {
		return jen.Id(exportIdentifier(op.ID) + "Frame"), nil
	}
	if op.Kind != ClientOperationSSE && !clientJSONResponse(op) {
		if strings.HasPrefix(op.ResponseContentType, "text/") {
			return jen.String(), nil
		}
		return jen.Index().Byte(), nil
	}
	if op.Response == nil {
		if op.Kind == ClientOperationSSE {
			return jen.Qual("encoding/json", "RawMessage"), nil
		}
		return nil, nil
	}
	typ := reflect.TypeOf(op.Response)
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	return g.goType(typ)
}

func renderClientRequest(op ClientOperation) ([]jen.Code, *jen.Statement) {
	statements := []jen.Code{jen.Id("path").Op(":=").Lit(op.Path)}
	headers := jen.Nil()
	query := false
	hasHeaders := false
	for _, param := range op.Params {
		if param.In == clientParamInHeader && !hasHeaders {
			statements = append(statements, jen.Id("headers").Op(":=").Qual("net/http", "Header").Values())
			headers = jen.Id("headers")
			hasHeaders = true
		}
		if param.In == clientParamInQuery && !query {
			statements = append(statements, jen.Id("query").Op(":=").Qual("net/url", "Values").Values())
			query = true
		}
		field := func() *jen.Statement { return jen.Id("params").Dot(clientParameterField(param)) }
		value := field()
		optional := !param.Required && param.In != clientParamInPath
		if optional {
			value = jen.Op("*").Add(value)
		}
		encoded := renderClientParamValue(param.GoType, value)
		var encode *jen.Statement
		switch param.In {
		case clientParamInPath:
			encode = jen.Id("path").Op("=").Qual("strings", "ReplaceAll").Call(
				jen.Id("path"), jen.Lit("{"+param.Name+"}"), jen.Qual("net/url", "PathEscape").Call(encoded),
			)
		case clientParamInQuery:
			if param.GoType == "[]string" {
				encode = jen.For(jen.List(jen.Id("_"), jen.Id("value")).Op(":=").Range().Add(value)).Block(
					jen.Id("query").Dot("Add").Call(jen.Lit(param.Name), jen.Id("value")),
				)
			} else {
				encode = jen.Id("query").Dot("Set").Call(jen.Lit(param.Name), encoded)
			}
		case clientParamInHeader:
			encode = headers.Clone().Dot("Set").Call(jen.Lit(param.Name), encoded)
		}
		if optional {
			encode = jen.If(field().Op("!=").Nil()).Block(encode)
		}
		statements = append(statements, encode)
	}
	if query {
		statements = append(statements, jen.If(jen.Len(jen.Id("query")).Op(">").Lit(0)).Block(
			jen.Id("path").Op("+=").Lit("?").Op("+").Id("query").Dot("Encode").Call(),
		))
	}
	return statements, headers
}

func renderClientParamValue(goType string, value *jen.Statement) *jen.Statement {
	switch goType {
	case "bool":
		return jen.Qual("strconv", "FormatBool").Call(value)
	case "int":
		return jen.Qual("strconv", "Itoa").Call(value)
	case "int64":
		return jen.Qual("strconv", "FormatInt").Call(value, jen.Lit(10))
	default:
		return value
	}
}
