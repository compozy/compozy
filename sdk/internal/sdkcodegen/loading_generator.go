package sdkcodegen

import (
	"fmt"
	"strings"

	"github.com/dave/jennifer/jen"
)

func buildLoadingFile() *jen.File {
	f := jen.NewFile(packageName)
	addLoadingImports(f)
	for i := range ResourceSpecs {
		spec := &ResourceSpecs[i]
		f.ImportAlias(spec.PackagePath, spec.ImportAlias)
		declareLoadFunctions(f, spec)
	}
	return f
}

func addLoadingImports(f *jen.File) {
	f.ImportName("context", "context")
	f.ImportName("fmt", "fmt")
	f.ImportName("os", "os")
	f.ImportAlias("path/filepath", "filepath")
	f.ImportName("sort", "sort")
	f.ImportName("strings", "strings")
}

func declareLoadFunctions(f *jen.File, spec *ResourceSpec) {
	addLoadFunction(f, spec)
	addLoadDirFunction(f, spec)
}

func addLoadFunction(f *jen.File, spec *ResourceSpec) {
	f.Comment(fmt.Sprintf("Load%s loads a %s configuration from disk.", spec.Name, loadingSubject(spec.Name)))
	f.Func().
		Params(jen.Id("e").Op("*").Id("Engine")).
		Id(fmt.Sprintf("Load%s", spec.Name)).
		Params(jen.Id("ctx").Qual("context", "Context"), jen.Id("path").String()).
		Error().
		Block(
			jen.If(jen.Id("e").Op("==").Nil()).Block(
				jen.Return(jen.Qual("fmt", "Errorf").Call(jen.Lit("engine is nil"))),
			),
			jen.List(jen.Id("cfg"), jen.Err()).
				Op(":=").
				Id("loadYAML").
				Types(jen.Op("*").Qual(spec.PackagePath, spec.TypeName)).
				Call(jen.Id("ctx"), jen.Id("e"), jen.Id("path")),
			jen.If(jen.Err().Op("!=").Nil()).Block(
				jen.Return(jen.Qual("fmt", "Errorf").Call(
					jen.Lit(fmt.Sprintf("load %s config: %%w", strings.ToLower(spec.Name))),
					jen.Err(),
				)),
			),
			jen.If(jen.Id("ctx").Dot("Err").Call().Op("!=").Nil()).Block(
				jen.Return(jen.Qual("fmt", "Errorf").Call(
					jen.Lit(fmt.Sprintf("load %s config: %%w", strings.ToLower(spec.Name))),
					jen.Id("ctx").Dot("Err").Call(),
				)),
			),
			jen.Return(jen.Id("e").Dot(fmt.Sprintf("Register%s", spec.Name)).Call(jen.Id("cfg"))),
		)
}

func addLoadDirFunction(f *jen.File, spec *ResourceSpec) {
	values := make([]jen.Code, 0, len(spec.FileExtensions))
	for _, ext := range spec.FileExtensions {
		values = append(values, jen.Lit(ext))
	}

	f.Comment(
		fmt.Sprintf(
			"Load%sFromDir loads %s configurations from a directory.",
			spec.PluralName,
			pluralSubject(spec.Name),
		),
	)
	f.Func().
		Params(jen.Id("e").Op("*").Id("Engine")).
		Id(fmt.Sprintf("Load%sFromDir", spec.PluralName)).
		Params(jen.Id("ctx").Qual("context", "Context"), jen.Id("dir").String()).
		Error().
		Block(
			jen.If(jen.Id("e").Op("==").Nil()).Block(
				jen.Return(jen.Qual("fmt", "Errorf").Call(jen.Lit("engine is nil"))),
			),
			jen.Return(
				jen.Id("e").Dot("loadFromDir").Call(
					jen.Id("ctx"),
					jen.Id("dir"),
					jen.Index().String().Values(values...),
					jen.Func().
						Params(
							jen.Id("loaderCtx").Qual("context", "Context"),
							jen.Id("path").String(),
						).
						Error().
						Block(
							jen.Return(jen.Id("e").Dot(fmt.Sprintf("Load%s", spec.Name)).Call(jen.Id("loaderCtx"), jen.Id("path"))),
						),
				),
			),
		)
}

func loadingSubject(name string) string {
	if strings.Contains(strings.ToLower(name), "knowledge") {
		return "knowledge base"
	}
	return strings.ToLower(name)
}

func pluralSubject(name string) string {
	return strings.ToLower(name) + "s"
}
