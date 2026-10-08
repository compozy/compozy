import { act, renderHook, waitFor } from "@testing-library/react";
import { toast } from "sonner";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { AutomationApiError } from "../../adapters/automation-api";
import { automationEditorSeed, validateAutomationsSearch } from "../../lib/automation-route-search";
import { automationJobFixtures, automationTriggerFixtures } from "../../mocks/fixtures";

const createAutomationJobMock = vi.fn();
const updateAutomationJobMock = vi.fn();
const createAutomationTriggerMock = vi.fn();
const updateAutomationTriggerMock = vi.fn();

vi.mock("@/systems/agent/hooks/use-agents", () => ({
  useAgents: () => ({ data: [], error: null, isLoading: false }),
}));

vi.mock("../use-automation-actions", () => ({
  useCreateAutomationJob: () => ({ mutateAsync: createAutomationJobMock }),
  useUpdateAutomationJob: () => ({ mutateAsync: updateAutomationJobMock }),
  useCreateAutomationTrigger: () => ({ mutateAsync: createAutomationTriggerMock }),
  useUpdateAutomationTrigger: () => ({ mutateAsync: updateAutomationTriggerMock }),
}));

vi.mock("sonner", () => ({ toast: { error: vi.fn(), success: vi.fn() } }));

import { useAutomationEditor } from "../use-automation-editor";

function deferred<T>() {
  let resolve: (value: T) => void;
  const promise = new Promise<T>(resolvePromise => {
    resolve = resolvePromise;
  });
  return { promise, resolve: (value: T) => resolve(value) };
}

function fillReady(result: { current: ReturnType<typeof useAutomationEditor> }) {
  const editor = result.current.editorDialogProps.editor;
  if (!editor) throw new Error("editor is closed");
  act(() =>
    editor.onChange({
      ...editor.draft,
      name: "morning-digest",
      agent_name: "summarizer",
      prompt: "Summarize yesterday.",
    })
  );
}

describe("useAutomationEditor", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("clears the resource-bound editor when the active workspace changes", () => {
    const { result, rerender } = renderHook(
      ({ workspaceId }: { workspaceId: string | null }) =>
        useAutomationEditor({ activeWorkspaceId: workspaceId }),
      { initialProps: { workspaceId: "ws_alpha" } }
    );

    act(() => result.current.openCreate());
    expect(result.current.editor).toMatchObject({ mode: "create" });

    rerender({ workspaceId: "ws_beta" });

    expect(result.current.editor).toBeNull();
  });

  it("keeps a newly opened editor when an older submission resolves", async () => {
    const pending = deferred<{ id: string; name: string }>();
    createAutomationJobMock.mockReturnValue(pending.promise);
    const { result } = renderHook(() => useAutomationEditor({ activeWorkspaceId: "ws_alpha" }));

    act(() => result.current.openCreate());
    act(() => result.current.editorDialogProps.editor?.onSubmit());
    act(() => {
      result.current.close();
      result.current.openCreate();
    });

    await act(async () => {
      pending.resolve({ id: "job_old", name: "old" });
      await pending.promise;
    });

    await waitFor(() => expect(result.current.editor).toMatchObject({ mode: "create" }));
  });

  it("UT-092 saves a schedule as a job, toasts and hands the job to the detail route", async () => {
    const job = { ...automationJobFixtures[0], id: "job_1", name: "morning-digest" };
    createAutomationJobMock.mockResolvedValue(job);
    const onSaved = vi.fn();
    const { result } = renderHook(() =>
      useAutomationEditor({ activeWorkspaceId: "ws_alpha", onSaved })
    );

    act(() => result.current.openCreate());
    fillReady(result);
    act(() => result.current.editorDialogProps.editor?.onSubmit());

    await waitFor(() => expect(onSaved).toHaveBeenCalledWith({ entity: "job", automation: job }));
    expect(createAutomationJobMock).toHaveBeenCalledWith(
      expect.objectContaining({ name: "morning-digest", schedule: expect.any(Object) })
    );
    expect(createAutomationJobMock.mock.calls[0][0]).not.toHaveProperty("event");
    expect(createAutomationTriggerMock).not.toHaveBeenCalled();
    expect(toast.success).toHaveBeenCalledWith("Created morning-digest.");
    expect(result.current.editor).toBeNull();
  });

  it("UT-092 saves an event start as a trigger without job fields", async () => {
    const trigger = { ...automationTriggerFixtures[0], id: "trg_1", name: "morning-digest" };
    createAutomationTriggerMock.mockResolvedValue(trigger);
    const onSaved = vi.fn();
    const { result } = renderHook(() =>
      useAutomationEditor({ activeWorkspaceId: "ws_alpha", onSaved })
    );

    act(() => result.current.openCreate({ start: "event" }));
    fillReady(result);
    act(() => result.current.editorDialogProps.editor?.onSubmit());

    await waitFor(() =>
      expect(onSaved).toHaveBeenCalledWith({ entity: "trigger", automation: trigger })
    );
    const request = createAutomationTriggerMock.mock.calls[0][0];
    expect(request).toMatchObject({ event: "session.stopped", workspace_id: "ws_alpha" });
    expect(request).not.toHaveProperty("schedule");
    expect(request).not.toHaveProperty("task");
  });

  it("UT-093 keeps the draft and puts a name conflict on the Name field", async () => {
    createAutomationJobMock.mockRejectedValue(
      new AutomationApiError("automation: job name already exists in scope", 409)
    );
    const { result } = renderHook(() => useAutomationEditor({ activeWorkspaceId: "ws_alpha" }));

    act(() => result.current.openCreate());
    fillReady(result);
    act(() => result.current.editorDialogProps.editor?.onSubmit());

    await waitFor(() =>
      expect(result.current.editorDialogProps.editor).toMatchObject({
        submitError: "An automation named morning-digest already exists.",
        submitErrorField: "name",
      })
    );
    expect(result.current.editorDialogProps.editor?.draft.name).toBe("morning-digest");
  });

  it("UT-093 shows an edit conflict in the dialog without losing the draft", async () => {
    const job = { ...automationJobFixtures[0], profile_name: "default" };
    updateAutomationJobMock.mockRejectedValue(
      new AutomationApiError("automation: definition changed", 409)
    );
    const { result } = renderHook(() =>
      useAutomationEditor({ activeWorkspaceId: job.workspace_id })
    );

    act(() => result.current.openEdit(job));
    act(() => result.current.editorDialogProps.editor?.onSubmit());

    await waitFor(() =>
      expect(result.current.editorDialogProps.editor).toMatchObject({
        submitError: "automation: definition changed",
        submitErrorField: null,
      })
    );
    expect(result.current.editorDialogProps.editor?.draft.name).toBe(job.name);
  });

  it("preserves the job owner when saving an edit from an aggregate catalog", async () => {
    const job = { ...automationJobFixtures[0], profile_name: "marketing" };
    updateAutomationJobMock.mockResolvedValue(job);
    const { result } = renderHook(() =>
      useAutomationEditor({ activeWorkspaceId: job.workspace_id })
    );

    act(() => result.current.openEdit(job, { section: "options" }));
    expect(result.current.editor).toMatchObject({
      entity: "job",
      mode: "edit",
      section: "options",
    });
    act(() => result.current.editorDialogProps.editor?.onSubmit());

    await waitFor(() =>
      expect(updateAutomationJobMock).toHaveBeenCalledWith(
        expect.objectContaining({ id: job.id, profile: "marketing" })
      )
    );
  });

  it("preserves the trigger owner when saving an edit from an aggregate catalog", async () => {
    const trigger = { ...automationTriggerFixtures[0], profile_name: "support" };
    updateAutomationTriggerMock.mockResolvedValue(trigger);
    const { result } = renderHook(() =>
      useAutomationEditor({ activeWorkspaceId: trigger.workspace_id })
    );

    act(() => result.current.openEdit(trigger));
    expect(result.current.editor).toMatchObject({ entity: "trigger", draft: { start: "event" } });
    act(() => result.current.editorDialogProps.editor?.onSubmit());

    await waitFor(() =>
      expect(updateAutomationTriggerMock).toHaveBeenCalledWith(
        expect.objectContaining({ id: trigger.id, profile: "support" })
      )
    );
  });

  it("UT-106 opens deep links with Starts chosen and Does fixed to a seeded Loop", () => {
    expect(
      automationEditorSeed(validateAutomationsSearch({ create: "1", start: "event" }))
    ).toEqual({ key: "create:event", start: "event" });
    const loopSearch = validateAutomationsSearch({
      create: "loop",
      start: "schedule",
      loop: "software-delivery",
    });
    const loopSeed = automationEditorSeed(loopSearch);
    expect(loopSeed).toEqual({
      key: "loop:software-delivery:schedule",
      start: "schedule",
      loop: "software-delivery",
    });
    expect(validateAutomationsSearch({ create: "1", start: "webhook" }).start).toBe("webhook");
    expect(validateAutomationsSearch({ edit: "options" }).edit).toBe("options");
    expect(automationEditorSeed(validateAutomationsSearch({ start: "event" }))).toBeNull();

    const { result } = renderHook(() => useAutomationEditor({ activeWorkspaceId: "ws_alpha" }));

    act(() => result.current.openCreate({ start: "event" }));
    expect(result.current.editor?.draft.start).toBe("event");

    act(() => result.current.openCreate({ start: loopSeed?.start, loop: loopSeed?.loop }));
    expect(result.current.editor).toMatchObject({
      lockedLoop: "software-delivery",
      draft: {
        start: "schedule",
        target_kind: "loop",
        loop_target: expect.objectContaining({ loop_name: "software-delivery" }),
      },
    });
  });
});
