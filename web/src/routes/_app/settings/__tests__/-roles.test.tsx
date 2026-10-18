import { fireEvent, screen, within } from "@testing-library/react";
import { renderWithTopbar as render } from "@/test/render-with-topbar";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { RolesDisclosure, RolesRuntimeOptions } from "@/systems/settings";
import { buildRolesViewModel, type RoleViewModel } from "@/systems/settings";
import {
  rolesStatusFixture,
  rolesStatusWithDiagnosticFixture,
  settingsRolesConfigFixture,
} from "@/systems/settings/mocks";

const defaultRoles = buildRolesViewModel(rolesStatusFixture.roles, settingsRolesConfigFixture);
const diagnosticRoles = buildRolesViewModel(
  rolesStatusWithDiagnosticFixture.roles,
  settingsRolesConfigFixture
);

const restartBanner = {
  isVisible: false,
  isRestartRequired: false,
  operationId: null,
  status: null,
  activeSessionCount: 0,
  lastMutation: null,
  trigger: vi.fn(),
  isTriggerPending: false,
  triggerError: null,
  dismiss: vi.fn(),
};

const runtimeOptions: RolesRuntimeOptions = {
  providers: [{ id: "anthropic", name: "Anthropic", runtime_provider: "anthropic" }],
  models: [
    {
      id: "claude-haiku-4-5",
      provider: "anthropic",
      name: "claude-haiku-4-5",
      efforts: [],
      availability: "live",
    },
  ],
  agents: [],
  loading: false,
  catalogError: null,
  catalogStale: false,
  refresh: vi.fn(),
  refreshing: false,
};

/** Open state is exercised directly so each test states the disclosure it needs. */
function disclosureFor(openRoles: Set<string>): RolesDisclosure {
  return {
    isOpen: role => openRoles.has(role),
    setOpen: (role, open) => {
      if (open) openRoles.add(role);
      else openRoles.delete(role);
    },
    expandAll: vi.fn(),
    collapseAll: vi.fn(),
    anyOpen: openRoles.size > 0,
  };
}

let pageState: {
  isLoading: boolean;
  isEmpty: boolean;
  error: Error | null;
  roles: RoleViewModel[];
  runtimeOptions: RolesRuntimeOptions;
  disclosure: RolesDisclosure;
  isDirty: boolean;
  isInvalid: boolean;
  draftRevision: number;
  validationErrors: Record<string, string>;
  isSaving: boolean;
  saveError: string | null;
  warnings: string[] | undefined;
  lastAppliedLabel: string | null;
  restart: typeof restartBanner;
  setRoleEnabled: ReturnType<typeof vi.fn>;
  setRoleAgent: ReturnType<typeof vi.fn>;
  setRoleField: ReturnType<typeof vi.fn>;
  setRoleRuntime: ReturnType<typeof vi.fn>;
  clearRuntime: ReturnType<typeof vi.fn>;
  setNumberFieldValidity: ReturnType<typeof vi.fn>;
  addFallback: ReturnType<typeof vi.fn>;
  removeFallback: ReturnType<typeof vi.fn>;
  updateFallback: ReturnType<typeof vi.fn>;
  registerFieldRef: ReturnType<typeof vi.fn>;
  handleSave: ReturnType<typeof vi.fn>;
  handleReset: ReturnType<typeof vi.fn>;
  handleRetry: ReturnType<typeof vi.fn>;
};

vi.mock("@tanstack/react-router", async importOriginal => {
  const actual = await importOriginal<typeof import("@tanstack/react-router")>();
  return { ...actual, useNavigate: () => vi.fn() };
});

vi.mock("@/systems/settings/hooks/use-settings-roles-page", () => ({
  useSettingsRolesPage: () => pageState,
}));

beforeEach(() => {
  pageState = {
    isLoading: false,
    isEmpty: false,
    error: null,
    roles: defaultRoles,
    runtimeOptions,
    disclosure: disclosureFor(new Set()),
    isDirty: false,
    isInvalid: false,
    draftRevision: 0,
    validationErrors: {},
    isSaving: false,
    saveError: null,
    warnings: undefined,
    lastAppliedLabel: null,
    restart: { ...restartBanner, trigger: vi.fn(), dismiss: vi.fn() },
    setRoleEnabled: vi.fn(),
    setRoleAgent: vi.fn(),
    setRoleField: vi.fn(),
    setRoleRuntime: vi.fn(),
    clearRuntime: vi.fn(),
    setNumberFieldValidity: vi.fn(() => vi.fn()),
    addFallback: vi.fn(),
    removeFallback: vi.fn(),
    updateFallback: vi.fn(),
    registerFieldRef: vi.fn(() => vi.fn()),
    handleSave: vi.fn(),
    handleReset: vi.fn(),
    handleRetry: vi.fn(),
  };
});

import { RolesSettingsPage } from "../-roles-settings-page";

function group(role: string): HTMLElement {
  return screen.getByTestId(`settings-page-roles-group-${role}`);
}

describe("RolesSettingsPage", () => {
  it("renders a loading indicator while either read is pending", () => {
    pageState.isLoading = true;
    render(<RolesSettingsPage />);
    expect(screen.getByTestId("settings-page-roles-loading")).toBeInTheDocument();
  });

  it("renders the empty-projection anomaly with retry only", () => {
    pageState.isEmpty = true;
    render(<RolesSettingsPage />);
    const empty = screen.getByTestId("settings-page-roles-empty");
    expect(empty).toHaveTextContent("Couldn't load Roles");
    expect(empty).toHaveTextContent("No roles were returned.");
    fireEvent.click(screen.getByRole("button", { name: "Try again" }));
    expect(pageState.handleRetry).toHaveBeenCalledTimes(1);
  });

  it("renders the server error with a retry action", () => {
    pageState.error = new Error("roles service unavailable");
    render(<RolesSettingsPage />);
    const error = screen.getByTestId("settings-page-roles-error");
    expect(error).toHaveTextContent("Couldn't load Roles");
    expect(error).toHaveTextContent("roles service unavailable");
    fireEvent.click(screen.getByRole("button", { name: "Try again" }));
    expect(pageState.handleRetry).toHaveBeenCalledTimes(1);
  });

  it("renders two collapsed role rows in product order with built-in resolution lines (UT-074)", () => {
    render(<RolesSettingsPage />);

    const groups = screen.getAllByTestId(/^settings-page-roles-group-/);
    expect(groups.map(node => node.dataset.testid)).toEqual([
      "settings-page-roles-group-coordinator",
      "settings-page-roles-group-auto_title",
    ]);
    expect(screen.getByTestId("settings-page-roles-coordinator-resolution")).toHaveTextContent(
      /^Built in/
    );
    expect(within(group("coordinator")).queryByText("BUILTIN")).not.toBeInTheDocument();
    // Every row starts closed, so the routing controls stay out of the way.
    expect(
      screen.queryByTestId("settings-page-roles-coordinator-runtime-select")
    ).not.toBeVisible();
  });

  it("carries enabled state on the header switch without expanding the row", () => {
    render(<RolesSettingsPage />);

    const coordinator = screen.getByTestId("settings-page-roles-coordinator-enabled-switch");
    expect(coordinator).toHaveAttribute("aria-checked", "false");
    expect(screen.getByTestId("settings-page-roles-auto_title-enabled-switch")).toHaveAttribute(
      "aria-checked",
      "true"
    );

    fireEvent.click(coordinator);
    expect(pageState.setRoleEnabled).toHaveBeenCalledWith("coordinator", true);
  });

  it("states the default-agent resolution for inherit roles without a pill (UT-075)", () => {
    pageState.disclosure = disclosureFor(new Set(["auto_title"]));
    render(<RolesSettingsPage />);

    expect(within(group("auto_title")).queryByText("INHERIT")).not.toBeInTheDocument();
    expect(screen.getByTestId("settings-page-roles-auto_title-resolution")).toHaveTextContent(
      "Uses your default agent."
    );
    // A null projection is stated as unresolved, never as a fabricated route.
    expect(screen.getByTestId("settings-page-roles-auto_title-runtime")).toHaveTextContent(
      "Decided when the role runs."
    );
    expect(screen.queryByTestId("settings-page-roles-auto_title-route")).not.toBeInTheDocument();
  });

  it("renders policy rows only for roles that declare them (UT-076)", () => {
    pageState.disclosure = disclosureFor(new Set(["coordinator", "auto_title"]));
    render(<RolesSettingsPage />);

    expect(screen.getByTestId("settings-page-roles-coordinator-ttl-input")).toBeInTheDocument();
    expect(
      screen.queryByTestId("settings-page-roles-auto_title-ttl-input")
    ).not.toBeInTheDocument();
    // Every role carries a session identity, so each one offers an agent picker.
    expect(screen.getByTestId("settings-page-roles-coordinator-agent-select")).toBeInTheDocument();
    expect(screen.getByTestId("settings-page-roles-auto_title-agent-select")).toBeInTheDocument();
  });

  it("surfaces a role diagnostic as a visible warning on the affected row (UT-077)", () => {
    pageState.roles = diagnosticRoles;
    pageState.disclosure = disclosureFor(new Set(["auto_title"]));
    render(<RolesSettingsPage />);

    const notice = screen.getByTestId(
      "settings-page-roles-auto_title-diagnostics-role_agent_not_found"
    );
    expect(notice).toHaveTextContent("Warning");
    expect(notice).toHaveTextContent("ghost");
    // The collapsed header marks the warning too, so it is findable while closed.
    expect(screen.getByTestId("settings-page-roles-auto_title-warning-mark")).toBeInTheDocument();
  });

  it("renders the editable fallback chain and wires add to the page handler", () => {
    pageState.disclosure = disclosureFor(new Set(["auto_title"]));
    render(<RolesSettingsPage />);

    fireEvent.click(screen.getByTestId("settings-page-roles-auto_title-advanced-fallback-add"));
    expect(pageState.addFallback).toHaveBeenCalledWith("auto_title");
  });

  it("offers a bulk expand control over the role list", () => {
    render(<RolesSettingsPage />);

    expect(screen.getByTestId("settings-page-roles-count")).toHaveTextContent("2 roles · 1 off");
    fireEvent.click(screen.getByTestId("settings-page-roles-expand-toggle"));
    expect(pageState.disclosure.expandAll).toHaveBeenCalledTimes(1);
  });

  it("wires the shared save bar to the page handlers", () => {
    pageState.isDirty = true;
    render(<RolesSettingsPage />);

    fireEvent.click(screen.getByTestId("settings-page-roles-save"));
    expect(pageState.handleSave).toHaveBeenCalledTimes(1);
    fireEvent.click(screen.getByTestId("settings-page-roles-reset"));
    expect(pageState.handleReset).toHaveBeenCalledTimes(1);
  });
});
