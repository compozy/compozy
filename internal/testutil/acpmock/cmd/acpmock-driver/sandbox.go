package main

import (
	"context"

	"errors"

	"fmt"
	"os"
	"strings"

	"time"

	acpsdk "github.com/coder/acp-go-sdk"

	"github.com/compozy/agh/internal/testutil/acpmock"
)

func (a *mockAgent) executeSandboxCommand(
	ctx context.Context,
	sessionID acpsdk.SessionId,
	step acpmock.Step,
) (acpmock.DiagnosticsStep, error) {
	toolCallID, title := sandboxDescriptor(step)
	if err := a.startSandboxToolCall(ctx, sessionID, step, toolCallID, title); err != nil {
		return acpmock.DiagnosticsStep{}, err
	}

	result := a.runSandboxCommand(ctx, sessionID, step)
	if expected := strings.TrimSpace(step.ExpectErrorContains); expected != "" {
		return a.finishSandboxFailure(ctx, sessionID, step, toolCallID, title, result, expected)
	}
	if err := validateSandboxResult(step, result); err != nil {
		return acpmock.DiagnosticsStep{}, err
	}
	if err := a.finishSandboxSuccess(ctx, sessionID, step, toolCallID, title, result); err != nil {
		return acpmock.DiagnosticsStep{}, err
	}

	return acpmock.DiagnosticsStep{
		Kind:       acpmock.StepKindSandbox,
		ToolCallID: toolCallID,
		Command:    strings.TrimSpace(step.Command),
		Args:       append([]string(nil), step.Args...),
		ExitCode:   result.ExitCode,
		Output:     result.Output,
	}, nil
}

func (a *mockAgent) executeDriverControl(
	ctx context.Context,
	step acpmock.Step,
) (acpmock.DiagnosticsStep, error) {
	if step.DriverControl == nil {
		return acpmock.DiagnosticsStep{}, errors.New("driver_control payload is required")
	}

	diagnostics := acpmock.DiagnosticsStep{
		Kind:         acpmock.StepKindDriverControl,
		DriverAction: step.DriverControl.Action,
		Text:         strings.TrimSpace(step.DriverControl.RawJSONRPC),
	}
	control := *step.DriverControl
	if control.Async {
		lifecycleCtx := a.lifecycleContext()
		a.asyncWG.Add(1)
		go func(promptCtx context.Context, lifetimeCtx context.Context, control acpmock.DriverControlStep) {
			defer a.asyncWG.Done()

			if err := waitDriverControlDelay(promptCtx, lifetimeCtx, control.DelayMS); err != nil {
				return
			}
			if err := a.performDriverControl(promptCtx, control); err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "acpmock async driver_control %s error: %v\n", control.Action, err)
			}
		}(ctx, lifecycleCtx, control)
		return diagnostics, nil
	}
	if err := waitDriverControlDelay(ctx, a.lifecycleContext(), control.DelayMS); err != nil {
		return acpmock.DiagnosticsStep{}, err
	}
	return diagnostics, a.performDriverControl(ctx, control)
}

func waitDriverControlDelay(promptCtx context.Context, lifetimeCtx context.Context, delayMS int) error {
	if err := driverControlContextErr(promptCtx, lifetimeCtx); err != nil {
		return err
	}
	if delayMS <= 0 {
		return nil
	}
	timer := time.NewTimer(time.Duration(delayMS) * time.Millisecond)
	defer timer.Stop()

	select {
	case <-timer.C:
		return driverControlContextErr(promptCtx, lifetimeCtx)
	case <-contextDone(promptCtx):
		return promptCtx.Err()
	case <-contextDone(lifetimeCtx):
		return lifetimeCtx.Err()
	}
}

func (a *mockAgent) performDriverControl(ctx context.Context, control acpmock.DriverControlStep) error {
	switch control.Action {
	case acpmock.DriverControlDisconnect:
		os.Exit(23)
		return nil
	case acpmock.DriverControlWriteRawJSONRPC:
		frame := control.RawJSONRPC
		if !strings.HasSuffix(frame, "\n") {
			frame += "\n"
		}
		_, err := os.Stdout.WriteString(frame)
		return err
	case acpmock.DriverControlBlockUntilCancel:
		<-ctx.Done()
		return ctx.Err()
	case acpmock.DriverControlDelay:
		return nil
	default:
		return fmt.Errorf("unsupported driver_control action %s", control.Action)
	}
}

func sandboxDescriptor(step acpmock.Step) (string, string) {
	toolCallID := strings.TrimSpace(step.ToolCallID)
	title := strings.TrimSpace(step.Title)
	if title == "" {
		title = strings.TrimSpace(step.Command)
	}
	if title == "" {
		title = "sandbox command"
	}
	return toolCallID, title
}

func (a *mockAgent) startSandboxToolCall(
	ctx context.Context,
	sessionID acpsdk.SessionId,
	step acpmock.Step,
	toolCallID string,
	title string,
) error {
	if toolCallID == "" {
		return nil
	}

	if err := a.conn.SessionUpdate(ctx, acpsdk.SessionNotification{
		SessionId: sessionID,
		Update: acpsdk.StartToolCall(
			acpsdk.ToolCallId(toolCallID),
			title,
			acpsdk.WithStartKind(toolKind(step.ToolKind, acpsdk.ToolKindExecute)),
			acpsdk.WithStartStatus(acpsdk.ToolCallStatusInProgress),
			acpsdk.WithStartRawInput(map[string]any{
				"command": strings.TrimSpace(step.Command),
				"args":    append([]string(nil), step.Args...),
			}),
		),
	}); err != nil {
		return err
	}
	return pauseForDelivery(ctx)
}

func (a *mockAgent) runSandboxCommand(
	ctx context.Context,
	sessionID acpsdk.SessionId,
	step acpmock.Step,
) sandboxRunResult {
	req := acpsdk.CreateTerminalRequest{
		SessionId: sessionID,
		Command:   strings.TrimSpace(step.Command),
		Args:      append([]string(nil), step.Args...),
	}
	if cwd := strings.TrimSpace(step.Cwd); cwd != "" {
		req.Cwd = new(cwd)
	}

	createResp, err := a.conn.CreateTerminal(ctx, req)
	if err != nil {
		return sandboxRunResult{ObservedError: err.Error()}
	}

	result := sandboxRunResult{}
	waitResp, waitErr := a.conn.WaitForTerminalExit(ctx, acpsdk.WaitForTerminalExitRequest{
		SessionId:  sessionID,
		TerminalId: createResp.TerminalId,
	})
	if waitErr != nil {
		result.observeError(waitErr)
	} else {
		result.ExitCode = waitResp.ExitCode
		outputResp, outputErr := a.conn.TerminalOutput(ctx, acpsdk.TerminalOutputRequest{
			SessionId:  sessionID,
			TerminalId: createResp.TerminalId,
		})
		if outputErr != nil {
			result.observeError(outputErr)
		} else {
			result.Output = outputResp.Output
		}
	}

	cleanupCtx, cancelCleanup := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancelCleanup()
	_, releaseErr := a.conn.ReleaseTerminal(cleanupCtx, acpsdk.ReleaseTerminalRequest{
		SessionId:  sessionID,
		TerminalId: createResp.TerminalId,
	})
	if releaseErr != nil {
		result.observeError(fmt.Errorf("release terminal: %w", releaseErr))
	}
	return result
}

func (r *sandboxRunResult) observeError(err error) {
	if err == nil {
		return
	}
	if r.ObservedError == "" {
		r.ObservedError = err.Error()
		return
	}
	r.ObservedError += "; " + err.Error()
}

func (a *mockAgent) finishSandboxFailure(
	ctx context.Context,
	sessionID acpsdk.SessionId,
	step acpmock.Step,
	toolCallID string,
	title string,
	result sandboxRunResult,
	expected string,
) (acpmock.DiagnosticsStep, error) {
	if result.ObservedError == "" || !strings.Contains(result.ObservedError, expected) {
		return acpmock.DiagnosticsStep{}, fmt.Errorf(
			"sandbox command error %s did not include %s",
			result.ObservedError,
			expected,
		)
	}
	if err := a.emitSandboxFailure(ctx, sessionID, step, toolCallID, title, result.ObservedError); err != nil {
		return acpmock.DiagnosticsStep{}, err
	}
	return acpmock.DiagnosticsStep{
		Kind:       acpmock.StepKindSandbox,
		ToolCallID: toolCallID,
		Command:    strings.TrimSpace(step.Command),
		Args:       append([]string(nil), step.Args...),
		Error:      result.ObservedError,
	}, nil
}

func validateSandboxResult(step acpmock.Step, result sandboxRunResult) error {
	if result.ObservedError != "" {
		return errors.New(result.ObservedError)
	}
	if step.ExpectExitCode != nil {
		if result.ExitCode == nil || *result.ExitCode != *step.ExpectExitCode {
			got := "<nil>"
			if result.ExitCode != nil {
				got = fmt.Sprintf("%d", *result.ExitCode)
			}
			return fmt.Errorf(
				"sandbox exit code %s did not match expected %d",
				got,
				*step.ExpectExitCode,
			)
		}
	}
	if expected := strings.TrimSpace(step.ExpectOutputContains); expected != "" &&
		!strings.Contains(result.Output, expected) {
		return fmt.Errorf("sandbox output %q did not include %s", result.Output, expected)
	}
	return nil
}

func (a *mockAgent) finishSandboxSuccess(
	ctx context.Context,
	sessionID acpsdk.SessionId,
	step acpmock.Step,
	toolCallID string,
	title string,
	result sandboxRunResult,
) error {
	if toolCallID != "" {
		updateOpts := []acpsdk.ToolCallUpdateOpt{
			acpsdk.WithUpdateStatus(toolStatus(step.Status, acpsdk.ToolCallStatusCompleted)),
			acpsdk.WithUpdateTitle(title),
		}
		if result.Output != "" {
			updateOpts = append(updateOpts, acpsdk.WithUpdateContent(textToolContent(result.Output)))
		}
		if err := a.conn.SessionUpdate(ctx, acpsdk.SessionNotification{
			SessionId: sessionID,
			Update:    acpsdk.UpdateToolCall(acpsdk.ToolCallId(toolCallID), updateOpts...),
		}); err != nil {
			return err
		}
		if err := pauseForDelivery(ctx); err != nil {
			return err
		}
	}

	switch {
	case step.EmitOutput:
		if err := a.conn.SessionUpdate(ctx, acpsdk.SessionNotification{
			SessionId: sessionID,
			Update:    acpsdk.UpdateAgentMessageText(result.Output),
		}); err != nil {
			return err
		}
		return pauseForDelivery(ctx)
	case strings.TrimSpace(step.EmitText) != "":
		if err := a.conn.SessionUpdate(ctx, acpsdk.SessionNotification{
			SessionId: sessionID,
			Update:    acpsdk.UpdateAgentMessageText(strings.TrimSpace(step.EmitText)),
		}); err != nil {
			return err
		}
		return pauseForDelivery(ctx)
	default:
		return nil
	}
}
