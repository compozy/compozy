// Suite: SSH setup and computer view mappers (S8)
// Invariant: live daemon DTOs map deterministically to the 7-step SSH flow, host-key recovery
// commands, Rule 19 post-link commit detection, 4-hop diagnosis, linger notices, and mismatch flags.
import { describe, expect, it } from "vitest";

import { toComputerView, toLocalComputerView, toSSHHostSuggestionView } from "../computer-view";
import {
  buildHostKeyRecoveryCommand,
  buildLiveSshSetupSteps,
  buildLiveUpdateSteps,
  extractLiveHostKeyPrompt,
  formatKnownHostsPinEntry,
  isSetupLinkedCommitReached,
  toLiveSshStage,
  toLiveUpdateState,
} from "../ssh-setup-view";
import type { ComputerPayload, ComputerSetupStatusPayload } from "../../types";

function makeComputer(overrides: Partial<ComputerPayload> = {}): ComputerPayload {
  return {
    id: "cmp_devbox",
    account_id: "acct_1",
    name: "devbox",
    platform: "linux_x86_64",
    version: "0.8.0",
    protocol: { min: 1, max: 1 },
    status: "connected",
    enabled: true,
    plan_state: "ok",
    load_balancing_weight: 50,
    route_in_use: { id: "rt_ssh", kind: "ssh", label: "via SSH pedro@10.0.0.42" },
    routes: [
      {
        id: "rt_lan",
        kind: "direct",
        address: "192.168.1.42:47823",
        in_use: false,
        learned: true,
        last_error: null,
      },
      {
        id: "rt_ssh",
        kind: "ssh",
        address: "pedro@10.0.0.42",
        in_use: true,
        learned: false,
        last_error: null,
      },
    ],
    scopes: ["sessions:operate", "terminal:operate"],
    diagnosis: null,
    added_at: "2026-11-10T00:00:00Z",
    last_connected_at: "2026-11-22T12:00:00Z",
    ...overrides,
  };
}

describe("ssh-setup-view", () => {
  it("formats non-22 known_hosts pin entries with brackets and builds the recovery command", () => {
    expect(formatKnownHostsPinEntry("10.0.0.42", 22)).toBe("10.0.0.42");
    expect(formatKnownHostsPinEntry("10.0.0.42", "2222")).toBe("[10.0.0.42]:2222");
    expect(buildHostKeyRecoveryCommand("[10.0.0.42]:2222")).toBe(
      "ssh-keygen -R '[10.0.0.42]:2222' -f ~/.compozy/ssh/known_hosts"
    );
  });

  it("maps live setup states to AddComputerSshStage and 7-step progress items", () => {
    const trustStatus: ComputerSetupStatusPayload = {
      setup_id: "stp_1",
      computer_id: null,
      state: "needs_host_key",
      step: "connect",
      prompt: {
        kind: "host_key",
        host: "deploy@10.0.0.71:22",
        fingerprint: "SHA256:abcdef123456",
      },
      error: null,
    };

    expect(toLiveSshStage(trustStatus)).toBe("trust_host");
    expect(isSetupLinkedCommitReached(trustStatus)).toBe(false);

    const prompt = extractLiveHostKeyPrompt(trustStatus, "10.0.0.71", "22");
    expect(prompt).toMatchObject({
      mode: "trust",
      target: "deploy@10.0.0.71:22",
      fingerprint: "SHA256:abcdef123456",
    });

    const steps = buildLiveSshSetupSteps({
      status: trustStatus,
      hostLabel: "staging-box",
      targetAddress: "deploy@10.0.0.71",
      accountEmail: "pedro@example.com",
    });
    expect(steps.map(s => [s.id, s.status])).toEqual([
      ["admit", "done"],
      ["verify", "done"],
      ["connect", "needs_input"],
      ["install", "pending"],
      ["service", "pending"],
      ["link", "pending"],
      ["forward", "pending"],
    ]);
  });

  it("recognizes host_key_changed, plan_limit_reached, unsupported platform, and linked commit", () => {
    const changedStatus: ComputerSetupStatusPayload = {
      setup_id: "stp_2",
      computer_id: null,
      state: "failed",
      step: "connect",
      prompt: null,
      error: {
        code: "ssh_host_key_changed",
        error: "Host key changed",
        details: {
          host: "10.0.0.42",
          pin_entry: "[10.0.0.42]:2222",
          pinned_fingerprint: "SHA256:old",
          observed_fingerprint: "SHA256:new",
        },
      },
    };
    expect(toLiveSshStage(changedStatus)).toBe("host_key_changed");
    expect(extractLiveHostKeyPrompt(changedStatus, "10.0.0.42", "2222")).toMatchObject({
      mode: "changed",
      previousFingerprint: "SHA256:old",
      fingerprint: "SHA256:new",
      recoveryCommand: "ssh-keygen -R '[10.0.0.42]:2222' -f ~/.compozy/ssh/known_hosts",
    });

    expect(
      toLiveSshStage({
        setup_id: "stp_3",
        computer_id: null,
        state: "failed",
        step: "admit",
        prompt: null,
        error: { code: "plan_limit_reached", error: "Free includes 1 computer" },
      })
    ).toBe("plan_limit");

    expect(
      toLiveSshStage({
        setup_id: "stp_4",
        computer_id: null,
        state: "failed",
        step: "connect",
        prompt: null,
        error: { code: "ssh_unsupported_platform", error: "Windows is unsupported over SSH" },
      })
    ).toBe("unsupported");

    expect(
      isSetupLinkedCommitReached({
        setup_id: "stp_5",
        computer_id: "cmp_new",
        state: "running",
        step: "forward",
        prompt: null,
        error: null,
      })
    ).toBe(true);
  });

  it("projects SSH update statuses into 4-step progress lists", () => {
    const running: ComputerSetupStatusPayload = {
      setup_id: "upd_1",
      computer_id: "cmp_gpu",
      state: "running",
      step: "install",
      prompt: null,
      error: null,
    };
    expect(toLiveUpdateState(running)).toBe("running");
    expect(buildLiveUpdateSteps(running, "gpu-vm").map(s => [s.id, s.status])).toEqual([
      ["verify", "done"],
      ["install", "running"],
      ["service", "pending"],
      ["forward", "pending"],
    ]);
  });
});

describe("computer-view enrichments", () => {
  it("builds 4-hop diagnosis, linger attention notice, and route identity mismatch flags", () => {
    const view = toComputerView(
      makeComputer({
        status: "offline",
        routes: [
          {
            id: "rt_bad",
            kind: "direct",
            address: "192.168.1.99:47823",
            in_use: false,
            learned: true,
            last_error: "route_identity_mismatch: Different computer at this address",
          },
        ],
        diagnosis: {
          step: "route",
          code: "route_unreachable",
          message: "All routes timed out.",
          at: "2026-11-22T12:00:00Z",
        },
      })
    );

    expect(view.routes[0]?.identityMismatch).toBe(true);
    expect(view.diagnosis?.steps.map(s => [s.step, s.status])).toEqual([
      ["cloud", "done"],
      ["route", "failed"],
      ["computer", "skipped"],
      ["sign_in", "skipped"],
    ]);

    const linger = toComputerView(
      makeComputer({
        status: "needs_attention",
        diagnosis: {
          step: "computer",
          code: "linger_off",
          message: "Enable lingering so agents stay up after logout.",
          at: "2026-11-22T12:00:00Z",
        },
      })
    );
    expect(linger.attentionReason?.kind).toBe("linger_off");
    expect(linger.attentionReason?.command).toContain("loginctl enable-linger pedro");
  });

  it("marks SSH host suggestions as alreadyAdded and maps remote host status", () => {
    const existing = makeComputer();
    const suggestion = toSSHHostSuggestionView(
      { alias: "devbox", hostname: "10.0.0.42", user: "pedro", port: 22, source: "ssh_config" },
      [existing]
    );
    expect(suggestion.alreadyAdded).toBe(true);

    const local = toLocalComputerView(
      { run_agents: true, load_balancing: false, load_balancing_weight: 50 },
      {
        name: "MacBook Pro",
        remote: {
          enabled: true,
          computer_id: "cmp_local",
          control_link: "connected",
          listener: "0.0.0.0:47823",
          require_device_approval: true,
          direct: { enabled: true },
          cloud_route: { enabled: true, hostname: "macbook-1a2b.compozy.run", state: "ready" },
          entitlement: null,
        },
      }
    );
    expect(local.remoteEnabled).toBe(true);
    expect(local.requireDeviceApproval).toBe(true);
    expect(local.hostedRouteEnabled).toBe(true);
    expect(local.hostedRouteState).toBe("ready");
    expect(local.hostedUrl).toBe("macbook-1a2b.compozy.run");
  });
});
