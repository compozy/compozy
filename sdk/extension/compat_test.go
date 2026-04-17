package extension_test

import (
	"reflect"
	"testing"

	"github.com/compozy/compozy/internal/core/agent"
	extensions "github.com/compozy/compozy/internal/core/extension"
	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/prompt"
	extension "github.com/compozy/compozy/sdk/extension"
)

func TestPublicHookAndHostTypesStayAlignedWithRuntime(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		public  any
		runtime any
	}{
		{name: "IssueEntry", public: extension.IssueEntry{}, runtime: model.IssueEntry{}},
		{
			name:    "WorkflowMemoryContext",
			public:  extension.WorkflowMemoryContext{},
			runtime: prompt.WorkflowMemoryContext{},
		},
		{name: "BatchParams", public: extension.BatchParams{}, runtime: prompt.BatchParams{}},
		{name: "SessionRequest", public: extension.SessionRequest{}, runtime: agent.SessionRequest{}},
		{name: "ResumeSessionRequest", public: extension.ResumeSessionRequest{}, runtime: agent.ResumeSessionRequest{}},
		{name: "SessionIdentity", public: extension.SessionIdentity{}, runtime: agent.SessionIdentity{}},
		{name: "SessionOutcome", public: extension.SessionOutcome{}, runtime: agent.SessionOutcome{}},
		{name: "Job", public: extension.Job{}, runtime: model.Job{}},
		{name: "FetchConfig", public: extension.FetchConfig{}, runtime: model.FetchConfig{}},
		{name: "FixOutcome", public: extension.FixOutcome{}, runtime: model.FixOutcome{}},
		{name: "JobResult", public: extension.JobResult{}, runtime: model.JobResult{}},
		{name: "TaskRuntime", public: extension.TaskRuntime{}, runtime: model.TaskRuntime{}},
		{name: "TaskRuntimeTask", public: extension.TaskRuntimeTask{}, runtime: model.TaskRuntimeTask{}},
		{name: "RuntimeConfig", public: extension.RuntimeConfig{}, runtime: model.RuntimeConfig{}},
		{name: "RunArtifacts", public: extension.RunArtifacts{}, runtime: model.RunArtifacts{}},
		{name: "RunSummary", public: extension.RunSummary{}, runtime: model.RunSummary{}},
		{name: "TaskFrontmatter", public: extension.TaskFrontmatter{}, runtime: extensions.TaskFrontmatter{}},
		{name: "Task", public: extension.Task{}, runtime: extensions.Task{}},
		{name: "TaskCreateRequest", public: extension.TaskCreateRequest{}, runtime: extensions.TaskCreateRequest{}},
		{name: "RunConfig", public: extension.RunConfig{}, runtime: extensions.RunConfig{}},
		{name: "RunHandle", public: extension.RunHandle{}, runtime: extensions.RunHandle{}},
		{
			name:    "ArtifactReadRequest",
			public:  extension.ArtifactReadRequest{},
			runtime: extensions.ArtifactReadRequest{},
		},
		{name: "ArtifactReadResult", public: extension.ArtifactReadResult{}, runtime: extensions.ArtifactReadResult{}},
		{
			name:    "ArtifactWriteRequest",
			public:  extension.ArtifactWriteRequest{},
			runtime: extensions.ArtifactWriteRequest{},
		},
		{
			name:    "ArtifactWriteResult",
			public:  extension.ArtifactWriteResult{},
			runtime: extensions.ArtifactWriteResult{},
		},
		{
			name:    "PromptRenderRequest",
			public:  extension.PromptRenderRequest{},
			runtime: extensions.PromptRenderRequest{},
		},
		{name: "PromptRenderParams", public: extension.PromptRenderParams{}, runtime: extensions.PromptRenderParams{}},
		{name: "PromptIssueRef", public: extension.PromptIssueRef{}, runtime: extensions.PromptIssueRef{}},
		{name: "PromptRenderResult", public: extension.PromptRenderResult{}, runtime: extensions.PromptRenderResult{}},
		{name: "MemoryReadRequest", public: extension.MemoryReadRequest{}, runtime: extensions.MemoryReadRequest{}},
		{name: "MemoryReadResult", public: extension.MemoryReadResult{}, runtime: extensions.MemoryReadResult{}},
		{name: "MemoryWriteRequest", public: extension.MemoryWriteRequest{}, runtime: extensions.MemoryWriteRequest{}},
		{name: "MemoryWriteResult", public: extension.MemoryWriteResult{}, runtime: extensions.MemoryWriteResult{}},
		{
			name:    "EventSubscribeRequest",
			public:  extension.EventSubscribeRequest{},
			runtime: extensions.EventSubscribeRequest{},
		},
		{
			name:    "EventSubscribeResult",
			public:  extension.EventSubscribeResult{},
			runtime: extensions.EventSubscribeResult{},
		},
		{
			name:    "EventPublishRequest",
			public:  extension.EventPublishRequest{},
			runtime: extensions.EventPublishRequest{},
		},
		{name: "EventPublishResult", public: extension.EventPublishResult{}, runtime: extensions.EventPublishResult{}},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			publicFields := visibleFields(reflect.TypeOf(tc.public))
			runtimeFields := visibleFields(reflect.TypeOf(tc.runtime))
			if !reflect.DeepEqual(publicFields, runtimeFields) {
				t.Fatalf("field drift detected\npublic:  %#v\nruntime: %#v", publicFields, runtimeFields)
			}
		})
	}
}

func visibleFields(typ reflect.Type) []string {
	if typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}

	fields := make([]string, 0, typ.NumField())
	for idx := 0; idx < typ.NumField(); idx++ {
		field := typ.Field(idx)
		if field.PkgPath != "" {
			continue
		}
		tag := field.Tag.Get("json")
		if tag == "-" {
			continue
		}
		fields = append(fields, field.Name+"|"+tag+"|"+typeShape(field.Type))
	}
	return fields
}

func typeShape(typ reflect.Type) string {
	switch typ.Kind() {
	case reflect.Pointer:
		return "*" + typeShape(typ.Elem())
	case reflect.Slice:
		return "[]" + typeShape(typ.Elem())
	case reflect.Array:
		return "[" + typeShape(typ.Elem()) + "]"
	case reflect.Map:
		return "map[" + typeShape(typ.Key()) + "]" + typeShape(typ.Elem())
	default:
		return typ.Kind().String()
	}
}
