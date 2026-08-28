package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"go/format"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"github.com/compozy/compozy/internal/api/spec"
	terminalwire "github.com/compozy/compozy/internal/terminal/wire"
)

const (
	defaultTerminalWireManifestPath = "internal/terminal/wire/protocol.json"
	defaultTerminalWireGoPath       = "internal/terminal/wire/opcodes_generated.go"
	defaultTerminalWireTSPath       = "web/src/generated/terminal-wire.ts"
	defaultTerminalWireDocsPath     = "docs/design/generated/terminal-wire.md"
)

var (
	goOpcodeNamePattern        = regexp.MustCompile(`^(Server|Client)Op[A-Z][A-Za-z0-9]*$`)
	tsOpcodeNamePattern        = regexp.MustCompile(`^[a-z][A-Za-z0-9]*$`)
	terminalWireVersionPattern = regexp.MustCompile(`^compozy\.terminal\.v([0-9]+)$`)
)

type terminalWireManifest struct {
	Subprotocol       string               `json:"subprotocol"`
	CompatibilityHash string               `json:"compatibility_digest"`
	Limits            terminalWireLimits   `json:"limits"`
	Server            []terminalWireOpcode `json:"server"`
	Client            []terminalWireOpcode `json:"client"`
}

type terminalWireLimits struct {
	MaxInputBytes int `json:"max_input_bytes"`
	MinCols       int `json:"min_cols"`
	MaxCols       int `json:"max_cols"`
	MinRows       int `json:"min_rows"`
	MaxRows       int `json:"max_rows"`
}

type terminalWireOpcode struct {
	GoName string `json:"go_name"`
	TSName string `json:"ts_name"`
	Value  byte   `json:"value"`
}

type terminalWirePaths struct {
	manifest   string
	goOutput   string
	tsOutput   string
	docsOutput string
}

func runProtocolCodegenTarget(
	ctx context.Context,
	target string,
	loopEnumsPath string,
	wirePaths terminalWirePaths,
) error {
	switch target {
	case "loop-enums":
		return writeLoopEnums(ctx, loopEnumsPath)
	case "terminal-wire":
		return writeTerminalWire(ctx, wirePaths)
	default:
		return fmt.Errorf("unknown protocol codegen target %q", target)
	}
}

func writeTerminalWire(ctx context.Context, paths terminalWirePaths) error {
	return writeTerminalWireWith(ctx, paths, publishGeneratedFile)
}

func writeTerminalWireWith(
	ctx context.Context,
	paths terminalWirePaths,
	publish func(string, []byte) error,
) error {
	goContent, tsContent, docsContent, err := generateTerminalWire(ctx, paths)
	if err != nil {
		return err
	}
	return publishGeneratedArtifactSet([]generatedArtifact{
		{path: paths.goOutput, content: goContent},
		{path: paths.tsOutput, content: tsContent},
		{path: paths.docsOutput, content: docsContent},
	}, publish)
}

func checkTerminalWire(ctx context.Context, paths terminalWirePaths) error {
	goContent, tsContent, docsContent, err := generateTerminalWire(ctx, paths)
	if err != nil {
		return err
	}
	if err := checkFile(paths.goOutput, goContent); err != nil {
		return err
	}
	if err := checkFile(paths.tsOutput, tsContent); err != nil {
		return err
	}
	return checkFile(paths.docsOutput, docsContent)
}

func generateTerminalWire(ctx context.Context, paths terminalWirePaths) ([]byte, []byte, []byte, error) {
	manifest, err := readTerminalWireManifest(paths.manifest)
	if err != nil {
		return nil, nil, nil, err
	}
	goContent, err := format.Source(generateTerminalWireGo(manifest))
	if err != nil {
		return nil, nil, nil, fmt.Errorf("format terminal wire Go contract: %w", err)
	}
	tsContent, err := formatTypeScript(ctx, paths.tsOutput, generateTerminalWireTS(manifest))
	if err != nil {
		return nil, nil, nil, err
	}
	return goContent, tsContent, generateTerminalWireDocs(manifest), nil
}

func readTerminalWireManifest(path string) (terminalWireManifest, error) {
	content := terminalwire.ProtocolManifest
	if strings.TrimSpace(path) != "" {
		var err error
		content, err = os.ReadFile(path)
		if err != nil {
			return terminalWireManifest{}, fmt.Errorf("read terminal wire manifest: %w", err)
		}
	}
	var manifest terminalWireManifest
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return terminalWireManifest{}, fmt.Errorf("decode terminal wire manifest: %w", err)
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		return terminalWireManifest{}, errors.New("terminal wire manifest contains trailing JSON")
	}
	if err := validateTerminalWireManifest(manifest); err != nil {
		return terminalWireManifest{}, err
	}
	return manifest, nil
}

func validateTerminalWireManifest(manifest terminalWireManifest) error {
	versionMatch := terminalWireVersionPattern.FindStringSubmatch(manifest.Subprotocol)
	if len(versionMatch) != 2 {
		return fmt.Errorf("terminal wire subprotocol must carry an explicit version, got %q", manifest.Subprotocol)
	}
	version, ok := new(big.Int).SetString(versionMatch[1], 10)
	if !ok || version.Cmp(big.NewInt(2)) < 0 {
		return fmt.Errorf("terminal wire subprotocol version must be at least 2, got %q", versionMatch[1])
	}
	if err := validateTerminalWireLimits(manifest.Limits); err != nil {
		return err
	}
	directions := []struct {
		name    string
		opcodes []terminalWireOpcode
	}{{name: "server", opcodes: manifest.Server}, {name: "client", opcodes: manifest.Client}}
	for _, direction := range directions {
		seenGo := make(map[string]struct{}, len(direction.opcodes))
		seenTS := make(map[string]struct{}, len(direction.opcodes))
		for index, opcode := range direction.opcodes {
			if !goOpcodeNamePattern.MatchString(opcode.GoName) || !tsOpcodeNamePattern.MatchString(opcode.TSName) {
				return fmt.Errorf("terminal wire %s opcode %d has invalid names", direction.name, index)
			}
			if opcode.Value != byte(index+1) {
				return fmt.Errorf(
					"terminal wire %s opcode %s must be gap-free at %d",
					direction.name,
					opcode.GoName,
					index+1,
				)
			}
			if _, exists := seenGo[opcode.GoName]; exists {
				return fmt.Errorf("terminal wire %s repeats Go opcode %s", direction.name, opcode.GoName)
			}
			if _, exists := seenTS[opcode.TSName]; exists {
				return fmt.Errorf("terminal wire %s repeats TypeScript opcode %s", direction.name, opcode.TSName)
			}
			seenGo[opcode.GoName] = struct{}{}
			seenTS[opcode.TSName] = struct{}{}
		}
	}
	digest, err := terminalWireCompatibilityDigest(manifest)
	if err != nil {
		return err
	}
	if version.Cmp(big.NewInt(2)) == 0 && digest != terminalWireV2CompatibilityDigest {
		return fmt.Errorf(
			"terminal wire v2 contract changed incompatibly: got digest %s; bump the subprotocol version",
			digest,
		)
	}
	if manifest.CompatibilityHash != digest {
		return fmt.Errorf(
			"terminal wire compatibility_digest = %q, want %q",
			manifest.CompatibilityHash,
			digest,
		)
	}
	return nil
}

const terminalWireV2CompatibilityDigest = "db5bc0f26113f515237dc1eed6d4b2b974e7c4b2c4ac0e13be1417712e6937a2"

func validateTerminalWireLimits(limits terminalWireLimits) error {
	if limits.MaxInputBytes <= 0 {
		return errors.New("terminal wire max_input_bytes must be positive")
	}
	if limits.MinCols <= 0 || limits.MaxCols < limits.MinCols ||
		limits.MinRows <= 0 || limits.MaxRows < limits.MinRows {
		return errors.New("terminal wire dimension limits are invalid")
	}
	return nil
}

func terminalWireCompatibilityDigest(manifest terminalWireManifest) (string, error) {
	contract := struct {
		Limits terminalWireLimits   `json:"limits"`
		Server []terminalWireOpcode `json:"server"`
		Client []terminalWireOpcode `json:"client"`
	}{Limits: manifest.Limits, Server: manifest.Server, Client: manifest.Client}
	encoded, err := json.Marshal(contract)
	if err != nil {
		return "", fmt.Errorf("encode terminal wire compatibility contract: %w", err)
	}
	return fmt.Sprintf("%x", sha256.Sum256(encoded)), nil
}

func generateTerminalWireGo(manifest terminalWireManifest) []byte {
	var output strings.Builder
	output.WriteString("// Code generated by go run ./cmd/compozy-codegen terminal-wire. DO NOT EDIT.\n\n")
	output.WriteString("package wire\n\nconst (\n")
	fmt.Fprintf(&output, "\tSubprotocol = %q\n", manifest.Subprotocol)
	fmt.Fprintf(&output, "\tProtocolDescription = %q\n", terminalWireDescription(manifest))
	fmt.Fprintf(&output, "\tMaxInputBytes = %d\n", manifest.Limits.MaxInputBytes)
	fmt.Fprintf(&output, "\tMinCols = %d\n", manifest.Limits.MinCols)
	fmt.Fprintf(&output, "\tMaxCols = %d\n", manifest.Limits.MaxCols)
	fmt.Fprintf(&output, "\tMinRows = %d\n", manifest.Limits.MinRows)
	fmt.Fprintf(&output, "\tMaxRows = %d\n", manifest.Limits.MaxRows)
	writeTerminalGoOpcodes(&output, manifest.Server)
	writeTerminalGoOpcodes(&output, manifest.Client)
	output.WriteString(")\n")
	return []byte(output.String())
}

func terminalWireDescription(manifest terminalWireManifest) string {
	return fmt.Sprintf(
		"WebSocket upgrade using the binary %s subprotocol. Server frames: %s. Client frames: %s. "+
			"OUTPUT is one opcode byte, one u64 big-endian sequence, then raw bytes; every control frame is "+
			"one opcode byte followed by JSON.",
		manifest.Subprotocol,
		describeTerminalOpcodes(manifest.Server, "ServerOp"),
		describeTerminalOpcodes(manifest.Client, "ClientOp"),
	)
}

func describeTerminalOpcodes(opcodes []terminalWireOpcode, prefix string) string {
	values := make([]string, 0, len(opcodes))
	for _, opcode := range opcodes {
		values = append(values, fmt.Sprintf("%s=0x%02X", terminalOpcodeLabel(opcode.GoName, prefix), opcode.Value))
	}
	return strings.Join(values, ", ")
}

func terminalOpcodeLabel(name, prefix string) string {
	name = strings.TrimPrefix(name, prefix)
	var label strings.Builder
	for index, value := range name {
		if index > 0 && unicode.IsUpper(value) {
			label.WriteByte('_')
		}
		label.WriteRune(unicode.ToUpper(value))
	}
	return label.String()
}

func writeTerminalGoOpcodes(output *strings.Builder, opcodes []terminalWireOpcode) {
	for _, opcode := range opcodes {
		fmt.Fprintf(output, "\t%s byte = 0x%02x\n", opcode.GoName, opcode.Value)
	}
}

func generateTerminalWireTS(manifest terminalWireManifest) []byte {
	var output strings.Builder
	output.WriteString("// Code generated by go run ./cmd/compozy-codegen terminal-wire. DO NOT EDIT.\n\n")
	fmt.Fprintf(&output, "export const TERMINAL_SUBPROTOCOL = %q;\n\n", manifest.Subprotocol)
	fmt.Fprintf(&output, "export const TERMINAL_MAX_INPUT_BYTES = %d;\n", manifest.Limits.MaxInputBytes)
	fmt.Fprintf(&output, "export const TERMINAL_MIN_COLS = %d;\n", manifest.Limits.MinCols)
	fmt.Fprintf(&output, "export const TERMINAL_MAX_COLS = %d;\n", manifest.Limits.MaxCols)
	fmt.Fprintf(&output, "export const TERMINAL_MIN_ROWS = %d;\n", manifest.Limits.MinRows)
	fmt.Fprintf(&output, "export const TERMINAL_MAX_ROWS = %d;\n\n", manifest.Limits.MaxRows)
	writeTerminalOpcodeMap(&output, "TERMINAL_SERVER_OP", manifest.Server)
	writeTerminalOpcodeMap(&output, "TERMINAL_CLIENT_OP", manifest.Client)
	return []byte(output.String())
}

func writeTerminalOpcodeMap(output *strings.Builder, name string, opcodes []terminalWireOpcode) {
	fmt.Fprintf(output, "export const %s = {\n", name)
	for _, opcode := range opcodes {
		fmt.Fprintf(output, "  %s: 0x%02x,\n", opcode.TSName, opcode.Value)
	}
	output.WriteString("} as const;\n\n")
}

func generateTerminalWireDocs(manifest terminalWireManifest) []byte {
	var output strings.Builder
	output.WriteString("<!-- Code generated by go run ./cmd/compozy-codegen terminal-wire. DO NOT EDIT. -->\n\n")
	output.WriteString("# Integrated terminal wire protocol\n\n")
	fmt.Fprintf(
		&output,
		"The required WebSocket subprotocol is `%s`. Version 1 is rejected; there is no negotiation, alias, or fallback.\n\n",
		manifest.Subprotocol,
	)
	output.WriteString(
		"`OUTPUT` frames contain one opcode byte, one unsigned 64-bit big-endian sequence, then raw bytes. All control frames contain one opcode byte followed by JSON.\n\n",
	)
	writeTerminalOpcodeTable(&output, "Server to client", manifest.Server, "ServerOp")
	writeTerminalOpcodeTable(&output, "Client to server", manifest.Client, "ClientOp")
	output.WriteString(
		"`REDACTED_INPUT` is daemon-owned JSON with `seq` and `characters`; PTY output can never create this frame. `PRESENCE` reports the current viewer count. `RELEASE` yields the active write lease without detaching.\n",
	)
	fmt.Fprintf(
		&output,
		"\nObservable limits: input frames are at most %d bytes; columns are clamped to %d–%d and rows to %d–%d.\n",
		manifest.Limits.MaxInputBytes,
		manifest.Limits.MinCols,
		manifest.Limits.MaxCols,
		manifest.Limits.MinRows,
		manifest.Limits.MaxRows,
	)
	return []byte(output.String())
}

func writeTerminalOpcodeTable(
	output *strings.Builder,
	title string,
	opcodes []terminalWireOpcode,
	prefix string,
) {
	fmt.Fprintf(output, "## %s\n\n| Frame | Opcode |\n| --- | --- |\n", title)
	for _, opcode := range opcodes {
		fmt.Fprintf(output, "| `%s` | `0x%02X` |\n", terminalOpcodeLabel(opcode.GoName, prefix), opcode.Value)
	}
	output.WriteByte('\n')
}

func terminalWirePathsFor(openapiPath string) terminalWirePaths {
	if filepath.Clean(openapiPath) == filepath.Clean(spec.DefaultPath) {
		return terminalWirePaths{
			goOutput:   defaultTerminalWireGoPath,
			tsOutput:   defaultTerminalWireTSPath,
			docsOutput: defaultTerminalWireDocsPath,
		}
	}
	directory := filepath.Dir(openapiPath)
	return terminalWirePaths{
		goOutput:   filepath.Join(directory, "opcodes_generated.go"),
		tsOutput:   filepath.Join(directory, "terminal-wire.ts"),
		docsOutput: filepath.Join(directory, "terminal-wire.md"),
	}
}
