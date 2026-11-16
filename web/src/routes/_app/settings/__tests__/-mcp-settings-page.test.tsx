// Invariant: Settings actions select the daemon-returned MCP owner and scope, even when names repeat;
// runtime groups, approval and copy act on the daemon's runtime inventory and approval digests.
// Owner: the MCP Settings page; no existing route suite exercises this composition.
// Canonical suite: this file. HTTP is mocked; routing, query state, controllers and editors run normally.
import { Topbar, TopbarSlotProvider } from "@compozy/ui";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  createMemoryHistory,
  createRootRoute,
  createRouter,
  RouterProvider,
} from "@tanstack/react-router";
import { cleanup, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { HttpResponse, delay, http } from "msw";
import { setupServer } from "msw/node";
import { afterAll, afterEach, beforeAll, beforeEach, describe, expect, it } from "vitest";

import { handlers as profileHandlers } from "@/systems/profiles/mocks";
import { handlers as workspaceHandlers } from "@/systems/workspace/mocks";
import { handlers as statusHandlers } from "@/systems/status/mocks";
import { handlers as settingsHandlers } from "@/systems/settings/mocks";
import {
  mcpExtensionServerFixtures,
  settingsAppliedMutationFixture,
} from "@/systems/settings/mocks/fixtures";
import {
  claudeRuntimeMCPFixture,
  codexRuntimeMCPFixture,
  manyRuntimeMCPFixture,
  piRuntimeMCPFixture,
  runtimeMCPInventoryFixture,
} from "@/systems/settings/mocks/runtime-mcp-fixtures";
import type { RuntimeMCPSnapshot } from "@/systems/settings/types";
import { resetSettingsRestartStore } from "@/systems/settings/stores/use-settings-restart-store";
import {
  mcpServerRowTestId,
  type SettingsMCPServerEntry,
  type SettingsMCPServerRequest,
} from "@/systems/settings";
import { MCPSettingsPage } from "../-mcp-settings-page";

let definitions: SettingsMCPServerEntry[] = [];
const server = setupServer(
  http.get("*/api/settings/mcp-servers", () =>
    HttpResponse.json({
      collection: "mcp-servers",
      available_scopes: ["user", "workspace"],
      scope: "user",
      mcp_servers: definitions,
    })
  ),
  ...settingsHandlers,
  ...profileHandlers,
  ...workspaceHandlers,
  ...statusHandlers
);
const clients: QueryClient[] = [];
beforeAll(() => server.listen({ onUnhandledFrame: "error" }));
afterAll(() => server.close());
beforeEach(() => {
  definitions = structuredClone(mcpExtensionServerFixtures.slice(0, 2));
  resetSettingsRestartStore();
});
afterEach(() => {
  cleanup();
  clients.splice(0).forEach(client => client.clear());
  server.resetHandlers();
  resetSettingsRestartStore();
});
function renderSettings() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  clients.push(client);
  const root = createRootRoute({
    component: () => (
      <TopbarSlotProvider>
        <Topbar title="Settings" />
        <MCPSettingsPage />
      </TopbarSlotProvider>
    ),
  });
  const router = createRouter({
    routeTree: root,
    history: createMemoryHistory({ initialEntries: ["/settings/mcp"] }),
  });
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  );
}

async function openSettings() {
  renderSettings();
  await screen.findByRole("button", { name: "Select github from github" });
}

describe("MCP Settings page", () => {
  it("Should hold the table's place with a skeleton, and claim no server count, while the servers load", async () => {
    server.use(
      http.get("*/api/settings/mcp-servers", async () => {
        await delay("infinite");
        return HttpResponse.json({});
      })
    );
    renderSettings();

    const loading = await screen.findByTestId("settings-page-mcp-loading");
    expect(loading).toHaveAttribute("role", "status");
    expect(loading).toHaveTextContent("Loading MCP servers");
    expect(loading.querySelector('[data-slot="skeleton"]')).not.toBeNull();
    // The head and toolbar are live at once; only the count and the rows wait.
    expect(screen.getByTestId("settings-page-mcp-search")).toBeVisible();
    expect(screen.queryByTestId("settings-page-mcp-count")).not.toBeInTheDocument();
    expect(
      screen.getByTestId("settings-page-mcp-subhead").querySelector('[data-slot="skeleton"]')
    ).not.toBeNull();
  });

  it("Should keep same-name owners separate and save or reset only the extension override", async () => {
    const user = userEvent.setup();
    const writes: Array<{ url: URL; body: SettingsMCPServerRequest }> = [];
    const resets: URL[] = [];
    const manual = structuredClone(definitions[0]);
    server.use(
      http.put("*/api/settings/mcp-servers/:name", async ({ request }) => {
        const body = (await request.json()) as SettingsMCPServerRequest;
        writes.push({ url: new URL(request.url), body });
        definitions[1] = {
          ...definitions[1]!,
          override: { headers: body.server.headers, url: body.server.url },
        };
        return HttpResponse.json({ ...settingsAppliedMutationFixture, section: "mcp-servers" });
      }),
      http.delete("*/api/settings/mcp-servers/:name", ({ request }) => {
        resets.push(new URL(request.url));
        definitions[1] = { ...definitions[1]!, override: undefined };
        return HttpResponse.json({ ...settingsAppliedMutationFixture, section: "mcp-servers" });
      })
    );
    await openSettings();
    expect(screen.getByRole("button", { name: "Select github" })).toBeVisible();
    await user.click(screen.getByRole("button", { name: "Select github from github" }));
    expect(screen.getByTestId("settings-page-mcp-selection-provenance")).toHaveTextContent(
      "agents see it as github.github"
    );
    expect(screen.queryByTestId("settings-page-mcp-selection-delete")).not.toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Edit github from github" }));
    const dialog = await screen.findByRole("dialog", { name: "Set up github" });
    // The package declaration is read-only and folded under Technical details.
    await user.click(within(dialog).getByRole("button", { name: "Technical details" }));
    expect(within(dialog).getByText("extension:github")).toBeVisible();
    expect(
      within(dialog).queryByRole("textbox", { name: /Command|Auth|Server name/ })
    ).not.toBeInTheDocument();
    await user.type(
      within(dialog).getByRole("textbox", { name: "Address" }),
      "https://override.example/mcp"
    );
    await user.click(within(dialog).getByRole("button", { name: "Save" }));
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
    expect(writes).toHaveLength(1);
    expect(writes[0]!.url.pathname).toBe("/api/settings/mcp-servers/github");
    expect(writes[0]!.url.searchParams.get("owner")).toBe("extension:github");
    expect(writes[0]!.url.searchParams.get("scope")).toBe("user");
    expect(writes[0]!.body).toEqual({
      server: {
        name: "github",
        env: {},
        headers: { "X-GitHub-Api-Version": "2022-11-28" },
        url: "https://override.example/mcp",
      },
    });
    expect(definitions[0]).toEqual(manual);
    await user.click(screen.getByRole("button", { name: "Edit github from github" }));
    await user.click(await screen.findByRole("button", { name: "Reset to extension defaults" }));
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
    expect(resets).toHaveLength(1);
    expect(resets[0]!.searchParams.get("owner")).toBe("extension:github");
    expect(resets[0]!.searchParams.get("scope")).toBe("user");
    expect(definitions[0]).toEqual(manual);
    expect(screen.getByRole("button", { name: "Select github from github" })).toBeVisible();
  });

  it("Should delete the selected manual source without removing its same-name extension", async () => {
    const user = userEvent.setup();
    const removed: URL[] = [];
    server.use(
      http.delete("*/api/settings/mcp-servers/:name", ({ request }) => {
        removed.push(new URL(request.url));
        definitions = definitions.filter(entry => entry.owner !== "manual");
        return HttpResponse.json({ ...settingsAppliedMutationFixture, section: "mcp-servers" });
      })
    );
    await openSettings();
    // A workspace collection may inherit this user-owned definition. Its effective source owns deletion.
    await user.click(screen.getByTestId("settings-page-mcp-scope-workspace"));
    await user.click(screen.getByRole("button", { name: "Select github" }));
    await user.click(screen.getByTestId("settings-page-mcp-selection-delete"));
    const confirm = await screen.findByRole("dialog", { name: "Remove github?" });
    await user.click(within(confirm).getByRole("button", { name: "Remove server" }));
    await waitFor(() =>
      expect(screen.queryByRole("button", { name: "Select github" })).not.toBeInTheDocument()
    );
    expect(removed).toHaveLength(1);
    expect(removed[0]!.searchParams.get("owner")).toBeNull();
    expect(removed[0]!.searchParams.get("scope")).toBe("user");
    expect(removed[0]!.searchParams.get("target")).toBe("sidecar");
    expect(removed[0]!.searchParams.get("workspace_id")).toBeNull();
    expect(screen.getByRole("button", { name: "Select github from github" })).toBeVisible();
  });

  // Invariant: a new server whose scope differs from the listed one checks its name against that
  // scope's own list and saves to that scope, so it never overwrites a same-name server there.
  it("Should check and save a new server in the scope picked in the editor", async () => {
    const user = userEvent.setup();
    const listed: string[] = [];
    const saved: URL[] = [];
    server.use(
      http.get("*/api/settings/mcp-servers", ({ request }) => {
        listed.push(new URL(request.url).searchParams.get("scope") ?? "");
        return HttpResponse.json({
          collection: "mcp-servers",
          available_scopes: ["user", "workspace"],
          scope: "user",
          mcp_servers: definitions,
        });
      }),
      http.put("*/api/settings/mcp-servers/:name", ({ request }) => {
        saved.push(new URL(request.url));
        return HttpResponse.json({ ...settingsAppliedMutationFixture, section: "mcp-servers" });
      })
    );
    await openSettings();
    await user.click(screen.getByTestId("settings-page-mcp-scope-workspace"));
    await user.click(await screen.findByTestId("settings-page-mcp-create"));
    const editor = await screen.findByRole("dialog", { name: "Add an MCP server" });
    expect(
      within(editor).getByTestId("settings-mcp-servers-editor-scope-workspace")
    ).toHaveAttribute("aria-checked", "true");

    await user.click(within(editor).getByRole("radio", { name: "All projects" }));
    await user.type(within(editor).getByLabelText("Name"), "github");
    await user.type(within(editor).getByLabelText("URL or command"), "npx server");
    expect(
      await within(editor).findByText('An MCP server named "github" already exists.')
    ).toBeVisible();
    expect(listed).toContain("user");

    await user.clear(within(editor).getByLabelText("Name"));
    await user.type(within(editor).getByLabelText("Name"), "fresh");
    await user.click(within(editor).getByRole("button", { name: "Add server" }));
    await waitFor(() => expect(saved).toHaveLength(1));
    expect(saved[0]!.searchParams.get("scope")).toBe("user");
    expect(saved[0]!.searchParams.get("workspace_id")).toBeNull();
  });

  // Invariant: removing a definition another source also defines says which copy takes over.
  it("Should name the copy that takes over when a removed server is shadowed", async () => {
    const user = userEvent.setup();
    definitions[0] = {
      ...definitions[0]!,
      source_metadata: {
        ...definitions[0]!.source_metadata,
        shadowed_sources: [{ kind: "workspace-config", scope: "workspace" }],
      },
    };
    await openSettings();
    await user.click(screen.getByRole("button", { name: "Select github" }));
    await user.click(screen.getByTestId("settings-page-mcp-selection-delete"));
    const confirm = await screen.findByRole("dialog", { name: "Remove github?" });
    expect(within(confirm).getByTestId("settings-mcp-servers-delete-shadowed")).toHaveTextContent(
      "That one takes over after you remove this one."
    );
  });

  describe("servers by runtime", () => {
    function serveRuntimes(runtimes: RuntimeMCPSnapshot[]) {
      server.use(
        http.get("*/api/settings/runtime-mcp-servers", () =>
          HttpResponse.json(runtimeMCPInventoryFixture(runtimes))
        )
      );
    }

    it("UT-W03: Should offer only runtimes with servers and show just the picked runtime's group", async () => {
      const user = userEvent.setup();
      serveRuntimes([
        claudeRuntimeMCPFixture,
        codexRuntimeMCPFixture,
        { ...piRuntimeMCPFixture, servers: [] },
      ]);
      await openSettings();
      const filter = await screen.findByTestId("settings-mcp-runtime-filter");
      expect(within(filter).getByTestId("settings-mcp-runtime-filter-all")).toBeVisible();
      expect(within(filter).getByTestId("settings-mcp-runtime-filter-compozy")).toBeVisible();
      expect(within(filter).getByTestId("settings-mcp-runtime-filter-claude")).toBeVisible();
      expect(within(filter).getByTestId("settings-mcp-runtime-filter-codex")).toBeVisible();
      expect(
        within(filter).queryByTestId("settings-mcp-runtime-filter-pi")
      ).not.toBeInTheDocument();

      await user.click(within(filter).getByTestId("settings-mcp-runtime-filter-codex"));

      expect(screen.getByTestId("settings-mcp-runtime-group-codex")).toBeVisible();
      expect(screen.queryByTestId("settings-mcp-runtime-group-claude")).not.toBeInTheDocument();
      expect(screen.queryByTestId("settings-page-mcp-servers-list")).not.toBeInTheDocument();
    });

    it("Should say nothing matches when the search empties the picked runtime", async () => {
      const user = userEvent.setup();
      serveRuntimes([claudeRuntimeMCPFixture]);
      await openSettings();
      const filter = await screen.findByTestId("settings-mcp-runtime-filter");
      await user.type(screen.getByRole("searchbox", { name: "Search MCP servers" }), "github");
      await user.click(within(filter).getByTestId("settings-mcp-runtime-filter-claude"));

      expect(await screen.findByTestId("settings-page-mcp-query-empty")).toHaveTextContent(
        'Nothing matches "github".'
      );
      expect(screen.getByTestId("settings-mcp-runtime-filter")).toBeVisible();
    });

    it("UT-W04: Should name the unreadable file and list no rows for that runtime", async () => {
      serveRuntimes([
        {
          ...claudeRuntimeMCPFixture,
          read_state: "unreadable",
          read_error: "Unexpected token at line 14",
          config_files: ["~/.claude.json"],
          servers: [],
        },
        codexRuntimeMCPFixture,
      ]);
      await openSettings();
      const group = await screen.findByTestId("settings-mcp-runtime-group-claude");
      expect(within(group).getByRole("alert")).toHaveTextContent("Couldn't read ~/.claude.json");
      expect(within(group).queryAllByTestId(/^settings-mcp-runtime-row-claude-/)).toHaveLength(0);
      // Other groups are unaffected.
      expect(
        within(screen.getByTestId("settings-mcp-runtime-group-codex")).getByTestId(
          "settings-mcp-runtime-row-codex-github"
        )
      ).toBeVisible();
    });

    it("UT-W05: Should fold a group of 12 to 8 rows behind Show all 12", async () => {
      const user = userEvent.setup();
      serveRuntimes([manyRuntimeMCPFixture(12)]);
      await openSettings();
      const group = await screen.findByTestId("settings-mcp-runtime-group-cursor");
      const rows = () =>
        within(group).getAllByTestId(/^settings-mcp-runtime-row-cursor-server-\d+$/);
      expect(rows()).toHaveLength(8);

      await user.click(within(group).getByRole("button", { name: "Show all 12" }));

      expect(rows()).toHaveLength(12);
      expect(within(group).queryByRole("button", { name: /Show all/ })).not.toBeInTheDocument();
    });

    it("Should fold a runtime group under All and drop the fold once that runtime is picked", async () => {
      const user = userEvent.setup();
      serveRuntimes([claudeRuntimeMCPFixture, codexRuntimeMCPFixture]);
      await openSettings();
      const group = await screen.findByTestId("settings-mcp-runtime-group-codex");
      const fold = within(group).getByTestId("settings-mcp-runtime-group-codex-fold");
      expect(fold).toHaveAttribute("aria-expanded", "true");
      expect(within(group).getByTestId("settings-mcp-runtime-row-codex-github")).toBeVisible();

      await user.click(fold);

      expect(fold).toHaveAttribute("aria-expanded", "false");
      expect(
        within(group).queryByTestId("settings-mcp-runtime-row-codex-github")
      ).not.toBeInTheDocument();
      // The other runtime keeps its own fold state.
      expect(
        within(screen.getByTestId("settings-mcp-runtime-group-claude")).getByTestId(
          "settings-mcp-runtime-group-claude-fold"
        )
      ).toHaveAttribute("aria-expanded", "true");

      await user.click(
        within(screen.getByTestId("settings-mcp-runtime-filter")).getByTestId(
          "settings-mcp-runtime-filter-codex"
        )
      );

      const picked = screen.getByTestId("settings-mcp-runtime-group-codex");
      expect(
        within(picked).queryByTestId("settings-mcp-runtime-group-codex-fold")
      ).not.toBeInTheDocument();
      expect(within(picked).getByTestId("settings-mcp-runtime-row-codex-github")).toBeVisible();
    });

    it("UT-W06: Should keep the Add to CompozyOS panel open with an inline name error on a 409", async () => {
      const user = userEvent.setup();
      const requests: Array<{ url: URL; body: unknown }> = [];
      serveRuntimes([claudeRuntimeMCPFixture]);
      server.use(
        http.post(
          "*/api/settings/runtime-mcp-servers/:runtime/:name/import",
          async ({ request }) => {
            requests.push({ url: new URL(request.url), body: await request.json() });
            return HttpResponse.json(
              {
                error: "A CompozyOS server named exa already exists in Personal",
                code: "mcp_import_name_taken",
              },
              { status: 409 }
            );
          }
        )
      );
      await openSettings();
      await user.click(
        await screen.findByRole("button", { name: "Copy exa from Claude Code to CompozyOS" })
      );
      const dialog = await screen.findByRole("dialog", { name: "Add exa to CompozyOS" });
      expect(within(dialog).getByRole("radio", { name: /All projects/ })).toBeChecked();
      await user.click(within(dialog).getByTestId("mcp-copy-panel-submit"));

      await waitFor(() =>
        expect(within(dialog).getByRole("textbox", { name: "Name in CompozyOS" })).toHaveAttribute(
          "aria-invalid",
          "true"
        )
      );
      expect(dialog).toHaveTextContent("A CompozyOS server named exa already exists in Personal");
      // Still open (jsdom never runs the popover entrance, so it never reads as visible).
      expect(screen.getByRole("dialog", { name: "Add exa to CompozyOS" })).toBeInTheDocument();
      expect(requests).toHaveLength(1);
      expect(requests[0]!.url.pathname).toBe("/api/settings/runtime-mcp-servers/claude/exa/import");
      expect(requests[0]!.body).toEqual({ scope: "user" });
    });

    it("Should name the project in view when copying one of its runtime servers to Personal", async () => {
      const user = userEvent.setup();
      const listed: Array<string | null> = [];
      const bodies: unknown[] = [];
      server.use(
        http.get("*/api/settings/runtime-mcp-servers", ({ request }) => {
          listed.push(new URL(request.url).searchParams.get("workspace_id"));
          return HttpResponse.json(runtimeMCPInventoryFixture([claudeRuntimeMCPFixture]));
        }),
        http.post(
          "*/api/settings/runtime-mcp-servers/:runtime/:name/import",
          async ({ request }) => {
            bodies.push(await request.json());
            return HttpResponse.json(
              { error: "exa is no longer in .mcp.json", code: "mcp_server_not_found" },
              { status: 404 }
            );
          }
        )
      );
      await openSettings();
      await user.click(screen.getByTestId("settings-page-mcp-scope-workspace"));
      await waitFor(() => expect(listed.at(-1)).toBeTruthy());
      const project = listed.at(-1)!;

      await user.click(
        await screen.findByRole("button", { name: "Copy exa from Claude Code to CompozyOS" })
      );
      const dialog = await screen.findByRole("dialog", { name: "Add exa to CompozyOS" });
      await user.click(within(dialog).getByTestId("mcp-copy-panel-submit"));

      await waitFor(() => expect(bodies).toHaveLength(1));
      // A source gone from its file warns inside the panel and blocks another try.
      expect(await within(dialog).findByRole("alert")).toHaveTextContent(
        "exa is no longer in .mcp.json"
      );
      expect(within(dialog).getByTestId("mcp-copy-panel-submit")).toBeDisabled();
      expect(bodies[0]).toEqual({ scope: "user", workspace_id: project });
    });

    it("UT-W07: Should render no switch for a runtime that can't turn its servers off", async () => {
      serveRuntimes([claudeRuntimeMCPFixture, codexRuntimeMCPFixture]);
      await openSettings();
      const codex = await screen.findByTestId("settings-mcp-runtime-group-codex");
      expect(within(codex).queryByRole("switch")).not.toBeInTheDocument();
      expect(
        within(codex).getByTestId("settings-mcp-runtime-group-codex-phrase")
      ).toHaveTextContent("Codex always loads its own servers");
      const claude = screen.getByTestId("settings-mcp-runtime-group-claude");
      expect(
        within(claude).getByRole("switch", {
          name: "Use Claude Code's own MCP servers in CompozyOS sessions",
        })
      ).toBeChecked();
    });

    it("Should review what a waiting project server runs, allow it with its digest, and say so when the file changed", async () => {
      const user = userEvent.setup();
      const approvals: Array<{ name: string; body: unknown }> = [];
      definitions = [
        {
          ...definitions[0]!,
          scope: "workspace",
          workspace_id: "ws-platform",
          callable_in_sessions: false,
          approval: { required: true, state: "waiting", definition_digest: "sha256:seen" },
          source_metadata: {
            available_targets: ["workspace-config"],
            effective_source: {
              kind: "workspace-config",
              scope: "workspace",
              workspace_id: "ws-platform",
            },
          },
        },
      ];
      server.use(
        http.put("*/api/settings/mcp-servers/:name/approval", async ({ request, params }) => {
          approvals.push({ name: String(params.name), body: await request.json() });
          return HttpResponse.json(
            {
              error:
                "github changed since you looked at it. List it again and approve the new definition",
              code: "mcp_approval_definition_changed",
            },
            { status: 409 }
          );
        })
      );
      renderSettings();
      const allow = await screen.findByRole("button", { name: "Allow github" });
      expect(screen.getByText("Added by a file in this project")).toBeVisible();

      await user.click(allow);
      const review = await screen.findByRole("dialog", { name: "Allow github?" });
      expect(within(review).getByTestId("mcp-allow-panel-launch")).toHaveTextContent(
        "npx -y @modelcontextprotocol/server-github"
      );
      expect(approvals).toEqual([]);
      await user.click(within(review).getByTestId("mcp-allow-panel-allow"));

      expect(await screen.findByText("Changed since you allowed it")).toBeVisible();
      expect(screen.queryByRole("dialog", { name: "Allow github?" })).not.toBeInTheDocument();
      expect(screen.getByRole("alert")).toHaveTextContent("github changed since you looked at it");
      expect(approvals).toEqual([
        {
          name: "github",
          body: { workspace_id: "ws-platform", definition_digest: "sha256:seen" },
        },
      ]);
    });
    it("Should say why a server is not working, check it again in place, and close once it works", async () => {
      const user = userEvent.setup();
      const checks: URL[] = [];
      const broken: SettingsMCPServerEntry = {
        ...definitions[0]!,
        runtime_status: {
          state: "runtime_unavailable",
          probe: "failed",
          diagnostic: "spawn npx ENOENT",
        } as SettingsMCPServerEntry["runtime_status"],
      };
      definitions = [broken];
      const answers: SettingsMCPServerEntry[] = [
        {
          ...broken,
          runtime_status: {
            ...broken.runtime_status!,
            diagnostic: "spawn npx ENOENT again",
          },
        },
        {
          ...broken,
          runtime_status: {
            ...broken.runtime_status!,
            state: "ready",
            probe: "succeeded",
            tool_count: 4,
            diagnostic: "",
          },
        },
      ];
      server.use(
        http.post("*/api/settings/mcp-servers/:name/check", ({ request }) => {
          checks.push(new URL(request.url));
          return HttpResponse.json(answers[checks.length - 1]);
        })
      );
      renderSettings();

      await user.click(await screen.findByRole("button", { name: "github is not working" }));
      const panel = await screen.findByRole("dialog", { name: "github isn't working" });
      // Settings is where the panel lives: no "Open in Settings" link back to it.
      expect(within(panel).queryByRole("button", { name: "Open in Settings" })).toBeNull();
      await user.click(within(panel).getByRole("button", { name: "What the server said" }));
      expect(await within(panel).findByText("spawn npx ENOENT")).toBeInTheDocument();

      await user.click(within(panel).getByTestId("mcp-issue-panel-check"));
      expect(await within(panel).findByTestId("mcp-issue-panel-still")).toHaveTextContent(
        "Still not working after the check."
      );
      expect(checks[0]?.pathname).toMatch(/\/api\/settings\/mcp-servers\/github\/check$/);
      expect(checks[0]?.searchParams.get("scope")).toBe(broken.scope);

      await user.click(within(panel).getByTestId("mcp-issue-panel-check"));
      await waitFor(() =>
        expect(screen.queryByRole("dialog", { name: "github isn't working" })).toBeNull()
      );
      // The list took the check's answer without a reread: the row reads Ready, no repair offered.
      expect(screen.getByTestId(`${mcpServerRowTestId(broken)}-status`)).toHaveTextContent("Ready");
      expect(screen.queryByRole("button", { name: "github is not working" })).toBeNull();
      expect(checks).toHaveLength(2);
    });
  });
});
