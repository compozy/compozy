package sdkcodegen

import (
	"fmt"
	"strings"

	"github.com/dave/jennifer/jen"
)

func buildOptionsFile() *jen.File {
	f := jen.NewFile(packageName)
	for i := range ResourceSpecs {
		spec := &ResourceSpecs[i]
		f.ImportAlias(spec.PackagePath, spec.ImportAlias)
	}
	for i := range ResourceSpecs {
		spec := &ResourceSpecs[i]
		f.Comment(optionComment(spec))
		f.Func().
			Id(fmt.Sprintf("With%s", spec.Name)).
			Params(jen.Id("cfg").Op("*").Qual(spec.PackagePath, spec.TypeName)).
			Id("Option").
			Block(
				jen.Return(
					jen.Func().
						Params(jen.Id("c").Op("*").Id("config")).
						BlockFunc(func(g *jen.Group) {
							g.If(
								jen.Id("c").Op("==").Nil().Op("||").Id("cfg").Op("==").Nil(),
							).Block(
								jen.Return(),
							)
							if spec.IsSlice {
								g.Id("c").
									Dot(spec.BuilderField).
									Op("=").
									Append(jen.Id("c").Dot(spec.BuilderField), jen.Id("cfg"))
							} else {
								g.Id("c").Dot(spec.BuilderField).Op("=").Id("cfg")
							}
						}),
				),
			)
	}
	return f
}

func optionComment(spec *ResourceSpec) string {
	resource := strings.ToLower(spec.Name)
	if strings.Contains(resource, "knowledge") {
		resource = "knowledge base"
	}
	return fmt.Sprintf("With%s registers a %s configuration for the engine.", spec.Name, resource)
}
