// Suite: Removed built-ins section
// Invariant: the removed list shows each tombstone in the daemon's order with a restore action, reports
// load failures with a retry instead of an empty list, blocks a second restore while one is in flight,
// keeps a conflicting row listed, and refreshes the installed list after a restore. The section is absent
// for a viewer who cannot restore.
// Boundary IN: RemovedBuiltinsSection rendered over the real adapter, query cache and generated client,
// with only the daemon's HTTP responses stubbed.
// Boundary OUT: adapter route and error mapping, owned by adapters/__tests__/extensions-api.test.ts.
import { QueryClient, QueryClientProvider, useQuery } from "@tanstack/react-query";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { extensionKeys } from "../../lib/query-keys";
import { extensionsListOptions } from "../../lib/query-options";
import { RemovedBuiltinsSection } from "../removed-builtins-section";
import type { ExtensionRemovedBuiltin } from "../../types";

const REMOVED_AT = "2026-11-17T14:02:11Z";
const zetaTools: ExtensionRemovedBuiltin = {
  name: "zeta-tools",
  removed_at: REMOVED_AT,
  removed_by: "operator",
};
const alphaKit: ExtensionRemovedBuiltin = {
  name: "alpha-kit",
  removed_at: "2026-11-16T09:00:00Z",
  removed_by: "operator",
};

const restoredZeta = {
  enabled: true,
  name: "zeta-tools",
  path: "/var/lib/compozy/extensions/zeta-tools",
  source: "bundled",
  status: "restored",
  version: "0.7.3",
};

const installedOtel = {
  name: "otel-bridge",
  enabled: true,
  source: "marketplace",
  version: "1.0.0",
  mcp_servers: [],
  inputs: [],
  missing_inputs: [],
};

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    headers: { "Content-Type": "application/json" },
    status,
  });
}

type DaemonResponse = Response | Promise<Response>;

interface DaemonHandlers {
  removed: () => DaemonResponse;
  installed: () => DaemonResponse;
  restore: (name: string) => DaemonResponse;
}

/**
 * Stubs only the daemon's HTTP responses. Every other layer (adapter, generated client, query cache)
 * runs for real, so a wrong route or a missing invalidation fails the assertions below.
 */
function stubDaemon(overrides: Partial<DaemonHandlers> = {}) {
  const handlers: DaemonHandlers = {
    removed: () => jsonResponse({ removed: [] }),
    installed: () => jsonResponse({ extensions: [installedOtel] }),
    restore: () => jsonResponse(restoredZeta),
    ...overrides,
  };
  const fetchMock = vi.fn(async (request: Request) => {
    const { pathname } = new URL(request.url);
    if (request.method === "GET" && pathname === "/api/extensions/removed-builtins") {
      return handlers.removed();
    }
    if (request.method === "GET" && pathname === "/api/extensions") {
      return handlers.installed();
    }
    const restore = /^\/api\/extensions\/([^/]+)\/restore$/.exec(pathname);
    if (request.method === "POST" && restore) {
      return handlers.restore(decodeURIComponent(restore[1]!));
    }
    throw new Error(`unexpected daemon request ${request.method} ${pathname}`);
  });
  vi.stubGlobal("fetch", fetchMock);
  return fetchMock;
}

function requestCount(fetchMock: ReturnType<typeof stubDaemon>, path: string): number {
  return fetchMock.mock.calls.filter(([request]) => new URL(request.url).pathname === path).length;
}

/** Shows the installed list the way the installed page reads it, to observe a restore's effect. */
function InstalledProbe() {
  const installed = useQuery(extensionsListOptions());
  return (
    <ul data-testid="installed-probe">
      {installed.data?.map(extension => (
        <li data-testid={`installed-${extension.name}`} key={extension.name}>
          {extension.enabled ? "enabled" : "disabled"}
        </li>
      ))}
    </ul>
  );
}

function renderSection({ canRestore = true, probe = false } = {}) {
  const queryClient = new QueryClient({
    defaultOptions: { mutations: { retry: false }, queries: { retry: false } },
  });
  const utils = render(
    <QueryClientProvider client={queryClient}>
      <RemovedBuiltinsSection canRestore={canRestore} />
      {probe ? <InstalledProbe /> : null}
    </QueryClientProvider>
  );
  return { ...utils, queryClient };
}

function removedRow(name: string): HTMLElement {
  const row = screen
    .getAllByTestId("removed-builtin-row")
    .find(item => within(item).queryByText(name) !== null);
  expect(row, `removed row ${name}`).toBeDefined();
  return row!;
}

beforeEach(() => {
  vi.clearAllMocks();
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("RemovedBuiltinsSection listing", () => {
  // CT-002
  it("Should list each removed built-in with its removal time and restore action in the daemon's order", async () => {
    stubDaemon({
      removed: () => jsonResponse({ removed: [zetaTools, alphaKit] }),
    });
    renderSection();

    const section = await screen.findByTestId("removed-builtins-section");
    expect(within(section).getByText("Removed built-ins")).toBeInTheDocument();
    const rows = await within(section).findAllByTestId("removed-builtin-row");
    expect(rows.map(row => within(row).getByText(/^(zeta-tools|alpha-kit)$/).textContent)).toEqual([
      "zeta-tools",
      "alpha-kit",
    ]);
    expect(rows[0]!.querySelector("time")).toHaveAttribute("datetime", REMOVED_AT);
    expect(within(rows[0]!).getByRole("button", { name: "Restore zeta-tools" })).toBeEnabled();
    expect(within(rows[1]!).getByRole("button", { name: "Restore alpha-kit" })).toBeEnabled();
  });

  it("Should hide the section once the list loads empty", async () => {
    const fetchMock = stubDaemon({ removed: () => jsonResponse({ removed: [] }) });
    const { queryClient } = renderSection();

    await waitFor(() =>
      expect(queryClient.getQueryState(extensionKeys.removedBuiltins())?.status).toBe("success")
    );
    expect(requestCount(fetchMock, "/api/extensions/removed-builtins")).toBe(1);
    expect(screen.queryByTestId("removed-builtins-section")).not.toBeInTheDocument();
    expect(screen.queryByText("Removed built-ins")).not.toBeInTheDocument();
  });
});

describe("RemovedBuiltinsSection load failure", () => {
  // CT-003
  it("Should show an error with a retry control and no empty list when the request fails", async () => {
    let attempt = 0;
    const fetchMock = stubDaemon({
      removed: () => {
        attempt += 1;
        return attempt === 1
          ? jsonResponse({ error: "daemon unavailable" }, 500)
          : jsonResponse({ removed: [zetaTools] });
      },
    });
    renderSection();

    const error = await screen.findByTestId("removed-builtins-error");
    expect(within(error).getByText("Couldn't load removed built-ins")).toBeInTheDocument();
    expect(within(error).getByText("daemon unavailable")).toBeInTheDocument();
    expect(screen.queryByTestId("removed-builtin-row")).not.toBeInTheDocument();

    const retry = within(error).getByRole("button", { name: "Retry" });
    await userEvent.setup().click(retry);

    expect(await screen.findByTestId("removed-builtin-row")).toHaveTextContent("zeta-tools");
    expect(screen.queryByTestId("removed-builtins-error")).not.toBeInTheDocument();
    expect(requestCount(fetchMock, "/api/extensions/removed-builtins")).toBe(2);
  });
});

describe("RemovedBuiltinsSection restore", () => {
  // CT-004
  it("Should disable the pending restore and ignore a second trigger until it settles", async () => {
    const user = userEvent.setup();
    let releaseRestore: (response: Response) => void = () => undefined;
    let removedNow: ExtensionRemovedBuiltin[] = [zetaTools];
    const fetchMock = stubDaemon({
      removed: () => jsonResponse({ removed: removedNow }),
      restore: () =>
        new Promise<Response>(resolve => {
          releaseRestore = resolve;
        }).then(response => {
          removedNow = [];
          return response;
        }),
    });
    renderSection();

    const row = await screen.findByTestId("removed-builtin-row");
    await user.click(within(row).getByRole("button", { name: "Restore zeta-tools" }));

    await waitFor(() =>
      expect(requestCount(fetchMock, "/api/extensions/zeta-tools/restore")).toBe(1)
    );
    const pending = within(row).getByRole("button", { name: "Restore zeta-tools" });
    expect(pending).toBeDisabled();
    expect(pending).toHaveTextContent("Restoring…");

    await user.click(pending);
    expect(requestCount(fetchMock, "/api/extensions/zeta-tools/restore")).toBe(1);

    releaseRestore(jsonResponse(restoredZeta));
    await waitFor(() =>
      expect(screen.queryByTestId("removed-builtins-section")).not.toBeInTheDocument()
    );
    expect(requestCount(fetchMock, "/api/extensions/zeta-tools/restore")).toBe(1);
  });

  it("Should keep the row and show the conflict inline when an installed copy holds the name", async () => {
    const user = userEvent.setup();
    stubDaemon({
      removed: () => jsonResponse({ removed: [zetaTools] }),
      restore: () =>
        jsonResponse(
          {
            code: "extension_exists",
            error: "zeta-tools is already installed from marketplace; remove it first",
          },
          409
        ),
    });
    renderSection();

    await user.click(
      within(await screen.findByTestId("removed-builtin-row")).getByRole("button", {
        name: "Restore zeta-tools",
      })
    );

    const row = await screen.findByTestId("removed-builtin-error");
    expect(row).toHaveTextContent(
      "zeta-tools is already installed. Remove the installed copy first."
    );
    const sameRow = removedRow("zeta-tools");
    expect(within(sameRow).getByRole("button", { name: "Restore zeta-tools" })).toBeEnabled();
    expect(within(sameRow).getByRole("button", { name: "Restore zeta-tools" })).toHaveTextContent(
      "Restore"
    );
  });

  it("Should show any other refusal with the daemon's message on the same row", async () => {
    const user = userEvent.setup();
    stubDaemon({
      removed: () => jsonResponse({ removed: [zetaTools] }),
      restore: () =>
        jsonResponse(
          { code: "extension_builtin_not_removed", error: "zeta-tools is not permanently removed" },
          409
        ),
    });
    renderSection();

    await user.click(
      within(await screen.findByTestId("removed-builtin-row")).getByRole("button", {
        name: "Restore zeta-tools",
      })
    );

    expect(await screen.findByTestId("removed-builtin-error")).toHaveTextContent(
      "zeta-tools is not permanently removed"
    );
    expect(removedRow("zeta-tools")).toBeInTheDocument();
  });
});

describe("RemovedBuiltinsSection installed-list refresh", () => {
  // CT-005
  it("Should refresh the installed list on a successful restore so the built-in appears there enabled without a reload", async () => {
    const user = userEvent.setup();
    let removedNow: ExtensionRemovedBuiltin[] = [zetaTools];
    let installedNow = [installedOtel];
    const fetchMock = stubDaemon({
      removed: () => jsonResponse({ removed: removedNow }),
      installed: () => jsonResponse({ extensions: installedNow }),
      restore: () => {
        removedNow = [];
        installedNow = [
          installedOtel,
          { ...installedOtel, name: "zeta-tools", enabled: true, source: "bundled" },
        ];
        return jsonResponse(restoredZeta);
      },
    });
    renderSection({ probe: true });

    const installed = await screen.findByTestId("installed-probe");
    await waitFor(() =>
      expect(within(installed).getByTestId("installed-otel-bridge")).toBeInTheDocument()
    );
    expect(within(installed).queryByTestId("installed-zeta-tools")).not.toBeInTheDocument();

    await user.click(
      within(await screen.findByTestId("removed-builtin-row")).getByRole("button", {
        name: "Restore zeta-tools",
      })
    );

    await waitFor(() =>
      expect(within(installed).getByTestId("installed-zeta-tools")).toHaveTextContent("enabled")
    );
    expect(requestCount(fetchMock, "/api/extensions")).toBeGreaterThanOrEqual(2);
    await waitFor(() =>
      expect(screen.queryByTestId("removed-builtins-section")).not.toBeInTheDocument()
    );
  });
});

describe("RemovedBuiltinsSection permissions", () => {
  // CT-006
  it("Should render nothing and request no removed list when the viewer cannot restore", async () => {
    const fetchMock = stubDaemon({ removed: () => jsonResponse({ removed: [zetaTools] }) });
    const { container } = renderSection({ canRestore: false });

    await waitFor(() => expect(fetchMock).not.toHaveBeenCalled());
    expect(container).toBeEmptyDOMElement();
    expect(screen.queryByRole("button", { name: /Restore/ })).not.toBeInTheDocument();
    expect(screen.queryByText("Removed built-ins")).not.toBeInTheDocument();
  });
});
