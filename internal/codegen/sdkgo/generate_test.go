package sdkgo

import (
	"bytes"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"slices"
	"strings"
	"testing"

	extensioncontract "github.com/compozy/compozy/internal/extension/contract"
	"github.com/compozy/compozy/sdk/go/contracts"
)

func TestGenerate(t *testing.T) {
	t.Parallel()

	t.Run("Should match contract counts and emit describe and conformance contracts", func(t *testing.T) {
		t.Parallel()

		result, err := Generate()
		if err != nil {
			t.Fatalf("Generate() error = %v", err)
		}
		second, err := Generate()
		if err != nil {
			t.Fatalf("Generate() second call error = %v", err)
		}
		if !slices.Equal(result.FileNames(), second.FileNames()) {
			t.Fatalf("Generate() file names changed: %v != %v", result.FileNames(), second.FileNames())
		}
		for _, name := range result.FileNames() {
			if !bytes.Equal(result.Files[name], second.Files[name]) {
				t.Fatalf("Generate() output %s changed between calls", name)
			}
		}
		hooks, err := extensioncontract.BuildHookContracts()
		if err != nil {
			t.Fatalf("BuildHookContracts() error = %v", err)
		}
		if got, want := result.HostMethodCount, len(extensioncontract.HostAPIMethodSpecs()); got != want {
			t.Fatalf("HostMethodCount = %d, want %d", got, want)
		}
		if got, want := result.HookEventCount, len(hooks); got != want {
			t.Fatalf("HookEventCount = %d, want %d", got, want)
		}

		var generated []byte
		for _, name := range result.FileNames() {
			generated = append(generated, result.Files[name]...)
		}
		normalizedGenerated := bytes.Join(bytes.Fields(generated), []byte(" "))
		for _, symbol := range [][]byte{
			[]byte("type DescribePayload struct"),
			[]byte("IssueSeverityError"),
			[]byte("IssueSeverityWarning"),
			[]byte("LoopProvenanceRoleCoordinator"),
			[]byte("LoopProvenanceRoleCell"),
			[]byte("type ProvideConformanceFixture struct"),
			[]byte("func PublicProvideConformanceFixtures()"),
		} {
			if !bytes.Contains(generated, symbol) {
				t.Fatalf("Generate() output missing %q", symbol)
			}
		}
		for _, field := range [][]byte{
			[]byte("Tools []ExtensionToolRuntimeDescriptor"),
			[]byte("CommandGroups []ExtensionCommandGroupSpec"),
		} {
			if !bytes.Contains(normalizedGenerated, field) {
				t.Fatalf("Generate() output missing field signature %q", field)
			}
		}
	})

	t.Run("Should expose generated required methods for model provides", func(t *testing.T) {
		t.Parallel()

		if got := contracts.RequiredMethods("model.source"); !slices.Contains(got, "models/list") {
			t.Fatalf("RequiredMethods(model.source) = %v, want models/list", got)
		}
		if got := contracts.RequiredMethods("unknown.provide"); got != nil {
			t.Fatalf("RequiredMethods(unknown.provide) = %v, want nil", got)
		}
	})
}

type CreateSessionRequest struct {
	Workspace string `json:"workspace"`
}

type SessionResponse struct {
	ID string `json:"id"`
}

type SessionStreamFrame struct {
	Sequence int64 `json:"sequence"`
}

type clientGenericFixture[T any] struct {
	Value T `json:"value"`
}

func TestGenerateClient(t *testing.T) {
	t.Parallel()

	t.Run("Should generate deterministic typed methods and encoded parameter bindings", func(t *testing.T) {
		t.Parallel()
		ops := clientFixtureOperations(t)
		first, err := GenerateClient(ops, ClientOptions{})
		if err != nil {
			t.Fatal(err)
		}
		slices.Reverse(ops)
		second, err := GenerateClient(ops, ClientOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(first.FileNames(), second.FileNames()) {
			t.Fatalf("generated file order differs: %v and %v", first.FileNames(), second.FileNames())
		}
		for name, content := range first.Files {
			if !bytes.Equal(content, second.Files[name]) {
				t.Fatalf("generated %s differs after input reordering", name)
			}
			if _, parseErr := parser.ParseFile(token.NewFileSet(), name, content, parser.AllErrors); parseErr != nil {
				t.Fatalf("generated %s is invalid Go: %v", name, parseErr)
			}
		}
		assertClientCompiles(t, first)
		golden, err := os.ReadFile("testdata/client/operations.golden")
		if err != nil {
			t.Fatalf("read client golden: %v\nGenerated:\n%s", err, first.Files["operations_001_gen.go"])
		}
		if !bytes.Equal(first.Files["operations_001_gen.go"], golden) {
			t.Fatalf("client operation contract changed:\n%s", first.Files["operations_001_gen.go"])
		}
	})

	t.Run("Should reject invalid operations with actionable diagnostics", func(t *testing.T) {
		t.Parallel()
		for _, tc := range []struct {
			name string
			op   ClientOperation
			want string
		}{
			{"Should reject reserved method names", ClientOperation{ID: "prompt", Kind: ClientOperationJSON, Method: "POST", Path: "/prompt"}, "prompt collides with reserved identifier Prompt"},
			{"Should reject generic DTOs", ClientOperation{ID: "generic", Kind: ClientOperationJSON, Method: "POST", Path: "/generic", RequestBody: clientGenericFixture[string]{}}, "operation generic: generic instantiation"},
			{"Should reject missing path parameters", ClientOperation{ID: "missing", Kind: ClientOperationJSON, Method: "GET", Path: "/{id}"}, "operation missing has undeclared path parameters"},
			{"Should reject colliding parameter fields", ClientOperation{ID: "collision", Kind: ClientOperationJSON, Method: "GET", Path: "/", Params: []ClientParam{{Name: "session_id", In: "query", GoType: "string"}, {Name: "session-id", In: "query", GoType: "string"}}}, "collide at SessionID"},
			{"Should reject unsupported parameter types", ClientOperation{ID: "badParam", Kind: ClientOperationJSON, Method: "GET", Path: "/", Params: []ClientParam{{Name: "n", In: "query", GoType: "float64"}}}, "unsupported Go type"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				_, err := GenerateClient([]ClientOperation{tc.op}, ClientOptions{})
				if err == nil || !strings.Contains(err.Error(), tc.want) {
					t.Fatalf("GenerateClient() error = %v, want %q", err, tc.want)
				}
			})
		}
	})

	t.Run("Should preserve every stream variant and support binary text and multipart operations", func(t *testing.T) {
		t.Parallel()
		result, err := GenerateClient([]ClientOperation{
			{
				ID:                  "readBytes",
				Method:              "GET",
				Path:                "/bytes",
				Kind:                ClientOperationJSON,
				ResponseContentType: "application/octet-stream",
			},
			{ID: "readHTML", Method: "GET", Path: "/html", Kind: ClientOperationJSON, ResponseContentType: "text/html"},
			{
				ID:                 "uploadSessionAttachment",
				Method:             "POST",
				Path:               "/files",
				Kind:               ClientOperationJSON,
				RequestContentType: "multipart/form-data",
				Response:           SessionResponse{},
			},
			{
				ID:               "streamCatalog",
				Method:           "GET",
				Path:             "/events",
				Kind:             ClientOperationSSE,
				ResponseVariants: []any{SessionResponse{}, SessionStreamFrame{}},
			},
		}, ClientOptions{})
		if err != nil {
			t.Fatal(err)
		}
		assertClientCompiles(t, result)
		var output []byte
		for _, name := range result.FileNames() {
			output = append(output, result.Files[name]...)
		}
		for _, want := range []string{
			"ReadBytes(ctx context.Context) (*[]byte, error)",
			"ReadHTML(ctx context.Context) (*string, error)",
			"UploadSessionAttachment(ctx context.Context, body *UploadFile) (*SessionResponse, error)",
			"StreamCatalog(ctx context.Context) (*Stream[StreamCatalogFrame], error)",
			"AsSessionResponse() (*SessionResponse, error)",
			"AsSessionStreamFrame() (*SessionStreamFrame, error)",
			"json.Unmarshal(data, &f.Raw)",
		} {
			if !bytes.Contains(output, []byte(want)) {
				t.Fatalf("client contract missing %q:\n%s", want, output)
			}
		}
	})
}

func clientFixtureOperations(t *testing.T) []ClientOperation {
	t.Helper()
	return []ClientOperation{
		{
			ID: "createSession", Method: "POST", Path: "/api/sessions", Kind: ClientOperationJSON,
			Summary: "creates a session.", RequestBody: CreateSessionRequest{}, Response: SessionResponse{},
			Params: []ClientParam{
				{Name: "profile", In: "query", GoType: "string"},
				{Name: "active", In: "query", GoType: "bool"},
				{Name: "limit", In: "query", GoType: "int"},
				{Name: "after_sequence", In: "query", GoType: "int64"},
				{Name: "include", In: "query", GoType: "[]string"},
				{Name: "X-Audit-ID", In: "header", GoType: "string"},
			},
		},
		{
			ID:       "streamSession",
			Method:   "GET",
			Path:     "/api/workspaces/{workspace_id}/sessions/{session_id}/stream",
			Kind:     ClientOperationSSE,
			Summary:  "streams a session.",
			Response: SessionStreamFrame{},
			Params: []ClientParam{
				{Name: "workspace_id", In: "path", GoType: "string", Required: true},
				{Name: "session_id", In: "path", GoType: "string", Required: true},
				{Name: "Last-Event-ID", In: "header", GoType: "string"},
				{Name: "last_event_id", In: "query", GoType: "string", GoName: "LastEventIDQuery"},
			},
		},
	}
}

func assertClientCompiles(t *testing.T, result Result) {
	t.Helper()
	set := token.NewFileSet()
	files := make([]*ast.File, 0, len(result.Files)+1)
	for _, name := range result.FileNames() {
		file, err := parser.ParseFile(set, name, result.Files[name], parser.AllErrors)
		if err != nil {
			t.Fatalf("parse generated %s: %v", name, err)
		}
		files = append(files, file)
	}
	const runtime = `package client
import ("context"; "net/http")
type Client struct{}
type Stream[T any] struct{}
type UploadFile struct{}
func (c *Client) doContent(context.Context, string, string, http.Header, any, any, string, string) error { return nil }
func newStream[T any](context.Context, *Client, string, string, http.Header, any, string) (*Stream[T], error) { return nil, nil }
`
	file, err := parser.ParseFile(set, "runtime.go", runtime, parser.AllErrors)
	if err != nil {
		t.Fatal(err)
	}
	files = append(files, file)
	config := types.Config{Importer: importer.Default()}
	if _, err := config.Check("client", set, files, nil); err != nil {
		t.Fatalf("generated client does not typecheck: %v", err)
	}
}
