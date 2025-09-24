package worker

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.temporal.io/api/enums/v1"
	"go.temporal.io/api/serviceerror"
	"go.temporal.io/sdk/client"
	temporalmocks "go.temporal.io/sdk/mocks"

	"github.com/compozy/compozy/engine/project"
	appconfig "github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
)

func TestBuildDispatcherWorkflowID(t *testing.T) {
	t.Run("Should derive deterministic dispatcher ID from project and queue", func(t *testing.T) {
		id := buildDispatcherWorkflowID("Payments", "primary-queue")
		assert.Equal(t, "dispatcher-payments-primary-queue", id)
	})
	t.Run("Should truncate long queue names to configured maximum", func(t *testing.T) {
		longQueue := strings.Repeat("q", 400)
		id := buildDispatcherWorkflowID("", longQueue)
		assert.True(t, strings.HasPrefix(id, "dispatcher-"))
		assert.LessOrEqual(t, len(id), maxDispatcherWorkflowIDLength)
		assert.NotContains(t, id, "-segment-", "should not include placeholder segment when project name is empty")
	})
}

func TestWorkerEnsureDispatcherRunning(t *testing.T) {
	createContext := func(t *testing.T) context.Context {
		t.Helper()
		manager := appconfig.NewManager(appconfig.NewService())
		_, err := manager.Load(context.Background(), appconfig.NewDefaultProvider())
		require.NoError(t, err)
		cfg := manager.Get()
		cfg.Worker.DispatcherRetryDelay = 1 * time.Millisecond
		cfg.Worker.StartWorkflowTimeout = 50 * time.Millisecond
		cfg.Worker.DispatcherMaxRetries = 2
		ctx := appconfig.ContextWithManager(context.Background(), manager)
		log := logger.NewForTests()
		return logger.ContextWithLogger(ctx, log)
	}

	t.Run("Should start dispatcher with termination-first workflow reuse policy", func(t *testing.T) {
		ctx := createContext(t)
		mockClient := &temporalmocks.Client{}
		mockClient.Test(t)
		worker := &Worker{
			client:        &Client{Client: mockClient},
			dispatcherID:  buildDispatcherWorkflowID("payments", "payments-queue"),
			projectConfig: &project.Config{Name: "payments"},
			taskQueue:     "payments-queue",
			serverID:      "server-123",
		}
		startMatcher := mock.MatchedBy(func(opts client.StartWorkflowOptions) bool {
			return opts.ID == worker.dispatcherID && opts.TaskQueue == worker.taskQueue &&
				opts.WorkflowIDReusePolicy == enums.WORKFLOW_ID_REUSE_POLICY_TERMINATE_IF_RUNNING
		})
		mockClient.On(
			"SignalWithStartWorkflow",
			mock.Anything,
			worker.dispatcherID,
			DispatcherEventChannel,
			mock.Anything,
			startMatcher,
			mock.Anything,
			worker.projectConfig.Name,
			worker.serverID,
		).Return(nil, nil).Once()
		worker.ensureDispatcherRunning(ctx)
		mockClient.AssertExpectations(t)
	})

	t.Run("Should terminate stale dispatcher and retry startup", func(t *testing.T) {
		ctx := createContext(t)
		mockClient := &temporalmocks.Client{}
		mockClient.Test(t)
		worker := &Worker{
			client:        &Client{Client: mockClient},
			dispatcherID:  buildDispatcherWorkflowID("billing", "billing-queue"),
			projectConfig: &project.Config{Name: "billing"},
			taskQueue:     "billing-queue",
			serverID:      "server-456",
		}
		startMatcher := mock.MatchedBy(func(opts client.StartWorkflowOptions) bool {
			return opts.ID == worker.dispatcherID && opts.TaskQueue == worker.taskQueue &&
				opts.WorkflowIDReusePolicy == enums.WORKFLOW_ID_REUSE_POLICY_TERMINATE_IF_RUNNING
		})
		alreadyStarted := serviceerror.NewWorkflowExecutionAlreadyStarted(
			"dispatcher already running",
			worker.dispatcherID,
			"",
		)
		mockClient.On(
			"SignalWithStartWorkflow",
			mock.Anything,
			worker.dispatcherID,
			DispatcherEventChannel,
			mock.Anything,
			startMatcher,
			mock.Anything,
			worker.projectConfig.Name,
			worker.serverID,
		).Return(nil, alreadyStarted).Once()
		mockClient.On(
			"TerminateWorkflow",
			mock.Anything,
			worker.dispatcherID,
			"",
			dispatcherTakeoverReason,
			mock.Anything,
		).Return(nil).Once()
		mockClient.On(
			"SignalWithStartWorkflow",
			mock.Anything,
			worker.dispatcherID,
			DispatcherEventChannel,
			mock.Anything,
			startMatcher,
			mock.Anything,
			worker.projectConfig.Name,
			worker.serverID,
		).Return(nil, nil).Once()
		worker.ensureDispatcherRunning(ctx)
		mockClient.AssertExpectations(t)
	})
}
