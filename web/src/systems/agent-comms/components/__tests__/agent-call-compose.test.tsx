import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { AgentCallCompose, type AgentCallComposeProps } from "../agent-call-compose";

function renderCompose(overrides: Partial<AgentCallComposeProps> = {}) {
  const props: AgentCallComposeProps = {
    agentName: "reviewer",
    prompt: "",
    onPromptChange: vi.fn(),
    expect: "",
    onExpectChange: vi.fn(),
    onSubmit: vi.fn(),
    ...overrides,
  };
  return { props, ...render(<AgentCallCompose {...props} />) };
}

describe("AgentCallCompose", () => {
  it("Should gate submission on the prompt and required contract", () => {
    const first = renderCompose({ contractRequired: true });

    expect(screen.getByTestId("agent-call-compose-submit")).toBeDisabled();
    expect(screen.getByTestId("agent-call-compose-contract-note")).toHaveTextContent(
      "Write it again"
    );

    first.unmount();
    renderCompose({ contractRequired: true, prompt: "Review this", expect: '{"ok":true}' });
    expect(screen.getByTestId("agent-call-compose-submit")).toBeEnabled();
  });

  it("Should describe whether the call starts or continues a helper", () => {
    const { rerender } = render(
      <AgentCallCompose
        agentName="reviewer"
        prompt="Review this"
        onPromptChange={vi.fn()}
        expect=""
        onExpectChange={vi.fn()}
        onSubmit={vi.fn()}
      />
    );

    expect(screen.getByTestId("agent-call-compose-target")).toHaveTextContent(
      "Starts a new reviewer"
    );

    rerender(
      <AgentCallCompose
        agentName="reviewer"
        target={{ kind: "session", sessionId: "ses_child", agentName: "reviewer" }}
        prompt="Review this"
        onPromptChange={vi.fn()}
        expect=""
        onExpectChange={vi.fn()}
        onSubmit={vi.fn()}
      />
    );
    expect(screen.getByTestId("agent-call-compose-target")).toHaveTextContent(
      "Continues reviewer in ses_child"
    );
  });

  it("Should submit once and forward editor changes", async () => {
    const user = userEvent.setup();
    const { props } = renderCompose({ prompt: "Review this" });

    await user.click(screen.getByTestId("agent-call-compose-submit"));
    await user.type(screen.getByTestId("agent-call-compose-prompt"), " now");
    await user.type(screen.getByTestId("agent-call-compose-expect"), "shape");

    expect(props.onSubmit).toHaveBeenCalledOnce();
    expect(props.onPromptChange).toHaveBeenCalled();
    expect(props.onExpectChange).toHaveBeenCalled();
  });

  it("Should show the typed refusal and open an accepted call", async () => {
    const user = userEvent.setup();
    const onOpenAcceptedCall = vi.fn();
    renderCompose({
      prompt: "Review this",
      failure: { code: "call_agent_unknown", message: "There is no agent by that name here." },
      accepted: { callId: "call_123", childSessionId: "ses_child" },
      onOpenAcceptedCall,
    });

    expect(screen.getByTestId("agent-call-compose-error")).toHaveTextContent("call_agent_unknown");
    expect(screen.getByTestId("agent-call-compose-accepted")).toHaveTextContent(
      "reviewer is working in ses_child"
    );

    await user.click(screen.getByRole("button", { name: "Open call_123" }));
    expect(onOpenAcceptedCall).toHaveBeenCalledOnce();
    expect(onOpenAcceptedCall).toHaveBeenCalledWith("call_123");
  });
});
