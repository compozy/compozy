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
    daemon: {
      memory_report_interval: "5m",
      reload_timeouts: { bridges: "30s", mcp: "10s", providers: "5s" },
      socket: "/tmp/compozy.sock",
    },
    http: { host: "127.0.0.1", port: 2123 },
    limits: { max_concurrent_agents: 20 },
    permissions: { mode: "approve-all" as const },
    redact: { enabled: true },
    session_timeout: "0s",
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

vi.mock("@/systems/settings", async importOriginal => {
  const actual = await importOriginal<typeof import("@/systems/settings")>();
  return {
    ...actual,
    useSettingsGeneral: () => ({ data: envelope, isLoading: false, error: null }),
    useUpdateSettingsGeneral: () => mockMutation,
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
    expect(screen.getByTestId("settings-page-general-error")).toHaveTextContent("boom");
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
    expect(screen.getByText("Config file")).toBeInTheDocument();
    expect(screen.getByText("~/.compozy/config.toml")).toBeInTheDocument();
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

  it("reads the secret redaction switch and marks it restart-required", () => {
    render(<GeneralSettingsPage />);

    const section = screen.getByTestId("settings-page-general-redact");
    expect(within(section).getByText("Secret redaction heuristics")).toBeInTheDocument();
    expect(within(section).getByText("restart required")).toBeInTheDocument();
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

    expect(screen.getByTestId("settings-page-general-update-last-error")).toHaveTextContent(
      "update refresh timed out"
    );
    fireEvent.click(screen.getByTestId("settings-page-general-update-retry"));
    expect(pageState.update.refetch).toHaveBeenCalledTimes(1);
  });

  it("exposes the restart action button and triggers the restart mutation on click", () => {
    pageState.restart.isVisible = true;
    pageState.restart.isRestartRequired = true;
    render(<GeneralSettingsPage />);
    const button = screen.getByRole("button", { name: "Restart CompozyOS" });
    fireEvent.click(button);
    expect(pageState.restart.trigger).toHaveBeenCalledTimes(1);
  });

  it("renders the restart banner once the restart banner state reports visible", () => {
    pageState.restart.isVisible = true;
    pageState.restart.isRestartRequired = true;
    render(<GeneralSettingsPage />);
    expect(screen.getByTestId("settings-page-general-restart-notice")).toBeInTheDocument();
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

// UT-040..UT-048 — the Updates section renders the daemon's two-track projection
// and nothing else. Which snapshots produce which track model is owned by
// systems/settings/lib/__tests__/update-presentation.test.ts; these assert what
// reaches the DOM, especially where an affordance must be absent rather than
// disabled (SD-007).
describe("GeneralSettingsPage — Updates section", () => {
  const track = (target: "runtime" | "app") =>
    screen.getByTestId(`settings-page-general-update-track-${target}`);
  const apply = () => screen.queryByTestId("settings-page-general-update-apply");

  it("UT-040: Should render both tracks with their own versions, status, and release link", () => {
    render(<GeneralSettingsPage />);

    expect(track("runtime")).toHaveTextContent("0.5.0");
    expect(track("runtime")).toHaveTextContent("0.5.1");
    expect(within(track("runtime")).getByText("Update available")).toBeInTheDocument();
    expect(track("app")).toHaveTextContent("0.5.0");
    expect(within(track("app")).getByText("Update available")).toBeInTheDocument();
    expect(screen.getByTestId("settings-page-general-update-release-runtime")).toHaveAttribute(
      "href",
      "https://github.com/compozy/compozy/releases/tag/v0.5.1"
    );
    expect(screen.getByTestId("settings-page-general-update-release-app")).toBeInTheDocument();
    expect(apply()).toBeInTheDocument();
  });

  it("UT-041: Should render a single-track section when the host has no desktop app", () => {
    pageState.update.data = settingsUpdateNoAppFixture;
    render(<GeneralSettingsPage />);

    expect(track("runtime")).toBeInTheDocument();
    expect(screen.queryByTestId("settings-page-general-update-track-app")).toBeNull();
  });

  it("UT-042: Should show a managed runtime's command verbatim with no apply control in the DOM", () => {
    pageState.update.data = settingsUpdateManagedFixture;
    render(<GeneralSettingsPage />);

    expect(screen.getByTestId("settings-page-general-update-recommendation")).toHaveTextContent(
      "brew upgrade compozy"
    );
    expect(track("runtime")).toHaveTextContent(
      "CompozyOS 0.5.1 is available. Upgrade with your package manager."
    );
    // Absent, not disabled — a dimmed button would still claim the capability.
    expect(apply()).toBeNull();
  });

  it("UT-043: Should apply the clicked track and then render the daemon's staged phase", () => {
    const { rerender } = render(<GeneralSettingsPage />);

    fireEvent.click(screen.getByTestId("settings-page-general-update-apply"));
    expect(pageState.updateActions.apply).toHaveBeenCalledWith(["runtime", "app"]);

    // Progress is not optimistic: it appears only once the polled read reports it.
    pageState.update.data = settingsUpdateApplyingFixture;
    rerender(<GeneralSettingsPage />);

    const progress = screen.getByTestId("settings-page-general-update-progress-runtime");
    expect(progress).toHaveTextContent("install · 62%");
    expect(progress).toHaveAttribute("role", "status");
    expect(progress).toHaveAttribute("aria-live", "polite");
    expect(apply()).toBeNull();
  });

  it("Should disable the shared action while its durable request is pending", () => {
    pageState.updateActions.isApplying = true;
    render(<GeneralSettingsPage />);

    expect(screen.getByTestId("settings-page-general-update-apply")).toBeDisabled();
  });

  it("UT-044: Should report a staged app with the daemon's next-launch message and a cancel action", () => {
    pageState.update.data = settingsUpdateStagedFixture;
    render(<GeneralSettingsPage />);

    expect(track("app")).toHaveTextContent("CompozyOS app update staged; applies on next launch.");
    expect(within(track("app")).getByText("Staged")).toBeInTheDocument();
    expect(track("runtime")).toHaveTextContent(
      "Updated CompozyOS runtime to 0.5.1 and restarted the daemon."
    );

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

    expect(screen.getByTestId("settings-page-general-update-status")).toHaveTextContent("Checking");

    pageState.update = {
      data: settingsUpdateStatusFixture,
      isError: false,
      isLoading: false,
      isFetching: false,
      error: null,
      refetch: vi.fn(),
    };
    rerender(<GeneralSettingsPage />);

    expect(within(track("runtime")).getByText("Up to date")).toBeInTheDocument();
    expect(screen.queryByTestId("settings-page-general-update-status")).toBeNull();
  });

  it("UT-046: Should keep the last known tracks and offer one retry when the refresh fails", () => {
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
      "Showing the last known status. Refresh failed: daemon unreachable"
    );
    expect(screen.getByText("Refresh failed")).toBeInTheDocument();
    // The rows keep their last truth rather than restating the failure per track.
    expect(track("runtime")).toHaveTextContent("0.5.1");
    expect(apply()).toBeNull();

    fireEvent.click(screen.getByTestId("settings-page-general-update-retry"));
    expect(pageState.update.refetch).toHaveBeenCalledTimes(1);
  });

  it("UT-047: Should surface a blocked apply with its holder and claim no success", () => {
    pageState.update.data = settingsUpdateBlockedFixture;
    pageState.updateActions.result = {
      targets: ["runtime"],
      status: "blocked",
      message: "A runtime update is already in progress. Retry after it completes.",
      holder: {
        pid: 4242,
        pid_start_time: "2026-08-20T14:02:10Z",
        surface: "cli",
        executor_generation: "gen-1",
        lease_expires_at: "2026-08-20T14:07:11Z",
      },
    };
    render(<GeneralSettingsPage />);

    expect(screen.getByTestId("settings-page-general-update-blocked")).toHaveTextContent(
      "cli · PID 4242"
    );
    expect(within(track("runtime")).getByText("Blocked")).toBeInTheDocument();
    expect(screen.queryByText("Updated")).toBeNull();
    expect(apply()).toBeNull();
  });

  it("UT-048: Should keep the shared apply action tab-reachable and Enter-activatable", async () => {
    const user = userEvent.setup();
    pageState.update.data = settingsUpdateNoAppFixture;
    render(<GeneralSettingsPage />);

    const applyButton = screen.getByTestId("settings-page-general-update-apply");
    screen.getByTestId("settings-page-general-update-release-runtime").focus();
    await user.tab({ shift: true });
    expect(applyButton).toHaveFocus();

    await user.keyboard("{Enter}");
    expect(pageState.updateActions.apply).toHaveBeenCalledWith(["runtime"]);
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
    expect(result).toHaveTextContent("daemon · PID 5150");
  });

  it("Should report rollback truth as the restored version plus the daemon's error", () => {
    pageState.update.data = settingsUpdateRolledBackFixture;
    render(<GeneralSettingsPage />);

    expect(screen.getByTestId("settings-page-general-update-rollback")).toHaveTextContent("0.5.0");
    expect(screen.getByTestId("settings-page-general-update-last-error")).toHaveTextContent(
      "health check failed after swap"
    );
    // The failed projection is diagnostic-only until a fresh check reports available.
    expect(apply()).toBeNull();
  });
});
