import { fireEvent, screen } from "@testing-library/react";
import { renderWithTopbar as render } from "@/test/render-with-topbar";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type {
  VaultDeleteState,
  VaultDraft,
  VaultEditorState,
  VaultLastAction,
  VaultNamespaceFilter,
} from "@/systems/vault/hooks/use-vault-page";
import type { VaultListFilter, VaultSecret } from "@/systems/vault";
import type { ListingViewMode } from "@agh/ui";

import { VaultPage } from "@/systems/vault";

type PageState = {
  counts: { total: number; sessions: number; providers: number };
  deleteError: string | null;
  deleteIsPending: boolean;
  deleteTarget: VaultDeleteState;
  dismissLastAction: ReturnType<typeof vi.fn>;
  editor: VaultEditorState;
  editorError: string | null;
  editorIsSaving: boolean;
  editorIsValid: boolean;
  editorRefExists: boolean;
  filter: VaultListFilter;
  isLoading: boolean;
  isRefetching: boolean;
  lastAction: VaultLastAction | null;
  namespace: VaultNamespaceFilter;
  prefix: string;
  queryError: string | null;
  refetch: ReturnType<typeof vi.fn>;
  replaceError: string | null;
  replaceIsPending: boolean;
  replaceIsValid: boolean;
  replaceSecret: ReturnType<typeof vi.fn>;
  replaceValue: string;
  secrets: VaultSecret[];
  selectedSecret: VaultSecret | null;
  setNamespace: ReturnType<typeof vi.fn>;
  setPrefix: ReturnType<typeof vi.fn>;
  setReplaceValue: ReturnType<typeof vi.fn>;
  setView: ReturnType<typeof vi.fn>;
  closeDelete: ReturnType<typeof vi.fn>;
  closeEditor: ReturnType<typeof vi.fn>;
  closeInspect: ReturnType<typeof vi.fn>;
  confirmDelete: ReturnType<typeof vi.fn>;
  openCreate: ReturnType<typeof vi.fn>;
  openDelete: ReturnType<typeof vi.fn>;
  openInspect: ReturnType<typeof vi.fn>;
  saveEditor: ReturnType<typeof vi.fn>;
  updateDraft: ReturnType<typeof vi.fn>;
  view: ListingViewMode;
};

const { mockUseVaultPage } = vi.hoisted(() => ({
  mockUseVaultPage: vi.fn(),
}));

vi.mock("@/systems/vault/hooks/use-vault-page", async importOriginal => {
  const actual = await importOriginal<typeof import("@/systems/vault/hooks/use-vault-page")>();
  return {
    ...actual,
    useVaultPage: (...args: unknown[]) => mockUseVaultPage(...args),
  };
});

const sessionSecret: VaultSecret = {
  ref: "vault:sessions/sess_123/github-token",
  namespace: "sessions",
  kind: "token",
  present: true,
  created_at: "2026-05-02T10:00:00Z",
  updated_at: "2026-05-02T10:00:00Z",
};

function makeState(overrides: Partial<PageState> = {}): PageState {
  return {
    counts: { total: 1, sessions: 1, providers: 0 },
    deleteError: null,
    deleteIsPending: false,
    deleteTarget: { mode: "closed" },
    dismissLastAction: vi.fn(),
    editor: { mode: "closed" },
    editorError: null,
    editorIsSaving: false,
    editorIsValid: false,
    editorRefExists: false,
    filter: {},
    isLoading: false,
    isRefetching: false,
    lastAction: null,
    namespace: "all",
    prefix: "",
    queryError: null,
    refetch: vi.fn(),
    replaceError: null,
    replaceIsPending: false,
    replaceIsValid: false,
    replaceSecret: vi.fn(),
    replaceValue: "",
    secrets: [sessionSecret],
    selectedSecret: null,
    setNamespace: vi.fn(),
    setPrefix: vi.fn(),
    setReplaceValue: vi.fn(),
    setView: vi.fn(),
    closeDelete: vi.fn(),
    closeEditor: vi.fn(),
    closeInspect: vi.fn(),
    confirmDelete: vi.fn(),
    openCreate: vi.fn(),
    openDelete: vi.fn(),
    openInspect: vi.fn(),
    saveEditor: vi.fn(),
    updateDraft: vi.fn(),
    view: "rows",
    ...overrides,
  };
}

beforeEach(() => {
  mockUseVaultPage.mockReturnValue(makeState());
});

describe("VaultPage", () => {
  it("renders vault counts, filters, and redacted metadata rows", () => {
    render(<VaultPage />);

    expect(screen.getByTestId("vault-page-count")).toHaveTextContent("1");
    expect(screen.getByTestId("vault-page-sessions")).toHaveTextContent("1 session-scoped");
    expect(screen.getByTestId("vault-list-filters-add")).toBeInTheDocument();
    expect(screen.getByTestId("vault-page-sec-note")).toHaveTextContent(
      "1 redacted metadata entry"
    );
    expect(screen.getByTestId("vault-page-list")).toHaveTextContent(sessionSecret.ref);
    expect(screen.getByTestId("vault-page-list")).not.toHaveTextContent("super-secret-token");
    expect(screen.getByTestId("listing-view-toggle")).toBeInTheDocument();
  });

  it("forwards prefix search changes to the page state hook", () => {
    const setPrefix = vi.fn();
    mockUseVaultPage.mockReturnValue(makeState({ setPrefix }));

    render(<VaultPage />);

    fireEvent.change(screen.getByTestId("vault-page-prefix"), {
      target: { value: "vault:sessions/sess_123/" },
    });

    expect(setPrefix).toHaveBeenCalledWith("vault:sessions/sess_123/");
  });

  it("opens the create action and renders the write-only editor state", () => {
    const draft: VaultDraft = {
      ref: "vault:sessions/sess_123/github-token",
      kind: "token",
      secretValue: "super-secret-token",
      overwriteConfirmed: false,
    };
    const openCreate = vi.fn();
    mockUseVaultPage.mockReturnValue(
      makeState({
        editor: { mode: "create", draft },
        editorIsValid: true,
        openCreate,
      })
    );

    const { container } = render(<VaultPage />);

    fireEvent.click(screen.getByTestId("vault-page-create"));
    expect(openCreate).toHaveBeenCalled();
    expect(screen.getByLabelText("Secret value")).toHaveAttribute("type", "password");
    expect(container.textContent).not.toContain("super-secret-token");
  });

  it("gates the write behind an explicit confirmation when the ref already exists", () => {
    const draft: VaultDraft = {
      ref: "vault:sessions/sess_123/github-token",
      kind: "token",
      secretValue: "super-secret-token",
      overwriteConfirmed: false,
    };
    mockUseVaultPage.mockReturnValue(
      makeState({
        editor: { mode: "create", draft },
        editorIsValid: false,
        editorRefExists: true,
      })
    );

    render(<VaultPage />);

    expect(screen.getByTestId("settings-vault-editor-overwrite")).toHaveTextContent(
      /rotates its write-only value/i
    );
    expect(screen.getByTestId("settings-vault-editor-overwrite-confirm")).toHaveAttribute(
      "aria-checked",
      "false"
    );
    expect(screen.getByTestId("settings-vault-editor-save")).toBeDisabled();
  });

  it("omits the overwrite notice for a new ref", () => {
    const draft: VaultDraft = {
      ref: "vault:sessions/sess_999/new-token",
      kind: "token",
      secretValue: "fresh",
      overwriteConfirmed: false,
    };
    mockUseVaultPage.mockReturnValue(
      makeState({
        editor: { mode: "create", draft },
        editorIsValid: true,
        editorRefExists: false,
      })
    );

    render(<VaultPage />);

    expect(screen.queryByTestId("settings-vault-editor-overwrite")).not.toBeInTheDocument();
    expect(screen.getByTestId("settings-vault-editor-save")).toBeEnabled();
  });

  it("opens the inspect sheet when a row is selected", () => {
    const openInspect = vi.fn();
    mockUseVaultPage.mockReturnValue(
      makeState({
        openInspect,
        selectedSecret: sessionSecret,
      })
    );

    render(<VaultPage />);

    expect(screen.getByTestId("vault-secret-sheet")).toBeInTheDocument();
    expect(screen.getByTestId("vault-secret-sheet-ref")).toHaveTextContent(sessionSecret.ref);
    expect(screen.getByTestId("vault-secret-sheet-value")).toHaveTextContent("write-only");
    expect(screen.queryByText("super-secret-token")).toBeNull();

    fireEvent.click(screen.getByTestId(`vault-secrets-select-${sessionSecret.ref}`));
    expect(openInspect).toHaveBeenCalledWith(sessionSecret);
  });

  it("disables deletion while the selected secret replacement is pending", () => {
    const openDelete = vi.fn();
    mockUseVaultPage.mockReturnValue(
      makeState({
        openDelete,
        replaceIsPending: true,
        selectedSecret: sessionSecret,
      })
    );

    render(<VaultPage />);

    const deleteButton = screen.getByTestId("vault-secret-sheet-delete");
    expect(deleteButton).toBeDisabled();
    fireEvent.click(deleteButton);
    expect(openDelete).not.toHaveBeenCalled();
  });

  it("confirms delete against the selected vault ref", () => {
    const confirmDelete = vi.fn();
    mockUseVaultPage.mockReturnValue(
      makeState({
        deleteTarget: { mode: "open", secret: sessionSecret },
        confirmDelete,
      })
    );

    render(<VaultPage />);

    expect(screen.getByTestId("settings-vault-delete-description")).toHaveTextContent(
      sessionSecret.ref
    );
    fireEvent.click(screen.getByTestId("settings-vault-delete-confirm"));
    expect(confirmDelete).toHaveBeenCalled();
  });

  it("session-scoped delete uses a single Confirm button without confirmTyping", () => {
    mockUseVaultPage.mockReturnValue(
      makeState({
        deleteTarget: { mode: "open", secret: sessionSecret },
      })
    );

    render(<VaultPage />);

    const dialog = screen.getByTestId("settings-vault-delete");
    expect(dialog).toHaveAttribute("data-scope", "session");
    expect(screen.queryByTestId("settings-vault-delete-confirm-typing")).toBeNull();
    expect(screen.getByTestId("settings-vault-delete-confirm")).toBeEnabled();
  });

  it("cross-scope delete gates the Confirm button behind confirmTyping", () => {
    const providerSecret: VaultSecret = {
      ref: "vault:providers/codex/api_key",
      namespace: "providers",
      kind: "api_key",
      present: true,
      created_at: "2026-05-02T10:00:00Z",
      updated_at: "2026-05-02T10:00:00Z",
    };
    mockUseVaultPage.mockReturnValue(
      makeState({
        deleteTarget: { mode: "open", secret: providerSecret },
        secrets: [providerSecret],
      })
    );

    render(<VaultPage />);

    const dialog = screen.getByTestId("settings-vault-delete");
    expect(dialog).toHaveAttribute("data-scope", "cross");
    const typingInput = screen.getByTestId("settings-vault-delete-confirm-typing");
    expect(typingInput).toBeInTheDocument();
    expect(screen.getByTestId("settings-vault-delete-confirm")).toBeDisabled();
    fireEvent.change(typingInput, { target: { value: providerSecret.ref } });
    expect(screen.getByTestId("settings-vault-delete-confirm")).toBeEnabled();
  });

  it("renders an Empty action retry button when the vault table fails to load", () => {
    const refetch = vi.fn();
    mockUseVaultPage.mockReturnValue(
      makeState({
        queryError: "vault offline",
        secrets: [],
        refetch,
      })
    );

    render(<VaultPage />);

    expect(screen.getByTestId("vault-page-error")).toHaveTextContent("vault offline");
    const retry = screen.getByTestId("vault-page-error-retry");
    expect(retry).toBeInTheDocument();
    fireEvent.click(retry);
    expect(refetch).toHaveBeenCalled();
  });
});
