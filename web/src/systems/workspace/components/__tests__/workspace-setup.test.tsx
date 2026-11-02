import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { UIProvider } from "@compozy/ui";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { primaryAgentFixture } from "@/systems/agent/mocks";

import { useWorkspaceSetupContent } from "../../hooks/use-workspace-setup-content";
import type { WorkspaceSetupDefaultsModel } from "../../lib/workspace-setup-defaults";
import { WorkspaceSetupDialog } from "../workspace-setup";

const { mockCreateMutateAsync, mockToastSuccess, mockToastError, mockRegisteredRoots } = vi.hoisted(
  () => ({
    mockCreateMutateAsync: vi.fn(),
    mockToastSuccess: vi.fn(),
    mockToastError: vi.fn(),
    mockRegisteredRoots: [] as string[],
  })
);

vi.mock("sonner", () => ({
  toast: { success: mockToastSuccess, error: mockToastError },
}));

/** The daemon's `/api/fs/browse` over a small fixed tree — the I/O boundary of the folder field. */
const HOME = "/Users/pedro";
const FOLDERS: Record<string, string[]> = {
  [HOME]: ["Dev"],
  "/Users/pedro/Dev": ["billing", "checkout-platform"],
  "/Users/pedro/Dev/billing": [],
  "/Users/pedro/Dev/checkout-platform": [],
};
const FAILURES: Record<string, number> = {
  "/Users/pedro/notes.txt": 400,
  "/Users/pedro/locked": 403,
};

vi.mock("@/systems/onboarding/adapters/onboarding-api", async importOriginal => {
  const actual =
    await importOriginal<typeof import("@/systems/onboarding/adapters/onboarding-api")>();
  return {
    ...actual,
    browseDirectory: vi.fn(async ({ path = HOME }: { path?: string }) => {
      const children = FOLDERS[path];
      if (!children) {
        throw new actual.OnboardingApiError("browse failed", FAILURES[path] ?? 404);
      }
      return {
        path,
        home: HOME,
        roots: ["/"],
        entries: children.map(name => ({ name, path: `${path}/${name}`, is_dir: true })),
      };
    }),
  };
});

vi.mock("../../hooks/use-workspaces", () => ({
  useCreateWorkspace: () => ({ mutateAsync: mockCreateMutateAsync }),
  useWorkspaces: () => ({ data: mockRegisteredRoots.map(root_dir => ({ root_dir })) }),
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

  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const utils = render(
    <QueryClientProvider client={queryClient}>
      <UIProvider reducedMotion="never" skipAnimations>
        <WorkspaceSetupDialogHarness
          defaults={props.defaults ?? mockDefaults}
          open={open}
          onOpenChange={onOpenChange}
          onWorkspaceResolved={onWorkspaceResolved}
        />
      </UIProvider>
    </QueryClientProvider>
  );

  return { ...utils, onOpenChange, onWorkspaceResolved };
}

function resetMocks() {
  mockDefaults = {
    agents: { state: "ready", entries: [] },
  };
  mockRegisteredRoots.length = 0;
  mockCreateMutateAsync.mockReset();
  mockToastSuccess.mockReset();
  mockToastError.mockReset();
}

/** Types a root and waits for the daemon to validate it. */
async function chooseRoot(user: ReturnType<typeof userEvent.setup>, path: string) {
  await user.type(screen.getByTestId("workspace-setup-root-input"), path);
  await waitFor(() => expect(screen.getByTestId("workspace-setup-submit")).toBeEnabled());
}

describe("WorkspaceSetupDialog", () => {
  beforeEach(resetMocks);

  it("renders a portaled dialog when `open` is true", () => {
    renderDialog({ open: true });
    const host = screen.getByTestId("workspace-setup-dialog");
    expect(host).toBeInTheDocument();
    expect(host.closest("body")).toBe(document.body);
    expect(screen.getByTestId("workspace-setup-dialog-body")).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Add project" })).toBeInTheDocument();
  });

  it("keeps optional defaults behind Advanced, in their own pane", async () => {
    const user = userEvent.setup();
    renderDialog({ open: true });

    const body = screen.getByTestId("workspace-setup-dialog-body");
    expect(within(body).getByTestId("workspace-setup-root-input")).toBeInTheDocument();
    expect(within(body).queryByTestId("workspace-setup-add-dir-input")).not.toBeInTheDocument();

    await user.click(screen.getByTestId("workspace-setup-mode-advanced"));
    expect(within(body).getByTestId("workspace-setup-root-input")).toBeInTheDocument();
    expect(within(body).getByTestId("workspace-setup-add-dir-input")).toBeInTheDocument();
  });

  it("does not mount the dialog body when `open` is false", () => {
    renderDialog({ open: false });
    expect(screen.queryByTestId("workspace-setup-dialog-body")).not.toBeInTheDocument();
  });

  it("accepts a typed ~ root, validates it, and registers nothing until submit", async () => {
    const user = userEvent.setup();
    renderDialog({ open: true });

    expect(screen.getByTestId("workspace-setup-submit")).toBeDisabled();
    await chooseRoot(user, "~/Dev/checkout-platform");

    // Choosing a root must not write anything — registration waits for submit.
    expect(mockCreateMutateAsync).not.toHaveBeenCalled();
    expect(screen.getByTestId("workspace-setup-name-input")).toHaveValue("checkout-platform");
    mockCreateMutateAsync.mockResolvedValue(createdWorkspace);
    await user.click(screen.getByTestId("workspace-setup-submit"));
    await waitFor(() =>
      expect(mockCreateMutateAsync).toHaveBeenCalledWith({
        root_dir: "/Users/pedro/Dev/checkout-platform",
        name: "checkout-platform",
      })
    );
  });

  it("completes folders from the daemon listing with the keyboard", async () => {
    const user = userEvent.setup();
    renderDialog({ open: true });
    const input = screen.getByTestId("workspace-setup-root-input");

    await user.type(input, "~/Dev/ch");
    expect(await screen.findByRole("option", { name: "checkout-platform" })).toBeInTheDocument();
    expect(screen.queryByRole("option", { name: "billing" })).not.toBeInTheDocument();
    await user.keyboard("{Tab}");

    expect(input).toHaveValue("~/Dev/checkout-platform/");
    await waitFor(() => expect(screen.getByTestId("workspace-setup-submit")).toBeEnabled());

    // A listing of the previous folder never completes newer text.
    await user.clear(input);
    await user.type(input, "~/Dev/ch");
    expect(await screen.findByRole("option", { name: "checkout-platform" })).toBeInTheDocument();
    await user.keyboard("{Tab}src{Tab}");
    expect(input).toHaveValue("~/Dev/checkout-platform/src");
  });

  it("explains why a typed root cannot be a project", async () => {
    const user = userEvent.setup();
    mockRegisteredRoots.push("/Users/pedro/Dev/billing");
    renderDialog({ open: true });
    const input = screen.getByTestId("workspace-setup-root-input");
    const cases = [
      ["code/app", "Use an absolute path, like ~/code/app."],
      ["~/missing", "No folder at this path."],
      ["~/notes.txt", "This path is a file, not a folder."],
      ["~/locked", "This folder can't be read. Check its permissions."],
      ["~/Dev/billing", "This folder is already a project."],
      ["~", "Your home folder can't be a project. Choose a folder inside it."],
    ];

    for (const [path, message] of cases) {
      await user.clear(input);
      await user.type(input, path!);
      expect(await screen.findByTestId("workspace-setup-root-error")).toHaveTextContent(message!);
      expect(input).toHaveAttribute("aria-invalid", "true");
      expect(screen.getByTestId("workspace-setup-submit")).toBeDisabled();
    }
  });

  it("issues exactly one create request carrying the full draft on submit", async () => {
    const user = userEvent.setup();
    mockCreateMutateAsync.mockResolvedValue(createdWorkspace);
    const { onOpenChange, onWorkspaceResolved } = renderDialog({ open: true });

    await user.click(screen.getByTestId("workspace-setup-mode-advanced"));
    await chooseRoot(user, "/Users/pedro/Dev");
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

  it("submits the selected default agent", async () => {
    const user = userEvent.setup();
    mockCreateMutateAsync.mockResolvedValue(createdWorkspace);
    mockDefaults = {
      agents: { state: "ready", entries: [primaryAgentFixture] },
    };
    renderDialog({ open: true });

    await user.click(screen.getByTestId("workspace-setup-mode-advanced"));
    await chooseRoot(user, "/Users/pedro/Dev");
    await user.click(screen.getByTestId("workspace-setup-default-agent-select"));
    await user.click(screen.getByTestId(`agent-command-item-${primaryAgentFixture.name}`));
    await user.click(screen.getByTestId("workspace-setup-submit"));

    await waitFor(() => {
      expect(mockCreateMutateAsync).toHaveBeenCalledWith({
        root_dir: "/Users/pedro/Dev",
        name: "Dev",
        default_agent: primaryAgentFixture.name,
      });
    });
  });

  it("lets operators clear selected optional defaults before submitting", async () => {
    const user = userEvent.setup();
    mockCreateMutateAsync.mockResolvedValue(createdWorkspace);
    mockDefaults = {
      agents: { state: "ready", entries: [primaryAgentFixture] },
    };
    renderDialog({ open: true });

    await user.click(screen.getByTestId("workspace-setup-mode-advanced"));
    await chooseRoot(user, "/Users/pedro/Dev");
    await user.click(screen.getByTestId("workspace-setup-default-agent-select"));
    await user.click(screen.getByTestId(`agent-command-item-${primaryAgentFixture.name}`));
    await user.click(screen.getByTestId("workspace-setup-default-agent-select"));
    await user.click(screen.getByTestId("agent-command-item-clear"));
    await user.click(screen.getByTestId("workspace-setup-submit"));

    await waitFor(() => {
      expect(mockCreateMutateAsync).toHaveBeenCalledWith({
        root_dir: "/Users/pedro/Dev",
        name: "Dev",
      });
    });
  });

  it("keeps the agent collection loading state visible", async () => {
    const user = userEvent.setup();
    renderDialog({
      defaults: {
        agents: { state: "loading" },
      },
      open: true,
    });
    await user.click(screen.getByTestId("workspace-setup-mode-advanced"));

    expect(screen.getByTestId("workspace-setup-default-agent-loading")).toHaveTextContent(
      "Loading agents"
    );
    // The control is a placeholder until the catalog arrives, so nothing can be picked.
    expect(screen.queryByTestId("workspace-setup-default-agent-select")).not.toBeInTheDocument();
  });

  it.each([
    ["POSIX", "/Users/pedro/Dev/checkout-platform/"],
    ["Windows", "C:\\Users\\pedro\\Dev\\checkout-platform\\"],
  ])("derives a display name from %s path separators", async (_label, path) => {
    const user = userEvent.setup();
    renderDialog({ open: true });
    // Typing before the dialog's deferred initial focus lands would lose keystrokes.
    await waitFor(() => expect(screen.getByTestId("workspace-setup-dialog")).toHaveFocus());
    await user.type(screen.getByTestId("workspace-setup-root-input"), path);
    expect(screen.getByTestId("workspace-setup-name-input")).toHaveValue("checkout-platform");
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

    await chooseRoot(user, "/Users/pedro/Dev");
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

    await chooseRoot(user, "/Users/pedro/Dev");
    await user.click(screen.getByTestId("workspace-setup-submit"));

    expect(await screen.findByTestId("workspace-setup-error")).toHaveTextContent(
      "root is already registered"
    );
    expect(screen.getByTestId("workspace-setup-name-input")).toHaveValue("Dev");
    expect(onOpenChange).not.toHaveBeenCalledWith(false);
  });

  it("shows the config file, unknown keys and remediation while preserving the project draft", async () => {
    const user = userEvent.setup();
    const message =
      'workspace config invalid: unknown config keys in "/project/.compozy/config.toml": defaults.ide, tasks.run. ' +
      "Remove the unknown keys, or rename or remove this file if it belongs to the legacy CLI. " +
      "CompozyOS does not support legacy CLI configuration.";
    mockCreateMutateAsync.mockRejectedValue(new Error(message));
    const { onOpenChange, onWorkspaceResolved } = renderDialog({ open: true });

    await chooseRoot(user, "/Users/pedro/Dev");
    await user.click(screen.getByTestId("workspace-setup-submit"));

    expect(await screen.findByTestId("workspace-setup-error")).toHaveTextContent(message);
    expect(screen.getByTestId("workspace-setup-name-input")).toHaveValue("Dev");
    expect(screen.getByTestId("workspace-setup-submit")).toBeEnabled();
    expect(onOpenChange).not.toHaveBeenCalledWith(false);
    expect(onWorkspaceResolved).not.toHaveBeenCalled();
  });
});
