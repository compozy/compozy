package main

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/api/spec"
	"github.com/compozy/compozy/internal/codegen/sdkgo"
)

const sdkSampleTrue = "true"

type sdkSample struct {
	TypeScript string         `json:"typescript"`
	Go         string         `json:"go"`
	Kind       string         `json:"kind"`
	Resource   string         `json:"resource"`
	AppAccess  spec.AppAccess `json:"app_access"`
}

type sdkSampleManifest struct {
	Version    int                  `json:"version"`
	Operations map[string]sdkSample `json:"operations"`
}

func sdkSamplesPathFor(openapiPath string) string {
	return filepath.Join(filepath.Dir(openapiPath), "sdk-samples.json")
}

func generateSDKSamples() ([]byte, error) {
	client, err := generateSDKClient()
	if err != nil {
		return nil, err
	}
	bodies, err := sdkGoRequestExpressions(client.Files)
	if err != nil {
		return nil, err
	}
	manifest := sdkSampleManifest{Version: 1, Operations: make(map[string]sdkSample)}
	for _, operation := range spec.Operations() {
		kind := sdkOperationKind(operation)
		sample := sdkSample{Kind: kind, Resource: sdkResourceName(operation), AppAccess: operation.AppAccess}
		if kind != sdkKindWebSocket {
			sample.TypeScript = "import { Compozy } from \"@compozy/client\";\n\n" +
				"const compozy = await Compozy.connect();\n" + sdkTypeScriptSample(operation, kind)
			connection := "client.New(ctx)"
			if !slices.Contains(operation.Transports, spec.TransportUDS) {
				connection = `client.New(ctx, client.WithBaseURL("http://127.0.0.1:2123"))`
			}
			sample.Go = "c, err := " + connection + "\nif err != nil { return err }\n\n" + sdkGoSample(
				operation,
				kind,
				bodies[sdkgo.ClientMethodName(operation.OperationID)],
			)
		}
		manifest.Operations[operation.OperationID] = sample
	}
	content, err := json.Marshal(manifest, json.Deterministic(true), jsontext.WithIndent("    "))
	if err != nil {
		return nil, fmt.Errorf("encode SDK samples: %w", err)
	}
	return append(content, '\n'), nil
}

func sdkTypeScriptSample(operation spec.OperationSpec, kind string) string {
	if kind == sdkKindPrompt {
		return "const reply = await compozy.prompt({\n  workspaceId: \"workspace-id\",\n" +
			"  sessionId: \"session-id\",\n  message: \"Hello\",\n});\n" +
			"if (reply.kind === \"stream\") {\n" +
			"  for await (const chunk of reply.chunks) console.log(chunk);\n}"
	}
	options := sdkTypeScriptOptions(operation)
	if kind == sdkKindSSE {
		return fmt.Sprintf(
			"for await (const frame of compozy.events(%q, %s)) {\n  console.log(frame);\n}",
			operation.OperationID,
			options,
		)
	}
	if options == "{}" {
		options = ""
	}
	return fmt.Sprintf(
		"const result = await compozy.%s.%s(%s);",
		sdkResourceName(operation),
		operation.OperationID,
		options,
	)
}

func sdkTypeScriptOptions(operation spec.OperationSpec) string {
	fields := make([]string, 0, 4)
	for _, location := range []string{"path", "query", "header"} {
		params := make([]string, 0)
		for _, parameter := range operation.Parameters {
			if parameter.In != location || (!parameter.Required && parameter.Name != "status") {
				continue
			}
			value := fmt.Sprintf("%q", parameter.Name)
			switch {
			case len(parameter.Enum) > 0:
				value = fmt.Sprintf("%q", parameter.Enum[0])
			case parameter.Kind == sdkParamInteger:
				value = "1"
			case parameter.Kind == sdkParamBoolean:
				value = sdkSampleTrue
			}
			params = append(params, fmt.Sprintf("%q: %s", parameter.Name, value))
		}
		if len(params) > 0 {
			key := location
			if key == "header" {
				key = "headers"
			}
			fields = append(fields, key+": { "+strings.Join(params, ", ")+" }")
		}
	}
	if operation.RequestContentType == spec.RequestUploadForm {
		fields = append(fields, "body: { file: new Blob([\"content\"]) }")
	} else if operation.RequestBody != nil {
		fields = append(fields, "body: { /* request fields */ }")
	}
	if len(fields) == 0 {
		return "{}"
	}
	return "{ " + strings.Join(fields, ", ") + " }"
}

func sdkGoSample(operation spec.OperationSpec, kind, body string) string {
	if kind == sdkKindPrompt {
		return "stream, err := c.Prompt(ctx, client.PromptInput{WorkspaceID: " +
			"\"workspace-id\", SessionID: \"session-id\", Message: \"Hello\"})\n" +
			"if err != nil { return err }\ndefer stream.Close()\nfor {\n" +
			"  chunk, err := stream.Next()\n  if errors.Is(err, io.EOF) { break }\n" +
			"  if err != nil { return err }\n  fmt.Print(chunk.TextDelta())\n}"
	}
	method := sdkgo.ClientMethodName(operation.OperationID)
	arguments := []string{"ctx"}
	if len(operation.Parameters) > 0 {
		fields := make([]string, 0)
		for _, parameter := range operation.Parameters {
			if !parameter.Required {
				continue
			}
			value := fmt.Sprintf("%q", parameter.Name)
			switch {
			case len(parameter.Enum) > 0:
				value = fmt.Sprintf("%q", parameter.Enum[0])
			case parameter.Kind == sdkParamInteger:
				value = "1"
			case parameter.Kind == sdkParamBoolean:
				value = sdkSampleTrue
			}
			fields = append(fields, sdkgo.ClientParameterName(parameter.Name)+": "+value)
		}
		arguments = append(arguments, "client."+method+"Params{"+strings.Join(fields, ", ")+"}")
	}
	if body != "" {
		arguments = append(arguments, body)
	}
	if operation.OperationID == "listWorkspaces" {
		method = "ListWorkspacesWithParams"
	}
	call := fmt.Sprintf("c.%s(%s)", method, strings.Join(arguments, ", "))
	if kind == sdkKindSSE {
		return "stream, err := " + call + "\nif err != nil { return err }\ndefer stream.Close()\nfor {\n" +
			"  frame, err := stream.Next()\n  if errors.Is(err, io.EOF) { break }\n" +
			"  if err != nil { return err }\n  fmt.Println(frame.Data)\n}"
	}
	// Mirror the client generator: any success body gives the method a result,
	// even when a body-less success (such as 204) is declared first.
	success, result := false, false
	for _, response := range operation.Responses {
		if response.Status >= 200 && response.Status < 300 {
			success = true
			result = result || len(response.BodyTypes()) > 0
		}
	}
	if success && !result {
		return "err = " + call + "\nif err != nil { return err }"
	}
	return "result, err := " + call + "\nif err != nil { return err }\nfmt.Println(result)"
}

func sdkResourceName(operation spec.OperationSpec) string {
	if len(operation.Tags) == 0 {
		return "misc"
	}
	parts := strings.FieldsFunc(operation.Tags[0], func(r rune) bool { return r == '-' || r == '_' || r == ' ' })
	for index := 1; index < len(parts); index++ {
		parts[index] = strings.ToUpper(parts[index][:1]) + parts[index][1:]
	}
	return strings.Join(parts, "")
}

func writeSDKSamples(openapiPath string) error {
	content, err := generateSDKSamples()
	if err != nil {
		return err
	}
	return publishGeneratedFile(sdkSamplesPathFor(openapiPath), content)
}

func checkSDKSamples(openapiPath string) error {
	content, err := generateSDKSamples()
	if err != nil {
		return err
	}
	return checkSDKSampleManifest(sdkSamplesPathFor(openapiPath), content)
}

func checkSDKSampleMethods(root string) error {
	ts, err := os.ReadFile(filepath.Join(root, defaultSDKTypeScriptPath, sdkDirectory, "sdk.gen.ts"))
	if err != nil {
		return fmt.Errorf("read TypeScript SDK for sample validation: %w", err)
	}
	sse, err := os.ReadFile(filepath.Join(root, defaultSDKTypeScriptPath, sdkKindSSE, "sdk.gen.ts"))
	if err != nil {
		return fmt.Errorf("read stream SDK for sample validation: %w", err)
	}
	goFiles, err := filepath.Glob(filepath.Join(root, sdkDirectory, "go", sdkClientPackage, "*_gen.go"))
	if err != nil {
		return err
	}
	var goSource strings.Builder
	for _, path := range goFiles {
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		goSource.Write(data)
	}
	for _, operation := range spec.Operations() {
		kind := sdkOperationKind(operation)
		if kind == sdkKindPrompt || kind == sdkKindWebSocket {
			continue
		}
		resource := sdkResourceName(operation)
		_, class, found := strings.Cut(string(ts), "export class "+resource+" ")
		class, _, _ = strings.Cut(class, "\nexport class ")
		if kind == sdkJSONName && (!found || (!strings.Contains(class, operation.OperationID+"<") &&
			!strings.Contains(class, operation.OperationID+"("))) {
			return fmt.Errorf("SDK sample %s: TypeScript method is missing", operation.OperationID)
		}
		if kind == sdkKindSSE && !strings.Contains(string(sse), "export const "+operation.OperationID+" =") {
			return fmt.Errorf("SDK sample %s: TypeScript stream binding is missing", operation.OperationID)
		}
		if !strings.Contains(goSource.String(), "func (c *Client) "+sdkgo.ClientMethodName(operation.OperationID)+"(") {
			return fmt.Errorf("SDK sample %s: Go method is missing", operation.OperationID)
		}
	}
	return nil
}
