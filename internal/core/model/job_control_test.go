package model

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestJobControlRegistryRoutesByJobIDAndIndexAlias(t *testing.T) {
	t.Parallel()

	t.Run("Should route by job ID and index alias", func(t *testing.T) {
		t.Parallel()

		registry := NewJobControlRegistry()
		controller := &recordingJobController{}
		unregister := registry.Register(" run-1 ", 2, " task_02-safe ", controller)
		t.Cleanup(unregister)

		pauseResp, err := registry.Pause(context.Background(), "run-1", "task_02-safe")
		if err != nil {
			t.Fatalf("Pause(job id) error = %v", err)
		}
		if pauseResp.RunID != "run-1" || pauseResp.JobID != "task_02-safe" ||
			pauseResp.Index != 2 || pauseResp.Status != JobControlStatusPausing {
			t.Fatalf("Pause(job id) = %#v, want completed response", pauseResp)
		}

		messageResp, err := registry.SendMessage(context.Background(), "run-1", "job-002", "  continue here  ")
		if err != nil {
			t.Fatalf("SendMessage(index alias) error = %v", err)
		}
		if messageResp.RunID != "run-1" || messageResp.JobID != "job-002" ||
			messageResp.Index != 2 || messageResp.Status != JobControlStatusResumed {
			t.Fatalf("SendMessage(index alias) = %#v, want completed response", messageResp)
		}
		if controller.lastMessage != "continue here" {
			t.Fatalf("controller message = %q, want trimmed message", controller.lastMessage)
		}

		unregister()
		if _, err := registry.Pause(
			context.Background(),
			"run-1",
			"task_02-safe",
		); !errors.Is(
			err,
			ErrJobControlNotFound,
		) {
			t.Fatalf("Pause(after unregister) error = %v, want ErrJobControlNotFound", err)
		}
	})

	t.Run("Should normalize controller response identity from request", func(t *testing.T) {
		t.Parallel()

		registry := NewJobControlRegistry()
		controller := &recordingJobController{
			pauseResp: JobControlResponse{
				RunID:  "wrong-run",
				JobID:  "wrong-job",
				Index:  99,
				Status: JobControlStatusPausing,
			},
		}
		unregister := registry.Register("run-1", 2, "task_02-safe", controller)
		t.Cleanup(unregister)

		resp, err := registry.Pause(context.Background(), "run-1", "task_02-safe")
		if err != nil {
			t.Fatalf("Pause() error = %v", err)
		}
		if resp.RunID != "run-1" || resp.JobID != "task_02-safe" || resp.Index != 2 {
			t.Fatalf("normalized response = %#v, want request identity/index", resp)
		}
	})
}

func TestValidateJobControlMessageRejectsEmptyAndOversizedMessages(t *testing.T) {
	t.Parallel()

	t.Run("Should reject empty and oversized messages", func(t *testing.T) {
		t.Parallel()

		if _, err := ValidateJobControlMessage(" \n\t "); !errors.Is(err, ErrJobControlMessageRequired) {
			t.Fatalf("empty message error = %v, want ErrJobControlMessageRequired", err)
		}
		oversized := strings.Repeat("x", MaxJobControlMessageBytes+1)
		if _, err := ValidateJobControlMessage(oversized); !errors.Is(err, ErrJobControlMessageTooLarge) {
			t.Fatalf("oversized message error = %v, want ErrJobControlMessageTooLarge", err)
		}
	})

	t.Run("Should trim valid messages", func(t *testing.T) {
		t.Parallel()

		trimmed, err := ValidateJobControlMessage("  ok  ")
		if err != nil {
			t.Fatalf("valid message error = %v", err)
		}
		if trimmed != "ok" {
			t.Fatalf("trimmed message = %q, want ok", trimmed)
		}
	})
}

type recordingJobController struct {
	lastMessage string
	pauseResp   JobControlResponse
	sendResp    JobControlResponse
}

func (c *recordingJobController) Pause(
	context.Context,
	JobControlRequest,
) (JobControlResponse, error) {
	if c.pauseResp.Status != "" {
		return c.pauseResp, nil
	}
	return JobControlResponse{Status: JobControlStatusPausing}, nil
}

func (c *recordingJobController) SendMessage(
	_ context.Context,
	req JobControlRequest,
) (JobControlResponse, error) {
	c.lastMessage = req.Message
	if c.sendResp.Status != "" {
		return c.sendResp, nil
	}
	return JobControlResponse{Status: JobControlStatusResumed}, nil
}
