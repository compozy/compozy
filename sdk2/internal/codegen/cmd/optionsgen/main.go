package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/compozy/compozy/sdk/v2/internal/codegen"
)

func main() {
	var (
		engineFile = flag.String("engine", "", "Path to engine struct file (e.g., ../../../../engine/agent/config.go)")
		structName = flag.String("struct", "Config", "Name of struct to generate options for")
		output     = flag.String("output", "", "Output file path (e.g., ../../agent/options_generated.go)")
		pkgName    = flag.String(
			"package",
			"",
			"Package name for generated code (optional, defaults to engine package name)",
		)
	)
	flag.Parse()
	if *engineFile == "" || *output == "" {
		fmt.Fprintf(os.Stderr, "Usage: optionsgen -engine <file> -struct <name> -output <file> [-package <name>]\n\n")
		fmt.Fprintf(os.Stderr, "Example:\n")
		fmt.Fprintf(os.Stderr, "  optionsgen -engine ../../../../engine/agent/config.go ")
		fmt.Fprintf(os.Stderr, "-struct Config -output ../../agent/options_generated.go\n\n")
		flag.Usage()
		os.Exit(1)
	}
	info, err := codegen.ParseStruct(*engineFile, *structName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Failed to parse struct: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✅ Discovered %d fields from %s.%s\n", len(info.Fields), info.PackageName, info.StructName)
	for _, field := range info.Fields {
		typeInfo := field.Type
		if field.IsSlice {
			typeInfo = "[]" + field.ValueType
		}
		if field.IsMap {
			typeInfo = fmt.Sprintf("map[%s]%s", field.KeyType, field.ValueType)
		}
		if field.IsPtr {
			typeInfo = "*" + typeInfo
		}
		fmt.Printf("  - %s: %s\n", field.Name, typeInfo)
	}
	targetPkg := info.PackageName
	if *pkgName != "" {
		targetPkg = *pkgName
	}
	if err := codegen.GenerateOptions(info, *output, targetPkg); err != nil {
		fmt.Fprintf(os.Stderr, "❌ Failed to generate options: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("\n✅ Generated %s with %d option functions\n", *output, len(info.Fields))
}
