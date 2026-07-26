import { fireEvent, screen, within } from "@testing-library/react";
import { renderWithTopbar as render } from "@/test/render-with-topbar";
import { createElement, type ReactNode, type SetStateAction } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

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
  isPolling: boolean;
  isSuccessful: boolean;
  isFailed: boolean;
  operationId: string | null;
  status: string | null;
  failureReason?: string;
  activeSessionCount: number;
  trigger: ReturnType<typeof vi.fn>;
  isTriggerPending: boolean;
  triggerError: unknown;
  dismiss: ReturnType<typeof vi.fn>;
};

type UpdateStatus = {
  supported: boolean;
  managed: boolean;
  install_method: string;
  current_version: string;
  latest_version?: string;
  available: boolean;
  status: string;
  recommendation?: string;
  release_url?: string;
  checked_at?: string | null;
  last_error?: string;
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
  scope: "global" as const,
  available_scopes: ["global"] as const,
  actions: {
    restart: { available: true, behavior: "action_trigger" as const, name: "restart" },
  },
  config: {
    daemon: {
      memory_report_interval: "5m",
      reload_timeouts: { bridges: "30s", mcp: "10s", providers: "5s" },
      socket: "/tmp/agh.sock",
    },
    defaults: { agent: "general", provider: "claude", sandbox: "local" },
    http: { host: "127.0.0.1", port: 2123 },
    limits: { max_concurrent_agents: 20 },
    permissions: { mode: "approve-all" as const },
    redact: { enabled: true },
    session_timeout: "0s",
  },
  config_paths: {
    daemon_info: "/tmp/daemon.json",
    global_config: "~/.agh/config.toml",
    global_mcp_sidecar: "~/.agh/mcp.json",
    home_dir: "~/.agh",
    log_file: "~/.agh/agh.log",
  },
  runtime: {
    active_agents: 7,
    active_sessions: 4,
    available: true,
    http_host: "127.0.0.1",
    http_port: 2123,
    socket: "~/.agh/daemon.sock",
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
  applyRecords: ApplyRecordsQuery;
  handleReload: ReturnType<typeof vi.fn>;
  isReloading: boolean;
  reloadError: string | null;
  reloadResult: null;
};

const restartBanner: RestartBanner = {
  isVisible: false,
  isRestartRequired: false,
  isPolling: false,
  isSuccessful: false,
  isFailed: false,
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
    useSettingsProviders: () => ({ data: { providers: [{ name: "claude" }] } }),
    useSettingsSandboxes: () => ({ data: { sandboxes: [{ name: "local" }] } }),
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
      data: {
        supported: true,
        managed: false,
        install_method: "direct-binary",
        current_version: "v1.0.0",
        latest_version: "v1.1.0",
        available: true,
        status: "available",
        recommendation: "Run `agh update`.",
        release_url: "https://github.com/compozy/agh/releases/tag/v1.1.0",
        checked_at: "2026-05-03T19:00:00Z",
      },
      isLoading: false,
      isError: false,
      isFetching: false,
      error: null,
      refetch: vi.fn(),
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

  it("renders runtime, defaults, permissions, and config path from the section envelope", () => {
    render(<GeneralSettingsPage />);

    expect(screen.getByTestId("settings-page-general-subhead")).toHaveTextContent(
      "4 active sessions"
    );
    expect(screen.getByTestId("settings-page-general-subhead")).toHaveTextContent(
      "7 agents working"
    );
    expect(screen.getByTestId("settings-page-general-default-agent-input")).toHaveValue("general");
    expect(screen.getByTestId("settings-page-general-default-provider-input")).toHaveValue(
      "claude"
    );
    expect(screen.getByTestId("settings-page-general-permissions-group")).toBeInTheDocument();
    expect(screen.getByRole("radio", { name: /Allow everything/ })).toBeChecked();
    expect(screen.getByTestId("settings-page-general-update-status")).toHaveTextContent(
      "available"
    );
    expect(screen.getByTestId("settings-page-general-update-recommendation")).toHaveTextContent(
      "Run `agh update`."
    );
    fireEvent.click(screen.getByRole("button", { name: "Advanced" }));
    expect(screen.getByText("Config file")).toBeInTheDocument();
    expect(screen.getByText("~/.agh/config.toml")).toBeInTheDocument();
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

  it("renders manual guidance and the last refresh error for unsupported update snapshots", () => {
    pageState.update.data = {
      supported: false,
      managed: false,
      install_method: "direct-binary",
      current_version: "v1.0.0",
      latest_version: "v1.1.0",
      available: true,
      status: "unsupported",
      recommendation:
        "Download the latest AGH Windows release archive and replace `agh.exe` manually.",
      release_url: "https://github.com/compozy/agh/releases/tag/v1.1.0",
      checked_at: "2026-05-03T19:00:00Z",
      last_error: "cached refresh failed",
    };

    render(<GeneralSettingsPage />);

    expect(screen.getByTestId("settings-page-general-update-status")).toHaveTextContent(
      /unsupported/i
    );
    expect(screen.getByTestId("settings-page-general-update-recommendation")).toHaveTextContent(
      "replace `agh.exe` manually"
    );
    expect(screen.getByTestId("settings-page-general-update-last-error")).toHaveTextContent(
      "cached refresh failed"
    );
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
    const button = screen.getByRole("button", { name: "Restart daemon" });
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
