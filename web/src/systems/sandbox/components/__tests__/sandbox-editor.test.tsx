import { render, screen } from "@testing-library/react";
import { useState } from "react";
import { describe, expect, it, vi } from "vitest";

import { emptySandboxDraft, type SandboxDraft } from "../../lib/sandbox-profile-draft";
import { SandboxEditor } from "../sandbox-editor";

function EditorHarness({
  draft = emptySandboxDraft(),
  initialTier = "advanced",
}: {
  draft?: SandboxDraft;
  initialTier?: "simple" | "advanced";
}) {
  const [currentDraft, setDraft] = useState(draft);

  return (
    <SandboxEditor
      editor={{ mode: "create", draft: currentDraft }}
      error={null}
      existingNames={["existing-profile"]}
      initialTier={initialTier}
      isSaving={false}
      isValid
      onChange={updater => setDraft(updater)}
      onClose={vi.fn()}
      onSave={vi.fn()}
    />
  );
}

describe("SandboxEditor", () => {
  it("Should use multiline controls for environment and network rule collections", () => {
    render(
      <EditorHarness
        draft={{
          ...emptySandboxDraft(),
          env: [{ key: "LOG_LEVEL", value: "info" }],
          secretEnv: [{ key: "TOKEN", value: "env:SANDBOX_TOKEN" }],
          network: { allow_list: ["api.example.com"], deny_list: ["metadata.internal"] },
        }}
      />
    );

    for (const testId of [
      "sandbox-editor-env",
      "sandbox-editor-secret-env",
      "sandbox-editor-allow-list",
      "sandbox-editor-deny-list",
    ]) {
      expect(screen.getByTestId(testId).tagName).toBe("TEXTAREA");
    }
  });

  it("Should show duplicate profile feedback beside the editable name", () => {
    render(
      <EditorHarness
        draft={{ ...emptySandboxDraft(), name: "existing-profile" }}
        initialTier="simple"
      />
    );

    expect(screen.getByTestId("sandbox-editor-name-error")).toHaveTextContent(
      'A sandbox named "existing-profile" already exists.'
    );
    expect(screen.getByTestId("settings-sandbox-editor-save")).toBeDisabled();
  });

  it("Should show invalid environment-key feedback and block saving", () => {
    render(
      <EditorHarness
        draft={{
          ...emptySandboxDraft(),
          env: [{ key: "9INVALID", value: "ignored" }],
        }}
      />
    );

    expect(screen.getByTestId("sandbox-editor-env-error")).toHaveTextContent(
      "9INVALID is not a valid environment variable name."
    );
    expect(screen.getByTestId("settings-sandbox-editor-save")).toBeDisabled();
  });

  it("Should expose the daemon sync-mode vocabulary", () => {
    render(<EditorHarness initialTier="simple" />);

    expect(screen.getByTestId("sandbox-editor-sync-mode")).toHaveValue("");
    expect(
      screen.getByRole("option", { name: "Session bidirectional — sync at start and stop" })
    ).toHaveValue("session-bidirectional");
    expect(
      screen.getByRole("option", { name: "Turn bidirectional — sync every turn" })
    ).toHaveValue("turn-bidirectional");
  });

  it("Should omit unsupported Daytona network rules and explain the limitation", () => {
    render(
      <EditorHarness
        draft={{
          ...emptySandboxDraft(),
          backend: "daytona",
          network: {
            allow_public_ingress: true,
            allow_outbound: true,
            allow_list: ["api.example.com"],
            deny_list: ["metadata.internal"],
          },
        }}
      />
    );

    expect(screen.getByTestId("sandbox-editor-allow-ingress")).toBeInTheDocument();
    expect(screen.queryByTestId("sandbox-editor-allow-outbound")).toBeNull();
    expect(screen.queryByTestId("sandbox-editor-allow-list")).toBeNull();
    expect(screen.queryByTestId("sandbox-editor-deny-list")).toBeNull();
    expect(screen.getByTestId("sandbox-editor-daytona-network-limitation")).toHaveTextContent(
      "does not enforce outbound or host allow and deny rules"
    );
  });
});
