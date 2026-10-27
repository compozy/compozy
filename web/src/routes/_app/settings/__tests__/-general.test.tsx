import { fireEvent, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { renderWithTopbar as render } from "@/test/render-with-topbar";
import { createElement, type ReactNode, type SetStateAction } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type {
  SettingsUpdateApplyResult,
  SettingsUpdateCancelResult,
  SettingsUpdateStatus,
} from "@/systems/settings";
import {
  settingsUpdateApplyingFixture,
  settingsUpdateBlockedFixture,
  settingsUpdateBothAvailableFixture,
  settingsUpdateManagedFixture,
  settingsUpdateNoAppFixture,
  settingsUpdateRolledBackFixture,
  settingsUpdateStagedFixture,
  settingsUpdateStatusFixture,
} from "@/systems/settings/mocks/settings-update-fixture";

type Envelope = typeof envelope;
type Mutation = {
  mutate: ReturnType<typeof vi.fn>;
  isPending: boolean;
  error: Error | null;
  data: { warnings: string[] } | undefined;
};

type RestartBanner = {
  isVisible: boolean;
  isRestartRequired: boolean;
  operationId: string | null;
  status: string | null;
  failureReason?: string;
  activeSessionCount: number;
  trigger: ReturnType<typeof vi.fn>;
  isTriggerPending: boolean;
  triggerError: unknown;
  dismiss: ReturnType<typeof vi.fn>;
};

type UpdateStatus = SettingsUpdateStatus;

type UpdateActions = {
  check: ReturnType<typeof vi.fn>;
  isChecking: boolean;
  checkError: string | null;
  apply: ReturnType<typeof vi.fn>;
  cancel: ReturnType<typeof vi.fn>;
  isApplying: boolean;
  isCanceling: boolean;
  result: SettingsUpdateApplyResult | null;
  cancelResult: SettingsUpdateCancelResult | null;
  error: string | null;
};

type ApplyRecordsQuery = {
  data: {
    entries: {
      id: string;
      desired_config_hash: string;
      active_config_hash: string;
      generation: number;
      actor: string;
      diff_class: "live" | "restart-required";
      status: "applied" | "blocked";
      lifecycle: "live" | "restart-required";
      next_action: "none" | "restart-daemon";
      created_at: string;
      updated_at: string;
    }[];
  };
  isLoading: boolean;
  isFetching: boolean;
  error: Error | null;
  refetch: ReturnType<typeof vi.fn>;
};

const envelope = {
  section: "general" as const,
  scope: "user" as const,
  available_scopes: ["user"] as const,
  actions: {
    restart: { available: true, behavior: "action_trigger" as const, name: "restart" },
  },
  config: {
    busy_input: { default_mode: "steer" },
    daemon: {
      memory_report_interval: "5m",
      reload_timeouts: { mcp: "10s", providers: "5s" },
      socket: "/tmp/compozy.sock",
    },
    http: { host: "127.0.0.1", port: 2123 },
    limits: { max_concurrent_agents: 20 },
    permissions: { mode: "approve-all" as const },
    redact: { enabled: true },
    session_timeout: "0s",
    idle_release_after: "30m0s" as string | undefined,
    terminal: {
      default_shell: "",
      shell_integration: true,
      scrollback_bytes: 1_048_576,
      detached_ttl: "24h",
      exit_retention: "15m",
      recording: false,
      recording_retention_days: 30,
      max_per_workspace: 8,
      max_per_daemon: 32,
      max_subscribers: 16,
    },
  },
  config_paths: {
    daemon_info: "/tmp/daemon.json",
    global_config: "~/.compozy/config.toml",
    global_mcp_sidecar: "~/.compozy/mcp.json",
    home_dir: "~/.compozy",
    log_file: "~/.compozy/compozy.log",
  },
  runtime: {
    active_agents: 7,
    active_sessions: 4,
    available: true,
    http_host: "127.0.0.1",
    http_port: 2123,
    socket: "~/.compozy/daemon.sock",
    total_sessions: 12,
    uptime_seconds: 3600,
  },
};

let pageState: {
  isLoading: boolean;
  error: Error | null;
  envelope: Envelope | null;
  draft: Envelope["config"] | null;
  setDraft: ReturnType<typeof vi.fn>;
  isDirty: boolean;
  isSaving: boolean;
  saveError: string | null;
  warnings: string[] | undefined;
  lastAppliedLabel: string | null;
  handleReset: ReturnType<typeof vi.fn>;
  handleSave: ReturnType<typeof vi.fn>;
  handleRetry: ReturnType<typeof vi.fn>;
  restart: RestartBanner;
  update: {
    data: UpdateStatus | null;
    isError: boolean;
    isLoading: boolean;
    isFetching: boolean;
    error: Error | null;
    refetch: ReturnType<typeof vi.fn>;
  };
  updateActions: UpdateActions;
  applyRecords: ApplyRecordsQuery;
  handleReload: ReturnType<typeof vi.fn>;
  isReloading: boolean;
  reloadError: string | null;
  reloadResult: null;
};

const restartBanner: RestartBanner = {
  isVisible: false,
  isRestartRequired: false,
  operationId: null,
  status: null,
  failureReason: undefined,
  activeSessionCount: 0,
  trigger: vi.fn(),
  isTriggerPending: false,
  triggerError: null,
  dismiss: vi.fn(),
};

vi.mock("@tanstack/react-router", async importOriginal => {
  const actual = await importOriginal<typeof import("@tanstack/react-router")>();
  return {
    ...actual,
    createFileRoute: () => (opts: { component: () => ReactNode }) => ({
      component: opts.component,
    }),
    useNavigate: () => vi.fn(),
  };
});

vi.mock("@/systems/settings/hooks/use-settings-general-page", () => ({
  useSettingsGeneralPage: () => pageState,
}));

const mockMutation: Mutation = {
  mutate: vi.fn(),
  isPending: false,
  error: null,
  data: undefined,
};

type PersonaConfig = {
  agent: string;
  provider?: string;
  model?: string;
  reasoning_effort?: string;
};

let personaState: {
  draft: PersonaConfig | null;
  error: Error | null;
  isDirty: boolean;
  isLoading: boolean;
  isSaving: boolean;
  profileName: string;
  saveError: string | null;
  warnings: string[] | undefined;
  setDraft: ReturnType<typeof vi.fn>;
  handleSave: ReturnType<typeof vi.fn>;
  handleReset: ReturnType<typeof vi.fn>;
  handleRetry: ReturnType<typeof vi.fn<() => void>>;
};

let providersState: {
  data: { providers: Array<{ name: string }> };
  error: Error | null;
  isLoading: boolean;
  refetch: ReturnType<typeof vi.fn<() => void>>;
};

const runtimeCatalog = {
  providers: [
    {
      id: "claude",
      name: "Claude",
      runtime_provider: "claude",
      runtime_strategy: "configured",
      needs_auth: false,
    },
  ],
  models: [],
  loading: false,
  catalogError: null,
  catalogStale: false,
  refresh: vi.fn(),
  refreshing: false,
};

vi.mock("@/systems/settings", async importOriginal => {
  const actual = await importOriginal<typeof import("@/systems/settings")>();
  return {
    ...actual,
    useSettingsGeneral: () => ({ data: envelope, isLoading: false, error: null }),
    useUpdateSettingsGeneral: () => mockMutation,
    useSettingsSessionDefaults: () => ({
      persona: personaState,
      runtime: runtimeCatalog,
      isLoading: personaState.isLoading || providersState.isLoading,
      error: personaState.error ?? providersState.error,
      retry: () => {
        personaState.handleRetry();
        void providersState.refetch();
      },
    }),
    SettingsApiError: class SettingsApiError extends Error {},
  };
});

// The remembered-decisions section owns its own workspace-scoped query/mutation; the
// page harness has no QueryClientProvider, so stub it to a marker and assert composition.
vi.mock("@/systems/tool-approvals", () => ({
  ToolApprovalGrantsSection: () =>
    createElement("div", { "data-testid": "settings-page-general-tool-approvals" }),
}));

beforeEach(() => {
  personaState = {
    draft: { agent: "general" },
    error: null,
    isDirty: false,
    isLoading: false,
    isSaving: false,
    profileName: "default",
    saveError: null,
    warnings: undefined,
    setDraft: vi.fn(),
    handleSave: vi.fn(),
    handleReset: vi.fn(),
    handleRetry: vi.fn(),
  };
  providersState = {
    data: { providers: [{ name: "claude" }] },
    error: null,
    isLoading: false,
    refetch: vi.fn(),
  };
  pageState = {
    isLoading: false,
    error: null,
    envelope,
    draft: structuredClone(envelope.config),
    setDraft: vi.fn(),
    isDirty: false,
    isSaving: false,
    saveError: null,
    warnings: undefined,
    lastAppliedLabel: null,
    handleReset: vi.fn(),
    handleSave: vi.fn(),
    handleRetry: vi.fn(),
    restart: { ...restartBanner, trigger: vi.fn(), dismiss: vi.fn() },
    update: {
      data: settingsUpdateBothAvailableFixture,
      isLoading: false,
      isError: false,
      isFetching: false,
      error: null,
      refetch: vi.fn(),
    },
    updateActions: {
      check: vi.fn(),
      isChecking: false,
      checkError: null,
      apply: vi.fn(),
      cancel: vi.fn(),
      isApplying: false,
      isCanceling: false,
      result: null,
      cancelResult: null,
      error: null,
    },
    applyRecords: {
      data: {
        entries: [
          {
            id: "cfg_apply_001",
            desired_config_hash: "sha256:desired",
            active_config_hash: "sha256:active",
            generation: 7,
            actor: "http",
            diff_class: "restart-required",
            status: "blocked",
            lifecycle: "restart-required",
            next_action: "restart-daemon",
            created_at: "2026-05-20T13:00:00Z",
            updated_at: "2026-05-20T13:00:01Z",
          },
        ],
      },
      isLoading: false,
      isFetching: false,
      error: null,
      refetch: vi.fn(),
    },
    handleReload: vi.fn(),
    isReloading: false,
    reloadError: null,
    reloadResult: null,
  };
});

import { GeneralSettingsPage } from "../-general-settings-page";

describe("GeneralSettingsPage", () => {
  it("renders a loading indicator while fetching", () => {
    pageState.isLoading = true;
    pageState.envelope = null;
    pageState.draft = null;
    render(<GeneralSettingsPage />);
    expect(screen.getByTestId("settings-page-general-loading")).toBeInTheDocument();
  });

  it("renders the error state when the query fails", () => {
    pageState.error = new Error("boom");
    pageState.envelope = null;
    pageState.draft = null;
    render(<GeneralSettingsPage />);
    const error = screen.getByTestId("settings-page-general-error");
    expect(error).toHaveTextContent("Couldn't load General");
    // The raw failure stays reachable behind the Details fold, never as the headline.
    expect(error).toHaveTextContent("boom");
    expect(screen.getByRole("heading", { name: "Couldn't load General" })).toBeInTheDocument();
  });

  it("renders runtime, permissions, and config path from the section envelope", () => {
    render(<GeneralSettingsPage />);

    expect(screen.getByTestId("settings-page-general-subhead")).toHaveTextContent(
      "4 active sessions"
    );
    expect(screen.getByTestId("settings-page-general-subhead")).toHaveTextContent(
      "7 agents working"
    );
    expect(screen.getByTestId("settings-page-general-permissions-group")).toBeInTheDocument();
    expect(screen.getByRole("radio", { name: /Allow everything/ })).toBeChecked();
    fireEvent.click(screen.getByRole("button", { name: "Advanced" }));
    expect(screen.getByText("Settings file")).toBeInTheDocument();
    expect(screen.getByText("~/.compozy/config.toml")).toBeInTheDocument();
  });

  it("keeps where CompozyOS listens inside the Advanced fold, not on the primary layer", () => {
    render(<GeneralSettingsPage />);

    const advanced = screen.getByTestId("settings-page-general-advanced");
    expect(within(advanced).getByTestId("settings-page-general-socket")).toBeInTheDocument();
    expect(within(advanced).getByTestId("settings-page-general-http-address")).toBeInTheDocument();
    expect(
      within(advanced).getByTestId("settings-page-general-memory-report-interval")
    ).toBeInTheDocument();
  });

  it("shows the unreachable fact once in the meta line when runtime facts are missing", () => {
    pageState.envelope = { ...envelope, runtime: { ...envelope.runtime, available: false } };
    render(<GeneralSettingsPage />);

    expect(screen.getByTestId("settings-page-general-subhead")).toHaveTextContent(
      "CompozyOS isn't reachable"
    );
  });

  it("writes the chosen idle-agent preset as seconds, including Never", () => {
    let nextDraft: Envelope["config"] = envelope.config;
    pageState.setDraft.mockImplementation((update: SetStateAction<Envelope["config"] | null>) => {
      const updated = typeof update === "function" ? update(envelope.config) : update;
      if (updated !== null) nextDraft = updated;
    });
    render(<GeneralSettingsPage />);

    const select = screen.getByTestId("settings-page-general-idle-release-input");
    fireEvent.change(select, { target: { value: String(60 * 60) } });
    expect(nextDraft.idle_release_after).toBe("3600s");
    fireEvent.change(select, { target: { value: "0" } });
    expect(nextDraft.idle_release_after).toBe("0s");
    expect(nextDraft.session_timeout).toBe(envelope.config.session_timeout);
  });

  it.each([
    ["0s", "Never"],
    ["15m0s", "15 minutes"],
    ["30m0s", "30 minutes"],
    ["1h0m0s", "1 hour"],
    ["4h0m0s", "4 hours"],
    ["24h0m0s", "1 day"],
    ["1h30m0s", "90 minutes"],
    ["2h0m0s", "2 hours"],
    ["1m0s", "1 minute"],
    ["1.5s", "1.5 seconds"],
  ])("Should display the saved idle-agent duration %s as %s", (duration, label) => {
    pageState.draft = { ...envelope.config, idle_release_after: duration };
    render(<GeneralSettingsPage />);

    expect(screen.getByTestId("settings-page-general-idle-release-input")).toHaveDisplayValue(
      label
    );
  });

  it("shows the 30-minute default when the daemon omits the idle-agent duration", () => {
    pageState.draft = { ...envelope.config, idle_release_after: undefined };
    render(<GeneralSettingsPage />);

    expect(screen.getByTestId("settings-page-general-idle-release-input")).toHaveDisplayValue(
      "30 minutes"
    );
  });

  it("composes the remembered-decisions section after the permissions policy", () => {
    render(<GeneralSettingsPage />);

    const permissions = screen.getByTestId("settings-page-general-permissions-group");
    const grants = screen.getByTestId("settings-page-general-tool-approvals");
    expect(grants).toBeInTheDocument();
    expect(
      permissions.compareDocumentPosition(grants) & Node.DOCUMENT_POSITION_FOLLOWING
    ).toBeTruthy();
  });

  it("reads and edits the daemon memory report interval as a duration string", () => {
    render(<GeneralSettingsPage />);

    const input = screen.getByTestId("settings-page-general-memory-report-interval-input");
    expect(input).toHaveValue("5m");

    fireEvent.change(input, { target: { value: "10m" } });
    expect(pageState.setDraft).toHaveBeenCalledTimes(1);
  });

  it("shows the disabled sampling value when the interval is 0s", () => {
    pageState.draft = structuredClone(envelope.config);
    pageState.draft.daemon.memory_report_interval = "0s";
    render(<GeneralSettingsPage />);

    expect(screen.getByTestId("settings-page-general-memory-report-interval-input")).toHaveValue(
      "0s"
    );
  });

  it("reads the daemon follow-up default and writes the flipped value through the page draft handler", () => {
    let nextDraft: Envelope["config"] = envelope.config;
    pageState.setDraft.mockImplementation((update: SetStateAction<Envelope["config"] | null>) => {
      const updated = typeof update === "function" ? update(envelope.config) : update;
      if (updated !== null) nextDraft = updated;
    });
    render(<GeneralSettingsPage />);

    const group = screen.getByTestId("settings-page-general-follow-up-group");
    expect(within(group).getByTestId("settings-page-general-follow-up-steer")).toHaveAttribute(
      "aria-pressed",
      "true"
    );
    expect(within(group).getByTestId("settings-page-general-follow-up-queue")).toHaveAttribute(
      "aria-pressed",
      "false"
    );

    fireEvent.click(screen.getByTestId("settings-page-general-follow-up-queue"));
    expect(pageState.setDraft).toHaveBeenCalledTimes(1);
    expect(nextDraft.busy_input).toEqual({ default_mode: "queue" });
    expect(nextDraft.idle_release_after).toBe(envelope.config.idle_release_after);
  });

  it("renders the queue follow-up value and locks the control while saving", () => {
    pageState.draft = structuredClone(envelope.config);
    pageState.draft.busy_input = { default_mode: "queue" };
    pageState.isSaving = true;
    render(<GeneralSettingsPage />);

    expect(screen.getByTestId("settings-page-general-follow-up-queue")).toHaveAttribute(
      "aria-pressed",
      "true"
    );
    expect(screen.getByTestId("settings-page-general-follow-up-steer")).toBeDisabled();
  });

  it("reads the secret redaction switch and marks it restart-required", () => {
    render(<GeneralSettingsPage />);

    const section = screen.getByTestId("settings-page-general-redact");
    expect(within(section).getByText("Hide passwords and keys in logs")).toBeInTheDocument();
    // Restart is announced by the page-level restart notice, not a chip in the label.
    expect(within(section).queryByText("restart required")).toBeNull();
    expect(screen.getByTestId("settings-page-general-redact-enabled-switch")).toBeChecked();
  });

  it("toggles the secret redaction switch through the page draft handler", () => {
    let nextDraft: Envelope["config"] = envelope.config;
    pageState.setDraft.mockImplementation((update: SetStateAction<Envelope["config"] | null>) => {
      const updated = typeof update === "function" ? update(envelope.config) : update;
      if (updated !== null) nextDraft = updated;
    });
    render(<GeneralSettingsPage />);

    fireEvent.click(screen.getByTestId("settings-page-general-redact-enabled-switch"));
    expect(pageState.setDraft).toHaveBeenCalledTimes(1);
    expect(nextDraft.redact.enabled).toBe(false);
    expect(nextDraft.daemon).toEqual(envelope.config.daemon);
  });

  it("surfaces transport errors and retries the update query when refresh fails", () => {
    pageState.update = {
      data: null,
      isError: true,
      isLoading: false,
      isFetching: false,
      error: new Error("update refresh timed out"),
      refetch: vi.fn(),
    };

    render(<GeneralSettingsPage />);

    // The failure is stated once, on the version row that also carries Retry.
    expect(screen.getByTestId("settings-page-general-update-status")).toHaveTextContent(
      "update refresh timed out"
    );
    expect(screen.queryByTestId("settings-page-general-update-last-error")).toBeNull();
    fireEvent.click(screen.getByTestId("settings-page-general-update-retry"));
    expect(pageState.updateActions.check).toHaveBeenCalledTimes(1);
  });

  it("exposes the restart action button and triggers the restart mutation on click", () => {
    pageState.restart.isVisible = true;
    pageState.restart.isRestartRequired = true;
    render(<GeneralSettingsPage />);
    expect(screen.getByTestId("settings-page-general-restart-notice")).toBeInTheDocument();
    const button = screen.getByRole("button", { name: "Restart CompozyOS" });
    fireEvent.click(button);
    expect(pageState.restart.trigger).toHaveBeenCalledTimes(1);
  });

  it("Should wait for the provider catalog before rendering session defaults", () => {
    // Invariant (from the retired Defaults page): an unloaded provider list is never
    // presented as "no providers". Owning layer: General route.
    providersState.isLoading = true;
    render(<GeneralSettingsPage />);

    expect(screen.getByTestId("settings-page-general-loading")).toBeInTheDocument();
    expect(screen.queryByTestId("settings-page-general-session-defaults")).toBeNull();
  });

  it("Should surface a session-defaults failure and retry every General dependency", () => {
    providersState.error = new Error("Provider catalog unavailable");
    render(<GeneralSettingsPage />);

    expect(screen.getByText("Couldn't load General")).toBeInTheDocument();
    expect(screen.getByText("Provider catalog unavailable")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Try again" }));

    expect(pageState.handleRetry).toHaveBeenCalledOnce();
    expect(personaState.handleRetry).toHaveBeenCalledOnce();
    expect(providersState.refetch).toHaveBeenCalledOnce();
  });

  it("Should open with the session defaults as the first group", () => {
    pageState.update.data = settingsUpdateStagedFixture;
    personaState.profileName = "work";
    render(<GeneralSettingsPage />);

    const group = screen.getByTestId("settings-page-general-session-defaults");
    const frame = group.parentElement;
    expect.assert(frame !== null);
    expect(frame.querySelector("section")).toBe(group);
    expect(within(group).getByText("Agent")).toBeInTheDocument();
    expect(within(group).getByText("Model")).toBeInTheDocument();
    expect(within(group).getByRole("switch", { name: "Smooth streaming" })).toBeInTheDocument();
    expect(screen.getByTestId("settings-page-general-session-defaults-scope")).toHaveTextContent(
      "New sessions in the work profile start with these."
    );
  });

  it("Should edit the default agent and offer Automatic only once a provider is chosen", () => {
    const { rerender } = render(<GeneralSettingsPage />);
    expect(screen.getByTestId("settings-page-general-default-agent")).toHaveValue("general");
    expect(screen.getByTestId("settings-page-general-default-runtime-select")).toHaveTextContent(
      "Automatic (recommended)"
    );
    expect(screen.queryByTestId("settings-page-general-default-runtime-clear")).toBeNull();

    fireEvent.change(screen.getByTestId("settings-page-general-default-agent"), {
      target: { value: "reviewer" },
    });
    const [agentUpdate] = personaState.setDraft.mock.calls.at(-1) ?? [];
    expect.assert(typeof agentUpdate === "function");
    expect(agentUpdate({ agent: "general" })).toEqual({ agent: "reviewer" });

    personaState.draft = {
      agent: "general",
      provider: "claude",
      model: "claude-opus",
      reasoning_effort: "high",
    };
    rerender(<GeneralSettingsPage />);
    fireEvent.click(screen.getByRole("button", { name: "Use provider default" }));
    const [providerUpdate] = personaState.setDraft.mock.calls.at(-1) ?? [];
    expect.assert(typeof providerUpdate === "function");
    expect(providerUpdate(personaState.draft)).toEqual({ agent: "general", provider: "claude" });
    fireEvent.click(screen.getByTestId("settings-page-general-default-runtime-clear"));
    const [runtimeUpdate] = personaState.setDraft.mock.calls.at(-1) ?? [];
    expect.assert(typeof runtimeUpdate === "function");
    // Automatic drops the provider-bound model and reasoning with the provider.
    expect(runtimeUpdate(personaState.draft)).toEqual({ agent: "general" });
  });

  it("Should save only the dirty draft and reset both through one save bar", () => {
    personaState.isDirty = true;
    render(<GeneralSettingsPage />);

    fireEvent.click(screen.getByTestId("settings-page-general-save"));
    expect(personaState.handleSave).toHaveBeenCalledOnce();
    expect(pageState.handleSave).not.toHaveBeenCalled();

    fireEvent.click(screen.getByTestId("settings-page-general-reset"));
    expect(personaState.handleReset).toHaveBeenCalledOnce();
    expect(pageState.handleReset).toHaveBeenCalledOnce();
  });

  it("Should report a session-defaults save failure on the shared save bar", () => {
    personaState.isDirty = true;
    personaState.saveError = "defaults.model requires defaults.provider";
    render(<GeneralSettingsPage />);

    expect(screen.getByTestId("settings-page-general-save-bar")).toHaveTextContent(
      "defaults.model requires defaults.provider"
    );
  });

  it("wires the save bar buttons to the page-level handlers", () => {
    pageState.isDirty = true;
    render(<GeneralSettingsPage />);

    fireEvent.click(screen.getByTestId("settings-page-general-save"));
    expect(pageState.handleSave).toHaveBeenCalled();

    fireEvent.click(screen.getByTestId("settings-page-general-reset"));
    expect(pageState.handleReset).toHaveBeenCalled();
  });

  it("surfaces the last-applied label when the save bar has a success message", () => {
    pageState.isDirty = true;
    pageState.isSaving = true;
    pageState.lastAppliedLabel = "Saved · restart required to apply";
    const { rerender } = render(<GeneralSettingsPage />);

    pageState.isDirty = false;
    pageState.isSaving = false;
    rerender(<GeneralSettingsPage />);

    expect(screen.getByTestId("settings-page-general-save-message")).toHaveTextContent(
      "restart required"
    );
  });
});

// UT-040..UT-048 — the Updates section renders CompozyOS as one product: one row,
// one version, one Update action, never the runtime/app split the daemon keeps.
// Which snapshots fold into which product view is owned by
// systems/settings/lib/__tests__/update-presentation.test.ts; these assert what
// reaches the DOM, especially where an affordance must be absent rather than
// disabled (SD-007).
describe("GeneralSettingsPage — Updates section", () => {
  const row = () => screen.getByTestId("settings-page-general-update-status");
  const apply = () => screen.queryByTestId("settings-page-general-update-apply");

  it("Should show the last successful check and request a manual daemon refresh", () => {
    render(<GeneralSettingsPage />);
    const checked = screen.getByTestId("settings-page-general-updates").querySelector("time");
    expect(checked).toHaveAttribute("datetime", "2026-10-18T14:00:00Z");
    fireEvent.click(screen.getByRole("button", { name: "Check for updates" }));
    expect(pageState.updateActions.check).toHaveBeenCalledTimes(1);
  });

  it("Should avoid claiming an unchecked installation is up to date", () => {
    pageState.update.data = {
      ...settingsUpdateStatusFixture,
      runtime: { ...settingsUpdateStatusFixture.runtime, checked_at: undefined },
    };
    render(<GeneralSettingsPage />);
    expect(row()).toHaveTextContent("Not checked");
    expect(screen.getByText("No successful update check yet.")).toBeInTheDocument();
    expect(within(row()).queryByText("Up to date")).toBeNull();
  });

  it("UT-040: Should render one product row with one version, status, and release link", () => {
    render(<GeneralSettingsPage />);

    expect(row()).toHaveTextContent("CompozyOS");
    expect(row()).toHaveTextContent("0.5.0");
    expect(row()).toHaveTextContent("0.5.1");
    expect(within(row()).getByText("Update available")).toBeInTheDocument();
    expect(screen.getByTestId("settings-page-general-update-release")).toHaveAttribute(
      "href",
      "https://github.com/compozy/compozy/releases/tag/v0.5.1"
    );
    expect(apply()).toHaveTextContent("Update");
  });

  it("UT-041: Should never name the runtime or app parts of the product", () => {
    render(<GeneralSettingsPage />);

    const updates = screen.getByTestId("settings-page-general-updates");
    expect(within(updates).queryByText(/runtime/iu)).toBeNull();
    expect(within(updates).queryByText(/^App$/u)).toBeNull();
    expect(screen.queryByTestId("settings-page-general-update-track-runtime")).toBeNull();
    expect(screen.queryByTestId("settings-page-general-update-track-app")).toBeNull();
  });

  it("UT-042: Should show a managed install's command verbatim with no apply control in the DOM", () => {
    pageState.update.data = settingsUpdateManagedFixture;
    render(<GeneralSettingsPage />);

    expect(row()).toHaveTextContent("Update with your package manager to get the new version.");
    expect(screen.getByTestId("settings-page-general-update-recommendation")).toHaveTextContent(
      "brew upgrade compozy"
    );
    // Absent, not disabled — a dimmed button would still claim the capability.
    expect(apply()).toBeNull();
  });

  it("UT-043: Should apply in one click and then render the daemon's live progress", () => {
    const { rerender } = render(<GeneralSettingsPage />);

    fireEvent.click(screen.getByTestId("settings-page-general-update-apply"));
    expect(pageState.updateActions.apply).toHaveBeenCalledTimes(1);

    // Progress is not optimistic: it appears only once the polled read reports it.
    pageState.update.data = settingsUpdateApplyingFixture;
    rerender(<GeneralSettingsPage />);

    const progress = screen.getByTestId("settings-page-general-update-progress");
    expect(progress).toHaveTextContent("Updating · 62%");
    expect(progress).toHaveAttribute("data-phase", "install");
    expect(progress).toHaveAttribute("role", "status");
    expect(progress).toHaveAttribute("aria-live", "polite");
    expect(apply()).toBeNull();
  });

  it("Should replace the action with progress while its durable request is pending", () => {
    pageState.updateActions.isApplying = true;
    render(<GeneralSettingsPage />);

    expect(apply()).toBeNull();
    expect(screen.getByTestId("settings-page-general-update-progress")).toHaveTextContent(
      "Updating"
    );
  });

  it("UT-044: Should report a staged update as finishing on next launch with a cancel action", () => {
    pageState.update.data = settingsUpdateStagedFixture;
    render(<GeneralSettingsPage />);

    expect(row()).toHaveTextContent(
      "The update finishes the next time you open the CompozyOS app."
    );
    expect(within(row()).getByText("Staged")).toBeInTheDocument();

    fireEvent.click(screen.getByTestId("settings-page-general-update-cancel"));
    expect(pageState.updateActions.cancel).toHaveBeenCalledTimes(1);
  });

  it("UT-045: Should fall back to the checking view when the daemon goes away mid-apply, then show reconnected truth", () => {
    pageState.update = {
      data: null,
      isError: false,
      isLoading: true,
      isFetching: true,
      error: null,
      refetch: vi.fn(),
    };
    const { rerender } = render(<GeneralSettingsPage />);

    expect(screen.getByTestId("settings-page-general-update-checking")).toHaveTextContent(
      "Checking for a new version"
    );
    expect(row()).not.toHaveTextContent("Up to date");

    pageState.update = {
      data: settingsUpdateStatusFixture,
      isError: false,
      isLoading: false,
      isFetching: false,
      error: null,
      refetch: vi.fn(),
    };
    rerender(<GeneralSettingsPage />);

    expect(within(row()).getByText("Up to date")).toBeInTheDocument();
    expect(screen.queryByTestId("settings-page-general-update-checking")).not.toBeInTheDocument();
  });

  it("UT-046: Should keep the last known status and offer one retry when the refresh fails", () => {
    pageState.update = {
      data: settingsUpdateBothAvailableFixture,
      isError: true,
      isLoading: false,
      isFetching: false,
      error: new Error("daemon unreachable"),
      refetch: vi.fn(),
    };
    render(<GeneralSettingsPage />);

    expect(screen.getByTestId("settings-page-general-updates")).toHaveTextContent(
      "Showing the last known status. Check failed: daemon unreachable"
    );
    expect(screen.getByText("Refresh failed")).toBeInTheDocument();
    expect(row()).toHaveTextContent("0.5.1");
    expect(apply()).toBeNull();

    fireEvent.click(screen.getByRole("button", { name: "Check for updates" }));
    expect(pageState.updateActions.check).toHaveBeenCalledTimes(1);
  });

  it("UT-047: Should surface a blocked apply with its holder and claim no success", () => {
    pageState.update.data = settingsUpdateBlockedFixture;
    pageState.updateActions.result = {
      targets: ["runtime"],
      status: "blocked",
      message: "An update is already in progress. Retry after it completes.",
      holder: {
        pid: 4242,
        pid_start_time: "2026-08-20T14:02:10Z",
        surface: "cli",
        executor_generation: "gen-1",
        lease_expires_at: "2026-08-20T14:07:11Z",
      },
    };
    render(<GeneralSettingsPage />);

    const blockedRow = screen.getByTestId("settings-page-general-update-blocked");
    expect(blockedRow).toHaveTextContent("In use by the command line");
    expect(within(blockedRow).getByTitle("cli · PID 4242")).toBeInTheDocument();
    expect(row()).toHaveTextContent("An update is already running from the command line.");
    expect(screen.queryByText("Updated")).toBeNull();
    expect(apply()).toBeNull();
  });

  it("UT-048: Should keep the apply action tab-reachable and Enter-activatable", async () => {
    const user = userEvent.setup();
    pageState.update.data = settingsUpdateNoAppFixture;
    render(<GeneralSettingsPage />);

    const applyButton = screen.getByTestId("settings-page-general-update-apply");
    screen.getByTestId("settings-page-general-update-release").focus();
    await user.tab({ shift: true });
    expect(applyButton).toHaveFocus();

    await user.keyboard("{Enter}");
    expect(pageState.updateActions.apply).toHaveBeenCalledTimes(1);
  });

  it("Should acknowledge a canceled dormant update", () => {
    pageState.update.data = settingsUpdateStagedFixture;
    pageState.updateActions.cancelResult = {
      status: "canceled",
      operation_id: "op-7f3a2c",
      message: "Canceled dormant update operation; the update channel is free.",
      holder: null,
    };
    render(<GeneralSettingsPage />);

    const result = screen.getByTestId("settings-page-general-update-cancel-result");
    expect(result).toHaveTextContent("Update canceled");
    expect(result).toHaveTextContent("the update channel is free");
    expect(result).toHaveTextContent("Canceled");
  });

  it("Should explain a declined cancellation with the active holder", () => {
    pageState.update.data = settingsUpdateStagedFixture;
    pageState.updateActions.cancelResult = {
      status: "blocked",
      operation_id: "op-7f3a2c",
      message: "The update operation became active before cancellation.",
      holder: {
        pid: 5150,
        pid_start_time: "2026-08-20T14:02:10Z",
        surface: "daemon",
        executor_generation: "gen-2",
        lease_expires_at: "2026-08-20T14:07:11Z",
      },
    };
    render(<GeneralSettingsPage />);

    const result = screen.getByTestId("settings-page-general-update-cancel-result");
    expect(result).toHaveTextContent("Cancel declined");
    expect(result).toHaveTextContent("became active before cancellation");
    expect(result).toHaveTextContent("In use by CompozyOS");
    expect(within(result).getByTitle("daemon · PID 5150")).toBeInTheDocument();
  });

  it("Should report rollback truth as the restored version plus the daemon's error", () => {
    pageState.update.data = settingsUpdateRolledBackFixture;
    render(<GeneralSettingsPage />);

    expect(row()).toHaveTextContent("The update didn't finish, so CompozyOS went back to 0.5.0.");
    expect(screen.getByTestId("settings-page-general-update-last-error")).toHaveTextContent(
      "health check failed after swap"
    );
    // The failed projection is diagnostic-only until a fresh check reports available.
    expect(apply()).toBeNull();
  });
});
