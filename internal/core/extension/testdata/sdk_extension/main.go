package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"

	extension "github.com/compozy/compozy/sdk/extension"
)

type record struct {
	Type    string         `json:"type"`
	Payload map[string]any `json:"payload,omitempty"`
}

type recorder struct {
	path string
	mu   sync.Mutex
}

func main() {
	rec := &recorder{path: strings.TrimSpace(os.Getenv("COMPOZY_SDK_RECORD_PATH"))}
	workflow := strings.TrimSpace(os.Getenv("COMPOZY_SDK_WORKFLOW"))
	if workflow == "" {
		workflow = "demo"
	}

	var ext *extension.Extension
	ext = extension.New("sdk-ext", "1.0.0").
		WithCapabilities(extension.CapabilityTasksRead).
		OnPromptPostBuild(func(
			ctx context.Context,
			hook extension.HookContext,
			payload extension.PromptPostBuildPayload,
		) (extension.PromptTextPatch, error) {
			tasks, err := hook.Host.Tasks.List(ctx, extension.TaskListRequest{Workflow: workflow})
			if err != nil {
				return extension.PromptTextPatch{}, err
			}

			request, _ := ext.InitializeRequest()
			rec.write("host_tasks_list", map[string]any{
				"workflow": workflow,
				"count":    len(tasks),
			})
			rec.write("execute_hook", map[string]any{
				"prompt_text": payload.PromptText,
				"task_count":  len(tasks),
				"extension":   request.Extension.Name,
			})
			return extension.PromptTextPatch{
				PromptText: extension.Ptr(payload.PromptText + "\npatched-by-sdk"),
			}, nil
		}).
		OnShutdown(func(_ context.Context, req extension.ShutdownRequest) error {
			rec.write("shutdown", map[string]any{"reason": req.Reason})
			return nil
		})

	if err := ext.Start(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func (r *recorder) write(kind string, payload map[string]any) {
	if r == nil || strings.TrimSpace(r.path) == "" {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	raw, err := json.Marshal(record{Type: strings.TrimSpace(kind), Payload: payload})
	if err != nil {
		return
	}

	file, err := os.OpenFile(r.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer file.Close()

	_, _ = file.Write(append(raw, '\n'))
}
