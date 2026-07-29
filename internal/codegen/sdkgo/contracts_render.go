package sdkgo

import (
	"fmt"
	"reflect"
	"unicode"

	"github.com/dave/jennifer/jen"

	extensioncontract "github.com/compozy/compozy/internal/extension/contract"
)

func renderHostMethods(specs []extensioncontract.HostAPIMethodSpec) ([]byte, error) {
	file := newGeneratedFile()
	file.Type().Id("HostAPIMethod").String()
	file.Line()
	definitions := make([]jen.Code, 0, len(specs))
	for _, spec := range specs {
		definitions = append(
			definitions,
			jen.Id("HostAPIMethod"+exportIdentifier(string(spec.Method))).
				Id("HostAPIMethod").Op("=").Lit(string(spec.Method)),
		)
	}
	file.Const().Defs(definitions...)
	return renderJenniferFile(file)
}

func renderHostContracts(specs []extensioncontract.HostAPIMethodSpec) ([]byte, error) {
	file := newGeneratedFile()
	file.Type().Id("HostAPIMethodContract").Types(
		jen.Id("P").Any(),
		jen.Id("R").Any(),
	).Struct(
		jen.Id("Method").Id("HostAPIMethod"),
		jen.Id("OptionalParams").Bool(),
	)
	file.Line()
	for _, spec := range specs {
		identifier := exportIdentifier(string(spec.Method))
		values := jen.Dict{jen.Id("Method"): jen.Id("HostAPIMethod" + identifier)}
		if spec.OptionalParams {
			values[jen.Id("OptionalParams")] = jen.True()
		}
		resultType := jen.Id(spec.Result.Name)
		if namedValueIsList(spec.Result.Value) {
			resultType = jen.Index().Add(resultType)
		}
		file.Var().Id("HostAPI"+identifier+"Contract").Op("=").
			Id("HostAPIMethodContract").Types(jen.Id(spec.Params.Name), resultType).Values(values)
	}
	return renderJenniferFile(file)
}

func namedValueIsList(value any) bool {
	if value == nil {
		return false
	}
	kind := reflect.TypeOf(value).Kind()
	return kind == reflect.Slice || kind == reflect.Array
}

func renderHookEvents(specs []extensioncontract.HookContractSpec) ([]byte, error) {
	file := newGeneratedFile()
	file.Type().Id("HookEvent").String()
	file.Line()
	definitions := make([]jen.Code, 0, len(specs))
	for _, spec := range specs {
		definitions = append(definitions,
			jen.Id("HookEvent"+exportIdentifier(string(spec.Event))).Id("HookEvent").Op("=").Lit(string(spec.Event)),
		)
	}
	file.Const().Defs(definitions...)
	return renderJenniferFile(file)
}

func renderHookContracts(specs []extensioncontract.HookContractSpec) ([]byte, error) {
	file := newGeneratedFile()
	file.Type().Id("HookContract").Types(
		jen.Id("P").Any(),
		jen.Id("Patch").Any(),
	).Struct(jen.Id("Event").Id("HookEvent"))
	file.Line()
	for _, spec := range specs {
		identifier := exportIdentifier(string(spec.Event))
		file.Var().Id("Hook"+identifier+"Contract").Op("=").
			Id("HookContract").Types(jen.Id(spec.Payload.Name), jen.Id(spec.Patch.Name)).Values(jen.Dict{
			jen.Id("Event"): jen.Id("HookEvent" + identifier),
		})
	}
	return renderJenniferFile(file)
}

func renderCapabilityContracts(contracts []extensioncontract.ProvideCapabilityContract) ([]byte, error) {
	file := newGeneratedFile()
	requiredMethods := jen.Dict{}
	for _, contract := range contracts {
		methods := make([]jen.Code, 0, len(contract.Methods))
		for _, method := range contract.Methods {
			methods = append(methods, jen.Lit(method))
		}
		requiredMethods[jen.Lit(contract.Provide)] = jen.Index().String().Values(methods...)
	}
	file.Var().Id("requiredMethodsByProvide").Op("=").Map(jen.String()).Index().String().Values(requiredMethods)
	file.Line()
	file.Func().Id("RequiredMethods").Params(jen.Id("provide").String()).Index().String().Block(
		jen.Id("methods").Op(":=").Id("requiredMethodsByProvide").Index(jen.Id("provide")),
		jen.If(jen.Len(jen.Id("methods")).Op("==").Lit(0)).Block(jen.Return(jen.Nil())),
		jen.Return(jen.Append(
			jen.Index().String().Call(jen.Nil()),
			jen.Id("methods").Op("..."),
		)),
	)
	file.Line()
	file.Type().Id("ProvideConformanceFixture").Struct(
		jen.Id("Provide").String(),
		jen.Id("RequiredMethods").Index().String(),
	)
	file.Line()
	fixtures := make([]jen.Code, 0, len(contracts))
	for _, contract := range contracts {
		if !contract.Public {
			continue
		}
		fixtures = append(fixtures, jen.Values(jen.Dict{
			jen.Id("Provide"):         jen.Lit(contract.Provide),
			jen.Id("RequiredMethods"): jen.Id("RequiredMethods").Call(jen.Lit(contract.Provide)),
		}))
	}
	file.Var().Id("publicProvideConformanceFixtures").Op("=").
		Index().Id("ProvideConformanceFixture").Values(fixtures...)
	file.Line()
	file.Func().Id("PublicProvideConformanceFixtures").Params().Index().Id("ProvideConformanceFixture").Block(
		jen.Id("fixtures").Op(":=").Make(
			jen.Index().Id("ProvideConformanceFixture"),
			jen.Len(jen.Id("publicProvideConformanceFixtures")),
		),
		jen.For(
			jen.List(jen.Id("index"), jen.Id("fixture")).
				Op(":=").Range().Id("publicProvideConformanceFixtures"),
		).Block(
			jen.Id("fixtures").Index(jen.Id("index")).Op("=").
				Id("ProvideConformanceFixture").Values(jen.Dict{
				jen.Id("Provide"): jen.Id("fixture").Dot("Provide"),
				jen.Id("RequiredMethods"): jen.Append(
					jen.Index().String().Values(),
					jen.Id("fixture").Dot("RequiredMethods").Op("..."),
				),
			}),
		),
		jen.Return(jen.Id("fixtures")),
	)
	return renderJenniferFile(file)
}

func exportIdentifier(value string) string {
	identifier := make([]rune, 0, len(value))
	upperNext := true
	for _, char := range value {
		if !unicode.IsLetter(char) && !unicode.IsDigit(char) {
			upperNext = true
			continue
		}
		if upperNext {
			identifier = append(identifier, unicode.ToUpper(char))
			upperNext = false
			continue
		}
		identifier = append(identifier, char)
	}
	if len(identifier) == 0 {
		return "Unknown"
	}
	return string(identifier)
}

func assertUniqueIdentifiers(values []string) error {
	seen := make(map[string]string, len(values))
	for _, value := range values {
		identifier := exportIdentifier(value)
		if previous, ok := seen[identifier]; ok && previous != value {
			return fmt.Errorf("generated identifier %s collides for %s and %s", identifier, previous, value)
		}
		seen[identifier] = value
	}
	return nil
}
