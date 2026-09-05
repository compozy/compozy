package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"os"
	"regexp"
	"strings"

	terminalwire "github.com/compozy/compozy/internal/terminal/wire"
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
	if frozenDigest, frozen := terminalWireCompatibilityDigests[version.String()]; frozen && digest != frozenDigest {
		return fmt.Errorf(
			"terminal wire v%s contract changed incompatibly: got digest %s; bump the subprotocol version",
			version.String(),
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

var terminalWireCompatibilityDigests = map[string]string{
	"2": "db5bc0f26113f515237dc1eed6d4b2b974e7c4b2c4ac0e13be1417712e6937a2",
	"3": "a2a0bc2049718faa7dc87ffb757852434937c9ea1cc6750f10423bf3b2b5b889",
}

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
