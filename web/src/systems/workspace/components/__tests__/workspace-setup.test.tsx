import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { UIProvider } from "@compozy/ui";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { primaryAgentFixture } from "@/systems/agent/mocks";

import { useWorkspaceSetupContent } from "../../hooks/use-workspace-setup-content";
import type { WorkspaceSetupDefaultsModel } from "../../lib/workspace-setup-defaults";
import { WorkspaceSetupDialog } from "../workspace-setup";

const { mockCreateMutateAsync, mockToastSuccess, mockToastError, mockBrowseState } = vi.hoisted(
  () => ({
    mockCreateMutateAsync: vi.fn(),
    mockToastSuccess: vi.fn(),
    mockToastError: vi.fn(),
    mockBrowseState: {
      data: {
        path: "/Users/pedro/Dev",
        parent: "/Users/pedro",
        home: "/Users/pedro",
        roots: ["/"],
        entries: [
          { name: "checkout-platform", path: "/Users/pedro/Dev/checkout-platform", is_dir: true },
          { name: "billing", path: "/Users/pedro/Dev/billing", is_dir: true },
        ],
      } as unknown,
      isLoading: false,
      isFetching: false,
      error: null as Error | null,
    },
  })
);

vi.mock("sonner", () => ({
  toast: { success: mockToastSuccess, error: mockToastError },
}));

vi.mock("@/systems/onboarding/hooks/use-directory-browser", () => ({
  useDirectoryBrowser: () => mockBrowseState,
}));

vi.mock("../../hooks/use-workspaces", () => ({
  useCreateWorkspace: () => ({ mutateAsync: mockCreateMutateAsync }),
}));

const createdWorkspace = {
  id: "ws_project",
  root_dir: "/Users/pedro/Dev/checkout-platform",
  add_dirs: [],
  name: "checkout-platform",
  created_at: "2026-04-10T12:00:00Z",
  updated_at: "2026-04-10T12:00:00Z",
};

let mockDefaults: WorkspaceSetupDefaultsModel;

function WorkspaceSetupDialogHarness({
  defaults,
  onOpenChange,
  onWorkspaceResolved,
  open,
}: {
  defaults: WorkspaceSetupDefaultsModel;
  onOpenChange: (open: boolean) => void;
  onWorkspaceResolved: (workspaceId: string) => void;
  open: boolean;
}) {
  const setup = useWorkspaceSetupContent({
    onWorkspaceResolved,
    onSuccessClose: () => onOpenChange(false),
  });
  return (
    <WorkspaceSetupDialog model={{ defaults, setup }} onOpenChange={onOpenChange} open={open} />
  );
}

function renderDialog(
  props: {
    defaults?: WorkspaceSetupDefaultsModel;
    onOpenChange?: (open: boolean) => void;
    onWorkspaceResolved?: (workspaceId: string) => void;
    open?: boolean;
  } = {}
) {
  const onOpenChange = props.onOpenChange ?? vi.fn();
  const onWorkspaceResolved = props.onWorkspaceResolved ?? vi.fn();
  const open = props.open ?? true;

  const utils = render(
    <UIProvider reducedMotion="always">
      <WorkspaceSetupDialogHarness
        defaults={props.defaults ?? mockDefaults}
        open={open}
        onOpenChange={onOpenChange}
        onWorkspaceResolved={onWorkspaceResolved}
      />
    </UIProvider>
  );

  return { ...utils, onOpenChange, onWorkspaceResolved };
}

function resetMocks() {
  mockDefaults = {
    agents: { state: "ready", entries: [] },
    sandboxes: { state: "ready", entries: [] },
  };
  mockBrowseState.isLoading = false;
  mockBrowseState.isFetching = false;
  mockBrowseState.error = null;
  mockBrowseState.data = {
    path: "/Users/pedro/Dev",
    parent: "/Users/pedro",
    home: "/Users/pedro",
    entries: [
      { name: "checkout-platform", path: "/Users/pedro/Dev/checkout-platform", is_dir: true },
      { name: "billing", path: "/Users/pedro/Dev/billing", is_dir: true },
    ],
  };
  mockCreateMutateAsync.mockReset();
  mockToastSuccess.mockReset();
  mockToastError.mockReset();
}

describe("WorkspaceSetupDialog", () => {
  beforeEach(resetMocks);

  it("offers no plain path input for root selection", () => {
    renderDialog({ open: true });

    expect(screen.queryByLabelText("Workspace path")).not.toBeInTheDocument();
    expect(screen.queryByTestId("workspace-manual-path-input")).not.toBeInTheDocument();
    expect(screen.queryByTestId("workspace-register-manual")).not.toBeInTheDocument();
  });

  it("renders a portaled dialog when `open` is true", () => {
    renderDialog({ open: true });
    const host = screen.getByTestId("workspace-setup-dialog");
    expect(host).toBeInTheDocument();
    expect(host.closest("body")).toBe(document.body);
    expect(screen.getByTestId("workspace-setup-dialog-body")).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Add workspace" })).toBeInTheDocument();
  });

  it("keeps the root browser and optional defaults in their separate panes", () => {
    renderDialog({ open: true });

    const body = screen.getByTestId("workspace-setup-dialog-body");
    expect(within(body).getByTestId("workspace-setup-browser")).toBeInTheDocument();
    expect(within(body).getByTestId("workspace-setup-add-dir-input")).toBeInTheDocument();
  });

  it("does not mount the dialog body when `open` is false", () => {
    renderDialog({ open: false });
    expect(screen.queryByTestId("workspace-setup-dialog-body")).not.toBeInTheDocument();
  });

  it("selects a root from the browser without registering anything", async () => {
    const user = userEvent.setup();
    renderDialog({ open: true });

    expect(screen.getByTestId("workspace-setup-root-empty")).toBeInTheDocument();
    expect(screen.getByTestId("workspace-setup-submit")).toBeDisabled();

    await user.click(
      screen.getByRole("button", { name: "Use checkout-platform as the workspace root" })
    );

    // Picking a root must not write anything — registration waits for submit.
    expect(mockCreateMutateAsync).not.toHaveBeenCalled();
    expect(
      within(screen.getByTestId("workspace-setup-selected-root")).getByText(
        "/Users/pedro/Dev/checkout-platform"
      )
    ).toBeInTheDocument();
    expect(screen.getByTestId("workspace-setup-name-input")).toHaveValue("checkout-platform");
    expect(screen.getByTestId("workspace-setup-submit")).toBeEnabled();
  });

  it("issues exactly one create request carrying the full draft on submit", async () => {
    const user = userEvent.setup();
    mockCreateMutateAsync.mockResolvedValue(createdWorkspace);
    const { onOpenChange, onWorkspaceResolved } = renderDialog({ open: true });

    await user.click(screen.getByTestId("workspace-setup-browser-use-current"));
    await user.clear(screen.getByTestId("workspace-setup-name-input"));
    await user.type(screen.getByTestId("workspace-setup-name-input"), "Checkout platform");
    await user.type(screen.getByTestId("workspace-setup-add-dir-input"), "/Users/pedro/Dev/shared");
    await user.click(screen.getByTestId("workspace-setup-add-dir-submit"));
    await user.click(screen.getByTestId("workspace-setup-submit"));

    await waitFor(() => {
      expect(mockCreateMutateAsync).toHaveBeenCalledTimes(1);
    });
    expect(mockCreateMutateAsync).toHaveBeenCalledWith({
      root_dir: "/Users/pedro/Dev",
      name: "Checkout platform",
      add_dirs: ["/Users/pedro/Dev/shared"],
    });
    await waitFor(() => {
      expect(onWorkspaceResolved).toHaveBeenCalledWith("ws_project");
    });
    await waitFor(() => {
      expect(onOpenChange).toHaveBeenCalledWith(false);
    });
  });

  it("submits the selected default agent and sandbox profile", async () => {
    const user = userEvent.setup();
    mockCreateMutateAsync.mockResolvedValue(createdWorkspace);
    mockDefaults = {
      agents: { state: "ready", entries: [primaryAgentFixture] },
      sandboxes: { state: "ready", entries: [{ name: "isolated", backend: "docker" }] },
    };
    renderDialog({ open: true });

    await user.click(screen.getByTestId("workspace-setup-browser-use-current"));
    await user.click(screen.getByTestId("workspace-setup-default-agent-select"));
    await user.click(screen.getByTestId(`agent-command-item-${primaryAgentFixture.name}`));
    await user.click(screen.getByTestId("workspace-setup-sandbox-select"));
    await user.click(screen.getByTestId("workspace-setup-sandbox-isolated"));
    await user.click(screen.getByTestId("workspace-setup-submit"));

    await waitFor(() => {
      expect(mockCreateMutateAsync).toHaveBeenCalledWith({
        root_dir: "/Users/pedro/Dev",
        name: "Dev",
        default_agent: primaryAgentFixture.name,
        sandbox_ref: "isolated",
      });
    });
  });

  it("lets operators clear selected optional defaults before submitting", async () => {
    const user = userEvent.setup();
    mockCreateMutateAsync.mockResolvedValue(createdWorkspace);
    mockDefaults = {
      agents: { state: "ready", entries: [primaryAgentFixture] },
      sandboxes: { state: "ready", entries: [{ name: "isolated" }] },
    };
    renderDialog({ open: true });

    await user.click(screen.getByTestId("workspace-setup-browser-use-current"));
    await user.click(screen.getByTestId("workspace-setup-default-agent-select"));
    await user.click(screen.getByTestId(`agent-command-item-${primaryAgentFixture.name}`));
    await user.click(screen.getByTestId("workspace-setup-default-agent-select"));
    await user.click(screen.getByTestId("agent-command-item-clear"));
    await user.click(screen.getByTestId("workspace-setup-sandbox-select"));
    await user.click(screen.getByTestId("workspace-setup-sandbox-isolated"));
    await user.click(screen.getByTestId("workspace-setup-sandbox-select"));
    await user.click(screen.getByTestId("workspace-setup-sandbox-none"));
    await user.click(screen.getByTestId("workspace-setup-submit"));

    await waitFor(() => {
      expect(mockCreateMutateAsync).toHaveBeenCalledWith({
        root_dir: "/Users/pedro/Dev",
        name: "Dev",
      });
    });
  });

  it("keeps independent collection loading and failure states visible", () => {
    renderDialog({
      defaults: {
        agents: { state: "loading" },
        sandboxes: { state: "error", message: "settings endpoint unavailable" },
      },
      open: true,
    });

    expect(screen.getByTestId("workspace-setup-default-agent-loading")).toHaveTextContent(
      "Loading agents"
    );
    expect(screen.getByTestId("workspace-setup-default-agent-select")).toBeDisabled();
    expect(screen.getByTestId("workspace-setup-sandbox-error")).toHaveTextContent(
      "settings endpoint unavailable"
    );
    expect(screen.getByTestId("workspace-setup-sandbox-select")).toBeDisabled();
  });

  it("derives a display name from POSIX and Windows path separators", async () => {
    const user = userEvent.setup();
    const cases = [
      { path: "/Users/pedro/Dev/checkout-platform/", name: "checkout-platform" },
      { path: "C:\\Users\\pedro\\Dev\\checkout-platform\\", name: "checkout-platform" },
    ];

    for (const testCase of cases) {
      mockBrowseState.data = {
        path: testCase.path,
        parent: null,
        home: null,
        entries: [],
      };
      const view = renderDialog({ open: true });
      await user.click(screen.getByTestId("workspace-setup-browser-use-current"));
      expect(screen.getByTestId("workspace-setup-name-input")).toHaveValue(testCase.name);
      view.unmount();
    }
  });

  it("blocks duplicate submits and the close route while the create is in flight", async () => {
    const user = userEvent.setup();
    let releaseCreate: (() => void) | undefined;
    mockCreateMutateAsync.mockImplementation(
      () =>
        new Promise(resolve => {
          releaseCreate = () => resolve(createdWorkspace);
        })
    );
    renderDialog({ open: true });

    await user.click(screen.getByTestId("workspace-setup-browser-use-current"));
    await user.click(screen.getByTestId("workspace-setup-submit"));

    await waitFor(() => {
      expect(screen.getByTestId("workspace-setup-submit")).toBeDisabled();
    });
    expect(screen.getByTestId("workspace-setup-cancel")).toBeDisabled();
    expect(
      screen
        .getByTestId("workspace-setup-dialog")
        .querySelectorAll('[data-slot="entity-dialog-header-close"]')
    ).toHaveLength(0);

    releaseCreate?.();
    await waitFor(() => {
      expect(mockCreateMutateAsync).toHaveBeenCalledTimes(1);
    });
  });

  it("keeps the draft and reports the failure when the create is rejected", async () => {
    const user = userEvent.setup();
    mockCreateMutateAsync.mockRejectedValue(new Error("root is already registered"));
    const { onOpenChange } = renderDialog({ open: true });

    await user.click(screen.getByTestId("workspace-setup-browser-use-current"));
    await user.click(screen.getByTestId("workspace-setup-submit"));

    expect(await screen.findByTestId("workspace-setup-error")).toHaveTextContent(
      "root is already registered"
    );
    expect(screen.getByTestId("workspace-setup-name-input")).toHaveValue("Dev");
    expect(onOpenChange).not.toHaveBeenCalledWith(false);
  });

  it("surfaces the browser reading, empty, and error states", () => {
    mockBrowseState.isLoading = true;
    const { unmount } = renderDialog({ open: true });
    expect(screen.getByTestId("workspace-setup-browser-loading")).toBeInTheDocument();
    unmount();

    resetMocks();
    mockBrowseState.data = {
      path: "/Users/pedro/Dev",
      parent: "/Users/pedro",
      home: "/Users/pedro",
      entries: [],
    };
    const empty = renderDialog({ open: true });
    expect(screen.getByTestId("workspace-setup-browser-empty")).toBeInTheDocument();
    empty.unmount();

    resetMocks();
    mockBrowseState.error = new Error("permission denied");
    renderDialog({ open: true });
    expect(screen.getByTestId("workspace-setup-browser-error")).toHaveTextContent(
      "permission denied"
    );
  });
});
