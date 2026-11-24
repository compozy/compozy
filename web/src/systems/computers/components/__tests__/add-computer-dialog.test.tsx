// Suite: AddComputerDialog live SSH, account, and command paths (S8 / U2)
// Invariant: live controller states render all 7 steps, host-key trust & changed recovery without
// a replace action, operator secret submission that clears state, plan limit, cancel vs remove,
// and command watcher detection.
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { AddComputerDialog } from "../add-computer-dialog";
import type { AddComputerLiveController } from "../../hooks/use-live-add-computer";

function makeLiveController(
  overrides: Partial<AddComputerLiveController> = {}
): AddComputerLiveController {
  return {
    method: "ssh",
    setMethod: vi.fn(),
    draft: { host: "staging-box", user: "deploy", port: "22", name: "staging-box" },
    setDraft: vi.fn(),
    sshStage: "picker",
    setupStatus: null,
    hostKeyPrompt: null,
    secretCountdownSeconds: 180,
    secretError: null,
    isAgentInitiated: false,
    linkedCommitReached: false,
    addedIds: [],
    detectedCommandComputer: null,
    onStartSsh: vi.fn(),
    onTrustHost: vi.fn(),
    onDeclineHost: vi.fn(),
    onSubmitSecret: vi.fn(),
    onConfirmCancel: vi.fn(),
    onRetrySetup: vi.fn(),
    onAddAccountComputer: vi.fn(),
    onBackToPicker: vi.fn(),
    onRemoveLinked: vi.fn(),
    ...overrides,
  };
}

describe("AddComputerDialog (live mode)", () => {
  it("renders the trust host key prompt and invokes onTrustHost / onDeclineHost", () => {
    const live = makeLiveController({
      sshStage: "trust_host",
      setupStatus: {
        setup_id: "stp_1",
        computer_id: null,
        state: "needs_input",
        step: "connect",
        prompt: { kind: "trust_host", host: "deploy@10.0.0.71:22", fingerprint: "SHA256:abcd1234" },
        error: null,
      },
      hostKeyPrompt: {
        mode: "trust",
        target: "deploy@10.0.0.71:22",
        pinEntry: "10.0.0.71",
        keyType: "ED25519",
        fingerprint: "SHA256:abcd1234",
      },
    });

    render(
      <AddComputerDialog
        accountEmail="pedro@example.com"
        live={live}
        open
        sshSuggestions={[]}
        unaddedComputers={[]}
      />
    );

    expect(screen.getByText("Trust this host?")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /Trust and continue/i }));
    expect(live.onTrustHost).toHaveBeenCalledTimes(1);

    fireEvent.click(screen.getByRole("button", { name: "Decline" }));
    expect(live.onDeclineHost).toHaveBeenCalledTimes(1);
  });

  it("renders host_key_changed with recovery command and no Replace button", () => {
    const live = makeLiveController({
      sshStage: "host_key_changed",
      setupStatus: {
        setup_id: "stp_2",
        computer_id: null,
        state: "failed",
        step: "connect",
        prompt: null,
        error: { code: "ssh_host_key_changed", error: "Host key changed" },
      },
      hostKeyPrompt: {
        mode: "changed",
        target: "pedro@10.0.0.42:2222",
        pinEntry: "[10.0.0.42]:2222",
        keyType: "ED25519",
        fingerprint: "SHA256:observed",
        previousFingerprint: "SHA256:pinned",
        recoveryCommand: "ssh-keygen -R '[10.0.0.42]:2222' -f ~/.compozy/ssh/known_hosts",
      },
    });

    render(
      <AddComputerDialog
        accountEmail="pedro@example.com"
        live={live}
        open
        sshSuggestions={[]}
        unaddedComputers={[]}
      />
    );

    expect(screen.getByText(/key changed since you trusted it/i)).toBeInTheDocument();
    expect(
      screen.getByText(/ssh-keygen -R '\[10\.0\.0\.42\]:2222' -f ~\/\.compozy\/ssh\/known_hosts/)
    ).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /replace/i })).not.toBeInTheDocument();
  });

  it("submits operator secret, clears the input immediately, and shows agent handoff context", () => {
    const live = makeLiveController({
      sshStage: "agent_secret",
      isAgentInitiated: true,
      setupStatus: {
        setup_id: "stp_3",
        computer_id: null,
        state: "needs_operator_secret",
        step: "connect",
        prompt: { kind: "secret", host: "deploy@10.0.0.71" },
        error: null,
      },
    });

    render(
      <AddComputerDialog
        accountEmail="pedro@example.com"
        live={live}
        open
        sshSuggestions={[]}
        unaddedComputers={[]}
      />
    );

    expect(screen.getByText(/Agents cannot see or answer this prompt/i)).toBeInTheDocument();
    const input = screen.getByLabelText(/Password or code for/i) as HTMLInputElement;
    fireEvent.change(input, { target: { value: "s3cr3t-otp" } });
    fireEvent.click(screen.getByRole("button", { name: "Continue" }));

    expect(live.onSubmitSecret).toHaveBeenCalledWith("s3cr3t-otp");
    expect(input.value).toBe("");
  });

  it("switches Cancel to Remove once linkedCommitReached is true (Rule 19)", () => {
    const live = makeLiveController({
      sshStage: "running",
      linkedCommitReached: true,
      setupStatus: {
        setup_id: "stp_4",
        computer_id: "cmp_new",
        state: "running",
        step: "forward",
        prompt: null,
        error: null,
      },
    });

    render(
      <AddComputerDialog
        accountEmail="pedro@example.com"
        live={live}
        open
        sshSuggestions={[]}
        unaddedComputers={[]}
      />
    );

    const removeBtn = screen.getByRole("button", { name: "Remove" });
    fireEvent.click(removeBtn);
    expect(live.onRemoveLinked).toHaveBeenCalledTimes(1);
  });

  it("shows live newly linked computer in Link with a command path", () => {
    const detected = {
      id: "cmp_office",
      name: "office-win",
      platform: "windows_x86_64" as const,
      iconKind: "windows" as const,
      online: true,
      hostedOnly: false,
      linkedVia: "command" as const,
    };
    const live = makeLiveController({
      method: "command",
      detectedCommandComputer: detected,
    });

    render(
      <AddComputerDialog
        accountEmail="pedro@example.com"
        live={live}
        open
        sshSuggestions={[]}
        unaddedComputers={[]}
      />
    );

    expect(screen.getByText("office-win")).toBeInTheDocument();
    expect(screen.getByText("Just linked")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /^Add$/i }));
    expect(live.onAddAccountComputer).toHaveBeenCalledWith(detected);
  });

  it("stops at Checking your plan on plan_limit_reached and invokes onUpgrade from Upgrade to Pro", () => {
    const onUpgrade = vi.fn();
    const live = makeLiveController({
      sshStage: "plan_limit",
      setupStatus: {
        setup_id: "stp_limit",
        computer_id: null,
        state: "failed",
        step: "admit",
        prompt: null,
        error: {
          code: "plan_limit_reached",
          error: "Free includes 1 remote computer. Upgrade to Pro for unlimited computers.",
        },
      },
    });

    render(
      <AddComputerDialog
        accountEmail="pedro@example.com"
        isFreePlan
        live={live}
        onUpgrade={onUpgrade}
        open
        sshSuggestions={[]}
        unaddedComputers={[]}
      />
    );

    expect(screen.getByText("Checking your plan")).toBeInTheDocument();
    expect(screen.getByText("Free includes 1 remote computer")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Upgrade to Pro" }));
    expect(onUpgrade).toHaveBeenCalledTimes(1);
  });
});
