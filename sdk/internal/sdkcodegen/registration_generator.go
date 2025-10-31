package sdkcodegen

import (
	"fmt"
	"strings"

	"github.com/dave/jennifer/jen"
)

func buildRegistrationFile() *jen.File {
	f := jen.NewFile(packageName)
	f.ImportName("fmt", "fmt")
	for i := range ResourceSpecs {
		spec := &ResourceSpecs[i]
		f.ImportAlias(spec.PackagePath, spec.ImportAlias)
	}
	for i := range ResourceSpecs {
		spec := &ResourceSpecs[i]
		f.Comment(fmt.Sprintf("Register%s registers a %s for runtime execution.", spec.Name, registrationSubject(spec)))
		f.Func().
			Params(jen.Id("e").Op("*").Id("Engine")).
			Id(fmt.Sprintf("Register%s", spec.Name)).
			Params(jen.Id("cfg").Op("*").Qual(spec.PackagePath, spec.TypeName)).
			Params(jen.Error()).
			BlockFunc(func(g *jen.Group) {
				requiredMessage := fmt.Sprintf("%s config is required", strings.ToLower(spec.Name))
				g.If(jen.Id("e").Op("==").Nil()).Block(
					jen.Return(jen.Qual("fmt", "Errorf").Call(jen.Lit("engine is nil"))),
				)
				g.If(jen.Id("cfg").Op("==").Nil()).Block(
					jen.Return(jen.Qual("fmt", "Errorf").Call(jen.Lit(requiredMessage))),
				)
				if spec.IsSlice {
					g.Id("e").Dot(spec.BuilderField).Op("=").Append(jen.Id("e").Dot(spec.BuilderField), jen.Id("cfg"))
				} else {
					g.Id("e").Dot(spec.BuilderField).Op("=").Id("cfg")
				}
				g.Return(jen.Nil())
			})
	}
	return f
}

func registrationSubject(spec *ResourceSpec) string {
	name := strings.ToLower(spec.Name)
	if strings.Contains(name, "knowledge") {
		return "knowledge base configuration"
	}
	return fmt.Sprintf("%s configuration", name)
}
