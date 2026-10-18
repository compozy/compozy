package main

import (
	"fmt"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/api/spec"
	"github.com/compozy/compozy/internal/codegen/sdkgo"
)

const (
	sdkDirectory      = "sdk"
	sdkClientPackage  = "client"
	sdkKindWebSocket  = "websocket"
	sdkKindPrompt     = "prompt"
	sdkKindSSE        = "sse"
	sdkParamString    = "string"
	sdkParamInteger   = "integer"
	sdkParamBoolean   = "boolean"
	sdkContentTypeSSE = "text/event-stream"
)

func sdkClientPathFor(openapiPath string) string {
	return filepath.Join(filepath.Dir(filepath.Dir(openapiPath)), sdkDirectory, "go", sdkClientPackage)
}

func generateSDKClient() (sdkgo.Result, error) {
	ops, err := sdkClientOperations(spec.Operations())
	if err != nil {
		return sdkgo.Result{}, err
	}
	return sdkgo.GenerateClient(ops, sdkgo.ClientOptions{
		PackageName: sdkClientPackage,
		Types:       []sdkgo.ClientType{{Name: "SendPromptResultPayload", Value: contract.SendPromptResultPayload{}}},
	})
}

func sdkClientOperations(registry []spec.OperationSpec) ([]sdkgo.ClientOperation, error) {
	ops := make([]sdkgo.ClientOperation, 0, len(registry))
	for _, source := range registry {
		if source.OperationID == "sendSessionPrompt" || sdkOperationKind(source) == sdkKindWebSocket {
			continue
		}
		op := sdkgo.ClientOperation{
			ID: source.OperationID, Method: source.Method, Path: source.Path,
			Summary: source.Summary, Stability: source.Stability,
			Kind: sdkgo.ClientOperationJSON, RequestBody: source.RequestBody,
			RequestContentType: source.RequestContentType,
		}
		for _, parameter := range source.Parameters {
			kind, err := sdkParameterType(parameter)
			if err != nil {
				return nil, fmt.Errorf("operation %s: %w", source.OperationID, err)
			}
			name := ""
			if parameter.In == "query" && parameter.Name == "last_event_id" {
				name = "LastEventIDQuery"
			}
			op.Params = append(op.Params, sdkgo.ClientParam{
				GoName: name,
				Name:   parameter.Name, In: parameter.In, Required: parameter.Required,
				GoType: kind, Enum: parameter.Enum,
			})
		}
		for _, response := range source.Responses {
			if response.Status < 200 || response.Status >= 300 {
				continue
			}
			op.ResponseContentType = response.ContentType
			if response.ContentType == sdkContentTypeSSE {
				op.Kind = sdkgo.ClientOperationSSE
			}
			// Reflection only adapts compile-time DTO types during code generation.
			for _, bodyType := range response.BodyTypes() {
				value := reflect.Zero(bodyType).Interface()
				op.ResponseVariants = append(op.ResponseVariants, value)
			}
			if len(op.ResponseVariants) == 1 {
				op.Response = op.ResponseVariants[0]
				op.ResponseVariants = nil
			}
			if response.ContentType != "" && response.ContentType != "application/json" &&
				response.ContentType != sdkContentTypeSSE {
				op.Response = []byte{}
				op.ResponseVariants = nil
			}
			break
		}
		ops = append(ops, op)
	}
	return ops, nil
}

func sdkParameterType(parameter spec.ParameterSpec) (string, error) {
	switch parameter.Kind {
	case "", sdkParamString:
		return sdkParamString, nil
	case "array":
		return "[]string", nil
	case sdkParamBoolean:
		return "bool", nil
	case sdkParamInteger:
		if parameter.Format == "int64" {
			return "int64", nil
		}
		return "int", nil
	default:
		return "", fmt.Errorf("unsupported parameter %s type %q", parameter.Name, parameter.Kind)
	}
}

func sdkOperationKind(operation spec.OperationSpec) string {
	if operation.OperationID == "sendSessionPrompt" {
		return sdkKindPrompt
	}
	upgrade := false
	for _, response := range operation.Responses {
		if response.ContentType == sdkContentTypeSSE {
			return sdkKindSSE
		}
		if response.Status == 101 {
			upgrade = true
		}
	}
	if strings.Contains(operation.Stream, sdkKindSSE) {
		return sdkKindSSE
	}
	if upgrade {
		return sdkKindWebSocket
	}
	return "json"
}
