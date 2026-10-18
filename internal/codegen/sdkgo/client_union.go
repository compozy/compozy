package sdkgo

import (
	"fmt"
	"reflect"

	"github.com/dave/jennifer/jen"
)

func renderClientUnion(g *typeGenerator, op ClientOperation) ([]*jen.Statement, error) {
	name := exportIdentifier(op.ID) + "Frame"
	blocks := []*jen.Statement{
		jen.Comment(name + " preserves each event payload; decode its variant using the event name.").Line().
			Type().Id(name).Struct(jen.Id("Raw").Qual("encoding/json", "RawMessage")),
		jen.Var().Id("_").Qual("encoding/json", "Unmarshaler").Op("=").Parens(jen.Op("*").Id(name)).Call(jen.Nil()),
		jen.Var().Id("_").Qual("encoding/json", "Marshaler").Op("=").Id(name).Values(),
		jen.Comment("UnmarshalJSON retains the complete event payload.").Line().Func().
			Params(jen.Id("f").Op("*").Id(name)).Id("UnmarshalJSON").Params(jen.Id("data").Index().Byte()).Error().Block(
			jen.Return(jen.Qual("encoding/json", "Unmarshal").Call(jen.Id("data"), jen.Op("&").Id("f").Dot("Raw"))),
		),
		jen.Comment("MarshalJSON returns the original event payload.").Line().Func().
			Params(jen.Id("f").Id(name)).Id("MarshalJSON").Params().Params(jen.Index().Byte(), jen.Error()).Block(
			jen.Return(jen.Qual("encoding/json", "Marshal").Call(jen.Id("f").Dot("Raw"))),
		),
	}
	seen := make(map[string]bool)
	for index, variant := range op.ResponseVariants {
		if variant == nil {
			return nil, fmt.Errorf("operation %s has a nil response variant", op.ID)
		}
		typ := reflect.TypeOf(variant)
		for typ.Kind() == reflect.Pointer {
			typ = typ.Elem()
		}
		code, err := g.goType(typ)
		if err != nil {
			return nil, fmt.Errorf("operation %s response variant: %w", op.ID, err)
		}
		variantName := g.preferred[typ]
		if variantName == "" {
			variantName = fmt.Sprintf("Variant%d", index+1)
		}
		methodName := "As" + variantName
		if seen[methodName] {
			continue
		}
		seen[methodName] = true
		blocks = append(blocks, jen.Comment(methodName+" decodes this payload as "+variantName+".").Line().Func().
			Params(jen.Id("f").Id(name)).Id(methodName).Params().Params(jen.Op("*").Add(code), jen.Error()).Block(
			jen.Var().Id("out").Add(code),
			jen.If(jen.Id("err").Op(":=").Qual("encoding/json", "Unmarshal").Call(jen.Id("f").Dot("Raw"), jen.Op("&").Id("out")),
				jen.Id("err").Op("!=").Nil()).Block(jen.Return(jen.Nil(), jen.Id("err"))),
			jen.Return(jen.Op("&").Id("out"), jen.Nil()),
		))
	}
	return blocks, nil
}
