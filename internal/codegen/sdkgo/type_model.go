package sdkgo

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/dave/jennifer/jen"

	extensioncontract "github.com/compozy/compozy/internal/extension/contract"
)

var (
	rawMessageType = reflect.TypeFor[json.RawMessage]()
	timeType       = reflect.TypeFor[time.Time]()
	durationType   = reflect.TypeFor[time.Duration]()
)

type typeGenerator struct {
	declarations map[string]reflect.Type
	preferred    map[reflect.Type]string
	rendered     map[string]*jen.Statement
}

var dedicatedGeneratedTypes = map[string]struct{}{
	"HookEvent":     {},
	"HostAPIMethod": {},
}

func newTypeGenerator() *typeGenerator {
	return &typeGenerator{
		declarations: make(map[string]reflect.Type),
		preferred:    make(map[reflect.Type]string),
		rendered:     make(map[string]*jen.Statement),
	}
}

func (g *typeGenerator) addNamed(named extensioncontract.NamedType) error {
	name := strings.TrimSpace(named.Name)
	if name == "" || named.Value == nil {
		return nil
	}
	typeValue := declarationType(reflect.TypeOf(named.Value))
	if _, dedicated := dedicatedGeneratedTypes[name]; dedicated {
		g.preferred[typeValue] = name
		return nil
	}
	if existing, ok := g.declarations[name]; ok && existing != typeValue {
		return fmt.Errorf("generated Go contract name %s maps to both %s and %s", name, existing, typeValue)
	}
	g.declarations[name] = typeValue
	if _, exists := g.preferred[typeValue]; !exists {
		g.preferred[typeValue] = name
	}
	return nil
}

func declarationType(value reflect.Type) reflect.Type {
	for value.Kind() == reflect.Pointer {
		value = value.Elem()
	}
	if value.Kind() == reflect.Slice || value.Kind() == reflect.Array {
		value = value.Elem()
	}
	return value
}

func (g *typeGenerator) renderAll() error {
	for {
		names := make([]string, 0, len(g.declarations))
		for name := range g.declarations {
			if _, ok := g.rendered[name]; !ok {
				names = append(names, name)
			}
		}
		if len(names) == 0 {
			return nil
		}
		sort.Strings(names)
		for _, name := range names {
			block, err := g.renderDeclaration(name, g.declarations[name])
			if err != nil {
				return err
			}
			g.rendered[name] = block
		}
	}
}

func (g *typeGenerator) ensureInternalNamed(value reflect.Type) string {
	typeValue := declarationType(value)
	if name, ok := g.preferred[typeValue]; ok {
		return name
	}
	if typeValue.Name() == "" || !strings.HasPrefix(typeValue.PkgPath(), "github.com/compozy/compozy/internal/") {
		return ""
	}
	name := typeValue.Name()
	if existing, ok := g.declarations[name]; ok && existing != typeValue {
		name = g.uniqueGeneratedName(typeValue)
	}
	g.declarations[name] = typeValue
	g.preferred[typeValue] = name
	return name
}

func (g *typeGenerator) uniqueGeneratedName(value reflect.Type) string {
	packageParts := strings.Split(strings.Trim(value.PkgPath(), "/"), "/")
	prefix := "Contract"
	if len(packageParts) > 0 && packageParts[len(packageParts)-1] != "" {
		prefix = exportIdentifier(packageParts[len(packageParts)-1])
	}
	base := prefix + value.Name()
	candidate := base
	for suffix := 2; ; suffix++ {
		if existing, ok := g.declarations[candidate]; !ok || existing == value {
			return candidate
		}
		candidate = fmt.Sprintf("%s%d", base, suffix)
	}
}
