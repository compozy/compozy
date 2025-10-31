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
	addLoadHelpers(f)
	return f
}

func addLoadingImports(f *jen.File) {
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
		Params(jen.Id("path").String()).
		Error().
		Block(
			jen.If(jen.Id("e").Op("==").Nil()).Block(
				jen.Return(jen.Qual("fmt", "Errorf").Call(jen.Lit("engine is nil"))),
			),
			jen.List(jen.Id("cfg"), jen.Err()).
				Op(":=").
				Id("loadYAML").
				Types(jen.Op("*").Qual(spec.PackagePath, spec.TypeName)).
				Call(jen.Id("path")),
			jen.If(jen.Err().Op("!=").Nil()).Block(
				jen.Return(jen.Qual("fmt", "Errorf").Call(
					jen.Lit(fmt.Sprintf("load %s config: %%w", strings.ToLower(spec.Name))),
					jen.Err(),
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
		Params(jen.Id("dir").String()).
		Error().
		Block(
			jen.If(jen.Id("e").Op("==").Nil()).Block(
				jen.Return(jen.Qual("fmt", "Errorf").Call(jen.Lit("engine is nil"))),
			),
			jen.Return(
				jen.Id("loadFromDir").Call(
					jen.Id("dir"),
					jen.Index().String().Values(values...),
					jen.Func().Params(jen.Id("path").String()).Error().Block(
						jen.Return(jen.Id("e").Dot(fmt.Sprintf("Load%s", spec.Name)).Call(jen.Id("path"))),
					),
				),
			),
		)
}

func addLoadHelpers(f *jen.File) {
	emitLoadYAMLHelper(f)
	emitLoadFromDirHelper(f)
	emitMatchesExtensionHelper(f)
}

func emitLoadYAMLHelper(f *jen.File) {
	f.Comment("loadYAML decodes a YAML file into the provided generic type.")
	f.Func().
		Id("loadYAML").
		Types(jen.Id("T").Any()).
		Params(jen.Id("path").String()).
		Params(jen.Id("T"), jen.Error()).
		Block(
			jen.Var().Id("zero").Id("T"),
			jen.Id("cleaned").Op(":=").Qual("strings", "TrimSpace").Call(jen.Id("path")),
			jen.If(jen.Id("cleaned").Op("==").Lit("")).Block(
				jen.Return(jen.Id("zero"), jen.Qual("fmt", "Errorf").Call(jen.Lit("path is required"))),
			),
			jen.List(jen.Id("data"), jen.Err()).Op(":=").Qual("os", "ReadFile").Call(jen.Id("cleaned")),
			jen.If(jen.Err().Op("!=").Nil()).Block(
				jen.Return(jen.Id("zero"), jen.Qual("fmt", "Errorf").Call(jen.Lit("read %s: %w"), jen.Id("cleaned"), jen.Err())),
			),
			jen.Var().Id("value").Id("T"),
			jen.If(
				jen.Err().
					Op("=").
					Qual("gopkg.in/yaml.v3", "Unmarshal").
					Call(
						jen.Id("data"),
						jen.Op("&").Id("value"),
					),
				jen.Err().Op("!=").Nil(),
			).Block(
				jen.Return(jen.Id("zero"), jen.Qual("fmt", "Errorf").Call(jen.Lit("decode %s: %w"), jen.Id("cleaned"), jen.Err())),
			),
			jen.Return(jen.Id("value"), jen.Nil()),
		)
}

func emitLoadFromDirHelper(f *jen.File) {
	f.Comment("loadFromDir applies loader to all files in dir matching the provided extensions.")
	f.Func().
		Id("loadFromDir").
		Params(
			jen.Id("dir").String(),
			jen.Id("extensions").Index().String(),
			jen.Id("loader").Func().Params(jen.String()).Error(),
		).
		Error().
		Block(
			jen.Id("cleaned").Op(":=").Qual("strings", "TrimSpace").Call(jen.Id("dir")),
			jen.If(jen.Id("cleaned").Op("==").Lit("")).Block(
				jen.Return(jen.Qual("fmt", "Errorf").Call(jen.Lit("directory is required"))),
			),
			jen.List(jen.Id("entries"), jen.Err()).Op(":=").Qual("os", "ReadDir").Call(jen.Id("cleaned")),
			jen.If(jen.Err().Op("!=").Nil()).Block(
				jen.Return(jen.Qual("fmt", "Errorf").Call(jen.Lit("read dir %s: %w"), jen.Id("cleaned"), jen.Err())),
			),
			jen.Id("files").Op(":=").Make(jen.Index().String(), jen.Lit(0), jen.Lit(8)),
			jen.For(jen.List(jen.Id("_"), jen.Id("entry")).Op(":=").Range().Id("entries")).Block(
				jen.If(jen.Id("entry").Dot("IsDir").Call()).Block(jen.Continue()),
				jen.If(jen.Op("!").Id("matchesExtension").Call(jen.Id("entry").Dot("Name").Call(), jen.Id("extensions"))).
					Block(jen.Continue()),
				jen.Id("files").
					Op("=").
					Append(
						jen.Id("files"),
						jen.Qual("path/filepath", "Join").Call(
							jen.Id("cleaned"),
							jen.Id("entry").Dot("Name").Call(),
						),
					),
			),
			jen.Qual("sort", "Strings").Call(jen.Id("files")),
			jen.For(jen.List(jen.Id("_"), jen.Id("file")).Op(":=").Range().Id("files")).Block(
				jen.If(
					jen.Err().Op("=").Id("loader").Call(jen.Id("file")),
					jen.Err().Op("!=").Nil(),
				).Block(jen.Return(jen.Err())),
			),
			jen.Return(jen.Nil()),
		)
}

func emitMatchesExtensionHelper(f *jen.File) {
	f.Comment("matchesExtension reports whether name has any of the provided extensions.")
	f.Func().
		Id("matchesExtension").
		Params(
			jen.Id("name").String(),
			jen.Id("extensions").Index().String(),
		).
		Bool().
		Block(
			jen.Id("lower").Op(":=").Qual("strings", "ToLower").Call(jen.Id("name")),
			jen.For(jen.List(jen.Id("_"), jen.Id("ext")).Op(":=").Range().Id("extensions")).Block(
				jen.If(jen.Qual("strings", "HasSuffix").Call(jen.Id("lower"), jen.Qual("strings", "ToLower").Call(jen.Id("ext")))).
					Block(
						jen.Return(jen.True()),
					),
			),
			jen.Return(jen.False()),
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
