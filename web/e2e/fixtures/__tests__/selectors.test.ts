// @vitest-environment node

import { describe, expect, it, vi } from "vitest";
import type { Locator } from "@playwright/test";

import {
  automationOperatorSelectors,
  automationOperatorTestIds,
  profilesOperatorSelectors,
  profilesTestIds,
  sessionWindowSelectors,
  sessionWindowTestIds,
  tasksOperatorSelectors,
} from "../selectors";

describe("session window selectors", () => {
  it("scopes in-window controls and resolves overflow actions from the owning portal", () => {
    const getByTestId = vi.fn((testId: string) => `win-locator:${testId}` as unknown as Locator);
    const portalGetByTestId = vi.fn(
      (testId: string) => `portal-locator:${testId}` as unknown as Locator
    );
    const getByRole = vi.fn(
      (role: string, options?: { name: string }) =>
        `win-role:${role}:${options?.name}` as unknown as Locator
    );
    const selectors = sessionWindowSelectors(
      {
        getByRole,
        getByTestId,
      } as unknown as Locator,
      { getByTestId: portalGetByTestId }
    );

    expect(selectors.chatView).toBe(`win-locator:${sessionWindowTestIds.chatView}`);
    expect(selectors.composerClearButton).toBe(
      `portal-locator:${sessionWindowTestIds.composerClearButton}`
    );
    expect(selectors.composerSendButton).toBe("win-role:button:Send message");
    expect(selectors.composerStopButton).toBe(
      `win-locator:${sessionWindowTestIds.composerStopButton}`
    );
    expect(selectors.composerTextarea).toBe("win-role:textbox:Session prompt");
    expect(selectors.deleteButton).toBe(`portal-locator:${sessionWindowTestIds.deleteButton}`);
    expect(selectors.permissionAllowAlways).toBe(
      `win-locator:${sessionWindowTestIds.permissionAllowAlways}`
    );
    expect(selectors.permissionAllowOnce).toBe(
      `win-locator:${sessionWindowTestIds.permissionAllowOnce}`
    );
    expect(selectors.permissionPrompt).toBe(`win-locator:${sessionWindowTestIds.permissionPrompt}`);
    expect(selectors.processingIndicator).toBe(
      `win-locator:${sessionWindowTestIds.processingIndicator}`
    );
    expect(selectors.resumeButton).toBe(`win-locator:${sessionWindowTestIds.resumeButton}`);
    expect(selectors.stopButton).toBe(`win-locator:${sessionWindowTestIds.stopButton}`);
    expect(selectors.topbarOverflow).toBe(`win-locator:${sessionWindowTestIds.topbarOverflow}`);
  });
});

describe("automation operator selectors", () => {
  it("maps the automations navigation, editor, detail, and run-list surfaces to stable test IDs", () => {
    const getByLabel = vi.fn((label: string) => `label:${label}` as unknown as Locator);
    const getByRole = vi.fn(
      (role: string, options?: { name?: string | RegExp }) =>
        `role:${role}:${String(options?.name)}` as unknown as Locator
    );
    const editorDialog = {
      getByLabel: vi.fn((label: string) => `editor-label:${label}` as unknown as Locator),
      getByTestId: vi.fn((testId: string) => `editor-testid:${testId}` as unknown as Locator),
      getByRole: vi.fn(
        (role: string, options?: { name?: string | RegExp }) =>
          `editor-role:${role}:${String(options?.name)}` as unknown as Locator
      ),
    } as unknown as Locator;
    const getByTestId = vi.fn((testId: string) =>
      testId === automationOperatorTestIds.automationEditorDialog
        ? editorDialog
        : (`locator:${testId}` as unknown as Locator)
    );
    const portalGetByTestId = vi.fn(
      (testId: string) => `portal-locator:${testId}` as unknown as Locator
    );
    const selectors = automationOperatorSelectors(
      {
        getByLabel,
        getByRole,
        getByTestId,
      },
      { getByTestId: portalGetByTestId }
    );

    expect(selectors.automationsShell).toBe(
      `locator:${automationOperatorTestIds.automationsShell}`
    );
    expect(selectors.automationSuggestionsCard).toBe(
      `locator:${automationOperatorTestIds.automationSuggestionsCard}`
    );
    expect(selectors.automationsListRows).toBe(
      `locator:${automationOperatorTestIds.automationsListRows}`
    );
    expect(selectors.automationsCreate).toBe(
      `locator:${automationOperatorTestIds.automationsCreate}`
    );
    expect(selectors.automationStartView("event")).toBe("locator:automation-start-event");
    expect(selectors.automationSwitch("morning-digest")).toBe(
      "locator:automation-switch-morning-digest"
    );
    expect(selectors.suggestion("suggestion-1")).toBe("locator:automation-suggestion-suggestion-1");
    expect(selectors.automationDeleteDialog).toBe(
      `locator:${automationOperatorTestIds.automationDeleteDialog}`
    );
    expect(selectors.automationDeleteConfirmTyping).toBe(
      `locator:${automationOperatorTestIds.automationDeleteConfirmTyping}`
    );
    expect(selectors.confirmDeleteAutomationButton).toBe(
      `locator:${automationOperatorTestIds.confirmDeleteAutomationButton}`
    );
    expect(selectors.detailPanel).toBe(
      `locator:${automationOperatorTestIds.automationDetailPanel}`
    );
    expect(selectors.editAutomationButton).toBe(
      `portal-locator:${automationOperatorTestIds.editAutomationButton}`
    );
    expect(selectors.deleteAutomationButton).toBe(
      `portal-locator:${automationOperatorTestIds.deleteAutomationButton}`
    );
    expect(selectors.form).toBe("locator:automation-form");
    expect(selectors.nameInput).toBe("locator:automation-name-input");
    expect(selectors.scheduleExpr).toBe("editor-label:Cron expression");
    expect(selectors.scheduleExpressionToggle).toBe("editor-role:button:Edit expression");
    expect(selectors.optionsToggle).toBe("locator:automation-options-toggle");
    expect(selectors.formSubmit).toBe("locator:automation-form-submit");
    expect(selectors.previewToggle).toBe("locator:automation-preview-toggle");
    expect(selectors.startChoice("webhook")).toBe("editor-testid:automation-start-webhook");
    expect(selectors.doesChoice("loop")).toBe("editor-testid:automation-does-loop");
    expect(selectors.runList).toBe("locator:automation-run-list");
    expect(selectors.detailRunNow).toBe("locator:automation-run-now-btn");
    expect(selectors.enableSwitch).toBe("locator:automation-enable-switch");
    expect(selectors.inspectSheet).toBe("portal-locator:automation-inspect-sheet");
    expect(selectors.eventOption("session.stopped")).toBe(
      "locator:automation-event-session.stopped"
    );
    expect(selectors.conditionAdd).toBe("locator:automation-condition-add");
    expect(selectors.conditionField(0)).toBe("locator:automation-condition-field-0");
    expect(selectors.conditionValue(0)).toBe("locator:automation-condition-value-0");
    expect(selectors.item("job_daily_review")).toBe(
      "locator:/^automation-row-(job|trigger)-job_daily_review$/"
    );
    expect(selectors.run("run_001")).toBe("locator:automation-run-run_001");
    expect(selectors.runDrawer("run_001")).toBe("locator:automation-run-drawer-run_001");
    expect(selectors.runOpenLink("run_001")).toBe("locator:automation-run-open-run_001");
    expect(selectors.runRetries("run_001")).toBe("locator:automation-run-retries-run_001");
  });
});

describe("tasks operator selectors", () => {
  it("scopes the task breadcrumb to the window path navigation", () => {
    const getByTestId = vi.fn((testId: string) => `locator:${testId}` as unknown as Locator);
    const getByRoleWithinWindowPath = vi.fn(
      (role: string, options?: { name: string }) =>
        `breadcrumb-role:${role}:${options?.name}` as unknown as Locator
    );
    const getByRole = vi.fn((role: string, options?: { name: string }) =>
      role === "navigation" && options?.name === "Window path"
        ? ({ getByRole: getByRoleWithinWindowPath } as unknown as Locator)
        : (`role:${role}:${options?.name}` as unknown as Locator)
    );
    const selectors = tasksOperatorSelectors({
      getByRole,
      getByTestId,
    });

    expect(selectors.detailBreadcrumbTasks).toBe("breadcrumb-role:button:Tasks");
  });
});

describe("profiles operator selectors", () => {
  it("scopes the settings surface to its owning window while chrome stays page-level", () => {
    const pageGetByTestId = vi.fn((testId: string) => `page:${testId}` as unknown as Locator);
    const scopeGetByTestId = vi.fn((testId: string) => `window:${testId}` as unknown as Locator);
    const selectors = profilesOperatorSelectors(
      { getByTestId: pageGetByTestId } as never,
      { getByTestId: scopeGetByTestId } as never
    );

    // A page-level match would resolve the settings surface of a second window.
    expect(selectors.page).toBe(`window:${profilesTestIds.page}`);
    expect(selectors.activeList).toBe(`window:${profilesTestIds.activeList}`);
    expect(selectors.switcher).toBe(`page:${profilesTestIds.switcher}`);
  });
});
