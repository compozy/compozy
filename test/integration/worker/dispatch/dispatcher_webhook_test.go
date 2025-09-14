package dispatch

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/testsuite"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/engine/webhook"
	"github.com/compozy/compozy/engine/worker"
	wf "github.com/compozy/compozy/engine/workflow"
	wfacts "github.com/compozy/compozy/engine/workflow/activities"
	"github.com/compozy/compozy/pkg/config"
)

func TestDispatcherWorkflow_WebhookEventDispatch(t *testing.T) {
	t.Run("Should dispatch based on webhook event name", func(t *testing.T) {
		testSuite := &testsuite.WorkflowTestSuite{}
		env := testSuite.NewTestWorkflowEnvironment()
		defer func() { env.AssertExpectations(t) }()
		eventSchema := &schema.Schema{
			"type":       "object",
			"properties": map[string]any{"body": map[string]any{"type": "string"}},
			"required":   []string{"body"},
		}
		mockWorkflows := []*wf.Config{{
			ID: "github_issue_comment_analyzer",
			Triggers: []wf.Trigger{{
				Type: wf.TriggerTypeWebhook,
				Webhook: &webhook.Config{
					Slug: "github-issues",
					Events: []webhook.EventConfig{{
						Name:   "issue.commented",
						Filter: "true",
						Input:  map[string]string{"body": "{{ .payload.comment.body }}"},
						Schema: eventSchema,
					}},
				},
			}},
		}}
		mockProjectConfig := &project.Config{Name: "github"}
		getData := &wfacts.GetData{ProjectConfig: mockProjectConfig, Workflows: mockWorkflows}
		mockAppConfig := &config.Config{
			Runtime: config.RuntimeConfig{
				DispatcherHeartbeatInterval: 30000000000,
				DispatcherHeartbeatTTL:      90000000000,
			},
		}
		env.RegisterActivityWithOptions(getData.Run, activity.RegisterOptions{Name: wfacts.GetDataLabel})
		env.OnActivity(wfacts.GetDataLabel, mock.Anything, mock.Anything).
			Return(&wfacts.GetData{ProjectConfig: mockProjectConfig, Workflows: mockWorkflows, AppConfig: mockAppConfig}, nil)
		env.OnWorkflow("CompozyWorkflow", mock.Anything, mock.Anything).Return(nil, nil).Once()
		env.RegisterWorkflow(worker.DispatcherWorkflow)
		env.RegisterWorkflow(worker.CompozyWorkflow)
		done := make(chan struct{})
		go func() { defer close(done); env.ExecuteWorkflow(worker.DispatcherWorkflow, "github", "test-server") }()
		env.RegisterDelayedCallback(func() {
			env.SignalWorkflow(
				worker.DispatcherEventChannel,
				worker.EventSignal{
					Name:          "issue.commented",
					Payload:       core.Input{"body": "This feature is really nice"},
					CorrelationID: "test-corr",
				},
			)
		}, 10*time.Millisecond)
		env.RegisterDelayedCallback(func() { env.CancelWorkflow() }, 120*time.Millisecond)
		<-done
		workflowErr := env.GetWorkflowError()
		require.Error(t, workflowErr)
		msg := workflowErr.Error()
		assert.True(
			t,
			msg == "canceled" || strings.Contains(msg, "deadline exceeded") || strings.Contains(msg, "timeout"),
		)
		env.AssertExpectations(t)
	})
}
