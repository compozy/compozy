import { fireEvent, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { renderWithTopbar as render } from "@/test/render-with-topbar";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { SettingsSandboxEntry } from "@/systems/settings";
import { emptySandboxDraft, SandboxPage } from "@/systems/sandbox";

type RestartNotice = {
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

const localEnv: SettingsSandboxEntry = {
  name: "local",
  workspace_usage_count: 3,
  profile: {
    backend: "local",
    sync_mode: "none",
    persistence: "transient",
    runtime_root: "~",
  },
  source_metadata: {
    available_targets: ["global-config"],
    effective_source: { kind: "global-config", scope: "global" },
  },
};

const builtinEnv: SettingsSandboxEntry = {
  name: "builtin-local",
  workspace_usage_count: 0,
  profile: {
    backend: "local",
    sync_mode: "none",
    persistence: "transient",
    runtime_root: "~",
  },
  source_metadata: {
    available_targets: ["global-config"],
    effective_source: { kind: "builtin-provider", scope: "global" },
  },
};

type PageState = {
  isLoading: boolean;
  isRefetching: boolean;
  queryError: string | null;
  error: Error | null;
  envelope: { sandboxes: SettingsSandboxEntry[] } | null;
  sandboxes: SettingsSandboxEntry[];
  filtered: SettingsSandboxEntry[];
  counts: { total: number; totalWorkspaces: number };
  query: string;
  backend: "all" | "local" | "daytona" | "e2b";
  persistence: "all" | "transient" | "reuse" | "archive";
  view: "rows" | "cards";
  hasActiveFilters: boolean;
  setQuery: ReturnType<typeof vi.fn>;
  setBackend: ReturnType<typeof vi.fn>;
  setPersistence: ReturnType<typeof vi.fn>;
  setView: ReturnType<typeof vi.fn>;
  clearFilters: ReturnType<typeof vi.fn>;
  selectedEntry: SettingsSandboxEntry | null;
  openInspect: ReturnType<typeof vi.fn>;
  closeInspect: ReturnType<typeof vi.fn>;
  refetch: ReturnType<typeof vi.fn>;
  restart: RestartNotice;
  editor: { mode: "closed" | "create" | "edit"; [key: string]: unknown };
  editorIsValid: boolean;
  editorError: string | null;
  editorWarnings: string[] | undefined;
  editorIsSaving: boolean;
  openCreate: ReturnType<typeof vi.fn>;
  openEdit: ReturnType<typeof vi.fn>;
  closeEditor: ReturnType<typeof vi.fn>;
  updateDraft: ReturnType<typeof vi.fn>;
  saveEditor: ReturnType<typeof vi.fn>;
  deleteTarget: { mode: "closed" | "open"; entry?: SettingsSandboxEntry };
  deleteError: string | null;
  deleteIsPending: boolean;
  openDelete: ReturnType<typeof vi.fn>;
  closeDelete: ReturnType<typeof vi.fn>;
  confirmDelete: ReturnType<typeof vi.fn>;
  lastAction: null | {
    kind: "saved" | "deleted";
    name: string;
    result: { restart_required: boolean };
    usageCount?: number;
  };
  dismissLastAction: ReturnType<typeof vi.fn>;
};

const restartNotice: RestartNotice = {
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

let pageState: PageState;

vi.mock("@/systems/sandbox/hooks/use-sandbox-page", async importOriginal => {
  const actual = await importOriginal<typeof import("@/systems/sandbox/hooks/use-sandbox-page")>();
  return {
    ...actual,
    useSandboxPage: () => pageState,
  };
});

function makeState(overrides: Partial<PageState> = {}): PageState {
  return {
    isLoading: false,
    isRefetching: false,
    queryError: null,
    error: null,
    envelope: { sandboxes: [localEnv, builtinEnv] },
    sandboxes: [localEnv, builtinEnv],
    filtered: [localEnv, builtinEnv],
    counts: { total: 2, totalWorkspaces: 3 },
    query: "",
    backend: "all",
    persistence: "all",
    view: "rows",
    hasActiveFilters: false,
    setQuery: vi.fn(),
    setBackend: vi.fn(),
    setPersistence: vi.fn(),
    setView: vi.fn(),
    clearFilters: vi.fn(),
    selectedEntry: null,
    openInspect: vi.fn(),
    closeInspect: vi.fn(),
    refetch: vi.fn(),
    restart: { ...restartNotice, trigger: vi.fn(), dismiss: vi.fn() },
    editor: { mode: "closed" },
    editorIsValid: false,
    editorError: null,
    editorWarnings: undefined,
    editorIsSaving: false,
    openCreate: vi.fn(),
    openEdit: vi.fn(),
    closeEditor: vi.fn(),
    updateDraft: vi.fn(),
    saveEditor: vi.fn(),
    deleteTarget: { mode: "closed" },
    deleteError: null,
    deleteIsPending: false,
    openDelete: vi.fn(),
    closeDelete: vi.fn(),
    confirmDelete: vi.fn(),
    lastAction: null,
    dismissLastAction: vi.fn(),
    ...overrides,
  };
}

beforeEach(() => {
  pageState = makeState();
});

describe("SandboxPage", () => {
  it("renders loading state", () => {
    pageState = makeState({ isLoading: true, envelope: null, sandboxes: [], filtered: [] });
    render(<SandboxPage />);
    expect(screen.getByRole("status", { name: "Loading sandbox profiles" })).toBe(
      screen.getByTestId("sandbox-page-loading")
    );
  });

  it("renders error state with the error message", () => {
    pageState = makeState({
      envelope: null,
      sandboxes: [],
      filtered: [],
      queryError: "nope",
      error: new Error("nope"),
    });
    render(<SandboxPage />);
    expect(screen.getByTestId("sandbox-page-error")).toHaveTextContent("nope");
  });

  it("renders the profile list with workspace usage counts", () => {
    render(<SandboxPage />);
    expect(screen.getByTestId("sandbox-page-total")).toHaveTextContent("2 profiles");
    expect(screen.getByTestId("sandbox-page-workspaces")).toHaveTextContent(
      "3 workspace references"
    );
    expect(screen.getByTestId("sandbox-page-card-local-usage")).toHaveTextContent("3");
    expect(screen.getByTestId("sandbox-page-card-local-source")).toHaveTextContent("CONFIG");
  });

  it("shows the @agh/ui Empty card when no sandboxes exist", () => {
    pageState = makeState({
      sandboxes: [],
      filtered: [],
      envelope: { sandboxes: [] },
      counts: { total: 0, totalWorkspaces: 0 },
    });
    render(<SandboxPage />);
    const empty = screen.getByTestId("sandbox-page-empty");
    expect(empty).toBeInTheDocument();
    expect(empty).toHaveAttribute("data-slot", "empty");
    expect(empty).toHaveTextContent("No sandbox profiles defined");
  });

  it("wires create, edit, delete, and inspect controls", () => {
    render(<SandboxPage />);
    fireEvent.click(screen.getByTestId("sandbox-page-create"));
    expect(pageState.openCreate).toHaveBeenCalled();

    fireEvent.click(screen.getByTestId("sandbox-page-card-local-edit"));
    expect(pageState.openEdit).toHaveBeenCalledWith(localEnv);

    fireEvent.click(screen.getByTestId("sandbox-page-card-local-delete"));
    expect(pageState.openDelete).toHaveBeenCalledWith(localEnv);

    fireEvent.click(screen.getByTestId("sandbox-page-select-local"));
    expect(pageState.openInspect).toHaveBeenCalledWith(localEnv);
  });

  it("Should preserve delete controls in the card view", () => {
    pageState = makeState({ view: "cards" });
    render(<SandboxPage />);

    fireEvent.click(screen.getByTestId("sandbox-page-card-local-delete"));

    expect(pageState.openDelete).toHaveBeenCalledWith(localEnv);
    expect(screen.getByTestId("sandbox-page-card-builtin-local-delete")).toBeDisabled();
  });

  it("disables delete for builtin sandboxes", () => {
    render(<SandboxPage />);
    expect(screen.getByTestId("sandbox-page-card-builtin-local-delete")).toBeDisabled();
  });

  it("renders the create dialog with an editable name and the selected backend", () => {
    pageState = makeState({
      editor: { mode: "create", draft: emptySandboxDraft() },
    });
    render(<SandboxPage />);
    expect(screen.getByTestId("settings-sandbox-editor-title")).toHaveTextContent(
      "Create sandbox profile"
    );
    expect(screen.queryByTestId("settings-modal-editor-title")).toBeNull();
    expect(screen.getByTestId("sandbox-editor-name")).not.toBeDisabled();
    expect(screen.getByTestId("sandbox-editor-backend-local")).toHaveAttribute(
      "aria-checked",
      "true"
    );
  });

  it("mounts the sandbox shell under ListingPage", () => {
    render(<SandboxPage />);
    const shell = screen.getByTestId("sandbox-shell");
    expect(shell).toBeInTheDocument();
    expect(screen.getByTestId("sandbox-page-sec-note")).toBeInTheDocument();
    expect(screen.getByTestId("sandbox-topbar-actions")).toBeInTheDocument();
  });

  it("Should wire inspection-sheet close, edit, and delete actions to the selected profile", () => {
    pageState = makeState({ selectedEntry: localEnv });
    render(<SandboxPage />);
    expect(screen.getByTestId("sandbox-profile-sheet")).toBeInTheDocument();
    expect(screen.getByTestId("sandbox-profile-sheet-title")).toHaveTextContent("local");
    expect(screen.getByTestId("sandbox-profile-sheet-foot")).toHaveTextContent(
      "agh config get sandboxes.local.backend"
    );

    fireEvent.click(screen.getByTestId("sandbox-profile-sheet-edit"));
    expect(pageState.openEdit).toHaveBeenCalledWith(localEnv);

    fireEvent.click(screen.getByTestId("sandbox-profile-sheet-delete"));
    expect(pageState.openDelete).toHaveBeenCalledWith(localEnv);

    fireEvent.click(screen.getByTestId("sandbox-profile-sheet-close"));
    expect(pageState.closeInspect).toHaveBeenCalledTimes(1);
  });

  it("hydrates nested profile keys into the Advanced tier instead of hiding them", async () => {
    const user = userEvent.setup();
    pageState = makeState({
      editor: {
        mode: "edit",
        name: "local",
        draft: {
          ...emptySandboxDraft(),
          name: "local",
          network: { allow_outbound: true, allow_list: ["api.github.com"] },
          secretEnv: [{ key: "GITHUB_TOKEN", value: "vault:sandbox/github" }],
        },
        entry: localEnv,
      },
    });
    render(<SandboxPage />);

    await user.click(screen.getByTestId("sandbox-editor-mode-advanced"));

    expect(screen.getByTestId("sandbox-editor-allow-outbound")).toHaveAttribute(
      "aria-checked",
      "true"
    );
    expect(screen.getByTestId("sandbox-editor-allow-list")).toHaveValue("api.github.com");
    expect(screen.getByTestId("sandbox-editor-secret-env")).toHaveValue(
      "GITHUB_TOKEN=vault:sandbox/github"
    );
  });

  it("blocks the save when a secret_env row carries a literal value instead of a reference", async () => {
    const user = userEvent.setup();
    pageState = makeState({
      editor: {
        mode: "edit",
        name: "local",
        draft: {
          ...emptySandboxDraft(),
          name: "local",
          secretEnv: [{ key: "GITHUB_TOKEN", value: "ghp_liveTokenValue" }],
        },
        entry: localEnv,
      },
    });
    render(<SandboxPage />);

    expect(screen.getByTestId("settings-sandbox-editor-save")).toBeDisabled();

    await user.click(screen.getByTestId("sandbox-editor-mode-advanced"));

    expect(screen.getByTestId("sandbox-editor-secret-env-error")).toHaveTextContent(
      "never a literal value"
    );
    expect(screen.getByTestId("settings-sandbox-editor-save")).toBeDisabled();
  });

  it("surfaces usage warnings in the delete dialog when a profile is referenced", () => {
    pageState = makeState({ deleteTarget: { mode: "open", entry: localEnv } });
    render(<SandboxPage />);
    expect(screen.getByTestId("sandbox-delete-usage")).toHaveTextContent(
      "3 workspaces currently reference this profile"
    );
  });

  it("shows the last action banner after save and delete via @agh/ui Alert", () => {
    pageState = makeState({
      lastAction: {
        kind: "deleted",
        name: "local",
        result: { restart_required: true },
        usageCount: 3,
      },
    });
    render(<SandboxPage />);
    const banner = screen.getByTestId("sandbox-page-action-result");
    expect(banner).toHaveAttribute("data-slot", "alert");
    expect(banner).toHaveTextContent('Deleted "local"');
    expect(banner).toHaveTextContent("3 workspaces affected");
    expect(screen.getByRole("button", { name: "Dismiss result" })).toBeInTheDocument();
  });
});
