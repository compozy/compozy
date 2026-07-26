import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { AgentDigestConflictError } from "../../adapters/agent-api";
import {
  buildAuthoredFileResourceKey,
  isAuthoredFileMissing,
} from "../../hooks/use-agent-authored-file-editor";
import type { AgentSoulHistoryResponse, AgentSoulPayload } from "../../types";
import { AgentAuthoredFileEditor } from "../agent-authored-file-editor";

vi.mock("../../hooks/use-unsaved-guard", () => ({
  useUnsavedGuard: () => ({ confirmDialog: null }),
}));

function soulPayload(overrides: Partial<AgentSoulPayload> = {}): AgentSoulPayload {
  return {
    active: true,
    present: true,
    enabled: true,
    valid: true,
    validation_status: "valid",
    digest: "soul-digest",
    source_path: "agents/coder/SOUL.md",
    frontmatter: {
      version: "1",
      role: "coder",
      tone: ["concise"],
      principles: ["Keep scope tight"],
    },
    body: "# Soul\n\nShip carefully.",
    limits: { max_body_bytes: 65536 },
    config_provenance: {
      digest: "config-digest",
      enabled: true,
      max_body_bytes: 65536,
      context_projection_bytes: 0,
    },
    ...overrides,
  };
}

const history = {
  revisions: [
    {
      id: "rev-1",
      action: "put",
      actor: { kind: "user" },
      agent_name: "coder",
      created_at: "2026-07-11T10:00:00Z",
      source_path: "agents/coder/SOUL.md",
    },
  ],
} as AgentSoulHistoryResponse;

const resourceKeyA = buildAuthoredFileResourceKey("ws-test", "coder", "soul");
const resourceKeyB = buildAuthoredFileResourceKey("ws-test", "reviewer", "soul");

describe("AgentAuthoredFileEditor", () => {
  it("Should treat undefined transport state as not missing", () => {
    expect(isAuthoredFileMissing(undefined)).toBe(false);
    expect(
      isAuthoredFileMissing(soulPayload({ present: false, validation_status: "missing" }))
    ).toBe(true);
  });

  it("Should edit the complete source, validate diagnostics, save with CAS, and restore history", async () => {
    const user = userEvent.setup();
    const onValidate = vi.fn().mockResolvedValue({
      validation_status: "invalid",
      diagnostics: [{ message: "Role is required", line: 2 }],
    });
    const saved = soulPayload({ digest: "soul-digest-saved", body: "saved body" });
    const restored = soulPayload({ digest: "soul-digest-restored", body: "restored body" });
    const onSave = vi.fn().mockResolvedValue(saved);
    const onRestore = vi.fn().mockResolvedValue(restored);
    render(
      <AgentAuthoredFileEditor
        resourceKey={resourceKeyA}
        kind="soul"
        payload={soulPayload()}
        isLoading={false}
        isError={false}
        history={history}
        onValidate={onValidate}
        onSave={onSave}
        onRestore={onRestore}
        onRetry={vi.fn()}
      />
    );

    const editor = screen.getByTestId("agent-soul-textarea");
    expect((editor as HTMLTextAreaElement).value).toContain('role: "coder"');
    await user.type(editor, "\nNew constraint.");
    await user.click(screen.getByTestId("agent-soul-validate"));
    expect(onValidate).toHaveBeenCalledWith(expect.stringContaining("New constraint."));
    expect(await screen.findByText("Role is required")).toBeVisible();
    expect(screen.getByText("invalid")).toBeVisible();

    await user.click(screen.getByTestId("agent-soul-save"));
    expect(onSave).toHaveBeenCalledWith(
      expect.stringContaining('principles:\n  - "Keep scope tight"'),
      "soul-digest"
    );

    await user.click(screen.getByTestId("agent-soul-history-toggle"));
    await user.click(screen.getByTestId("agent-soul-restore-rev-1"));
    expect(onRestore).toHaveBeenCalledWith("rev-1", "soul-digest-saved");
  });

  it("Should keep CAS digest d1 when a dirty draft sees a remote digest d2 rerender", async () => {
    const user = userEvent.setup();
    const onSave = vi.fn().mockResolvedValue(soulPayload({ digest: "d-after-save" }));
    const view = render(
      <AgentAuthoredFileEditor
        resourceKey={resourceKeyA}
        kind="soul"
        payload={soulPayload({ digest: "d1" })}
        isLoading={false}
        isError={false}
        history={history}
        onValidate={vi.fn().mockResolvedValue({})}
        onSave={onSave}
        onRestore={vi.fn().mockResolvedValue(soulPayload())}
        onRetry={vi.fn()}
      />
    );

    const editor = screen.getByTestId("agent-soul-textarea");
    await user.type(editor, "\nlocal-only");
    const dirtyDraft = (screen.getByTestId("agent-soul-textarea") as HTMLTextAreaElement).value;

    view.rerender(
      <AgentAuthoredFileEditor
        resourceKey={resourceKeyA}
        kind="soul"
        payload={soulPayload({ digest: "d2", body: "# Soul\n\nRemote rewrite." })}
        isLoading={false}
        isError={false}
        history={history}
        onValidate={vi.fn().mockResolvedValue({})}
        onSave={onSave}
        onRestore={vi.fn().mockResolvedValue(soulPayload())}
        onRetry={vi.fn()}
      />
    );

    expect((screen.getByTestId("agent-soul-textarea") as HTMLTextAreaElement).value).toBe(
      dirtyDraft
    );
    await user.click(screen.getByTestId("agent-soul-save"));
    expect(onSave).toHaveBeenCalledWith(dirtyDraft, "d1");
  });

  it("Should reseed draft and CAS when resourceKey changes across agents", async () => {
    const user = userEvent.setup();
    const view = render(
      <AgentAuthoredFileEditor
        resourceKey={resourceKeyA}
        kind="soul"
        payload={soulPayload({ digest: "digest-a", body: "# Soul\n\nAgent A." })}
        isLoading={false}
        isError={false}
        history={history}
        onValidate={vi.fn().mockResolvedValue({})}
        onSave={vi.fn().mockResolvedValue(soulPayload())}
        onRestore={vi.fn().mockResolvedValue(soulPayload())}
        onRetry={vi.fn()}
      />
    );

    const editor = screen.getByTestId("agent-soul-textarea");
    await user.type(editor, "\nDirty A draft");
    expect((editor as HTMLTextAreaElement).value).toContain("Dirty A draft");

    view.rerender(
      <AgentAuthoredFileEditor
        resourceKey={resourceKeyB}
        kind="soul"
        payload={soulPayload({
          digest: "digest-b",
          body: "# Soul\n\nAgent B.",
          frontmatter: {
            version: "1",
            role: "reviewer",
            tone: ["careful"],
            principles: ["Review carefully"],
          },
        })}
        isLoading={false}
        isError={false}
        history={history}
        onValidate={vi.fn().mockResolvedValue({})}
        onSave={vi.fn().mockResolvedValue(soulPayload())}
        onRestore={vi.fn().mockResolvedValue(soulPayload())}
        onRetry={vi.fn()}
      />
    );

    const next = screen.getByTestId("agent-soul-textarea") as HTMLTextAreaElement;
    expect(next.value).not.toContain("Dirty A draft");
    expect(next.value).toContain('role: "reviewer"');
  });

  it("Should render reload-and-retry recovery for a stale digest on save", async () => {
    const user = userEvent.setup();
    const retry = vi.fn();
    const onSave = vi.fn().mockRejectedValue(new AgentDigestConflictError("stale"));
    render(
      <AgentAuthoredFileEditor
        resourceKey={resourceKeyA}
        kind="soul"
        payload={soulPayload()}
        isLoading={false}
        isError={false}
        history={history}
        onValidate={vi.fn().mockResolvedValue({})}
        onSave={onSave}
        onRestore={vi.fn().mockResolvedValue(soulPayload())}
        onRetry={retry}
      />
    );

    await user.type(screen.getByTestId("agent-soul-textarea"), " changed");
    await user.click(screen.getByTestId("agent-soul-save"));
    expect(await screen.findByText("This file changed elsewhere. Reload and retry.")).toBeVisible();
    await user.click(screen.getByTestId("agent-soul-reload"));
    expect(retry).toHaveBeenCalledTimes(1);
    expect((screen.getByTestId("agent-soul-textarea") as HTMLTextAreaElement).value).not.toContain(
      "changed"
    );
  });

  it("Should render shared conflict recovery on create in the missing-file branch", async () => {
    const user = userEvent.setup();
    const retry = vi.fn();
    const onSave = vi.fn().mockRejectedValue(new AgentDigestConflictError("stale"));
    render(
      <AgentAuthoredFileEditor
        resourceKey={resourceKeyA}
        kind="soul"
        payload={soulPayload({ present: false, active: false, validation_status: "missing" })}
        isLoading={false}
        isError={false}
        history={{ revisions: [] } as AgentSoulHistoryResponse}
        onValidate={vi.fn().mockResolvedValue({})}
        onSave={onSave}
        onRestore={vi.fn().mockResolvedValue(soulPayload())}
        onRetry={retry}
      />
    );

    await user.click(screen.getByTestId("agent-soul-create"));
    expect(await screen.findByText("This file changed elsewhere. Reload and retry.")).toBeVisible();
    await user.click(screen.getByTestId("agent-soul-reload"));
    expect(retry).toHaveBeenCalledTimes(1);
  });

  it("Should render shared conflict recovery on restore without treating generic errors as conflicts", async () => {
    const user = userEvent.setup();
    const retry = vi.fn();
    const onRestore = vi.fn().mockRejectedValue(new AgentDigestConflictError("stale"));
    const view = render(
      <AgentAuthoredFileEditor
        resourceKey={resourceKeyA}
        kind="soul"
        payload={soulPayload()}
        isLoading={false}
        isError={false}
        history={history}
        onValidate={vi.fn().mockResolvedValue({})}
        onSave={vi.fn().mockResolvedValue(soulPayload())}
        onRestore={onRestore}
        onRetry={retry}
      />
    );

    await user.click(screen.getByTestId("agent-soul-history-toggle"));
    await user.click(screen.getByTestId("agent-soul-restore-rev-1"));
    expect(await screen.findByText("This file changed elsewhere. Reload and retry.")).toBeVisible();
    expect(screen.getByTestId("agent-soul-reload")).toBeVisible();

    view.rerender(
      <AgentAuthoredFileEditor
        resourceKey={resourceKeyA}
        kind="soul"
        payload={soulPayload()}
        isLoading={false}
        isError={false}
        history={history}
        onValidate={vi.fn().mockResolvedValue({})}
        onSave={vi.fn().mockRejectedValue(new Error("network down"))}
        onRestore={vi.fn().mockResolvedValue(soulPayload())}
        onRetry={retry}
      />
    );
    await user.type(screen.getByTestId("agent-soul-textarea"), " x");
    await user.click(screen.getByTestId("agent-soul-save"));
    expect(await screen.findByText("network down")).toBeVisible();
    expect(screen.queryByTestId("agent-soul-reload")).toBeNull();
  });

  it("Should create a missing file through PUT with the read digest and rebase from the payload", async () => {
    const user = userEvent.setup();
    const created = soulPayload({ digest: "created-digest", present: true, active: true });
    const onSave = vi.fn().mockResolvedValue(created);
    render(
      <AgentAuthoredFileEditor
        resourceKey={resourceKeyA}
        kind="soul"
        payload={soulPayload({ present: false, active: false, validation_status: "missing" })}
        isLoading={false}
        isError={false}
        history={{ revisions: [] } as AgentSoulHistoryResponse}
        onValidate={vi.fn().mockResolvedValue({})}
        onSave={onSave}
        onRestore={vi.fn().mockResolvedValue(soulPayload())}
        onRetry={vi.fn()}
      />
    );

    await user.click(screen.getByTestId("agent-soul-create"));
    expect(onSave).toHaveBeenCalledWith(expect.stringContaining("# Soul"), "soul-digest");
    expect(await screen.findByTestId("agent-soul-editor")).toBeVisible();
    expect(screen.getByText("valid")).toBeVisible();
    expect(screen.getByText("enabled")).toBeVisible();
    expect(screen.getByText("agents/coder/SOUL.md")).toBeVisible();
  });
});
