import { useMarketplaceUpdateRecovery } from "../use-marketplace-update-recovery";
// Invariant: catalog and installed actions retain acquisition identity, consent, and pending ownership.
// Owner: Marketplace action controller; canonical suite: marketplace-action-controller.test.tsx.
// HTTP adapters, navigation, and notifications are the only mocked I/O boundaries.
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  act,
  cleanup,
  fireEvent,
  render,
  renderHook,
  screen,
  waitFor,
} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { setupServer } from "msw/node";
import { http, HttpResponse } from "msw";
import { afterAll, afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import { handlers as profileHandlers } from "@/systems/profiles/mocks";
import { handlers as workspaceHandlers } from "@/systems/workspace/mocks";
import { handlers as statusHandlers } from "@/systems/status/mocks";
import { ExtensionsApiError } from "@/systems/extensions/adapters/extensions-api";
import type { InstalledExtensionView } from "@/systems/extensions";
import { extensionFixtures } from "@/systems/extensions/mocks";
import { marketplaceCatalogFixture, marketplaceCatalogDetailFixture } from "../../mocks";
import { MarketplaceApiError } from "../../adapters/marketplace-api-error";
import { marketplaceCatalogEntryOptions } from "../../lib/query-options";
import type { MarketplaceCatalogListing } from "../../types";
import { MarketplaceInstalledTrail } from "../marketplace-entry-trail";
import { useMarketplaceActionController } from "../use-marketplace-action-controller";

const io = vi.hoisted(() => ({
  install: vi.fn(),
  update: vi.fn(),
  preview: vi.fn(),
  toggle: vi.fn(),
  navigate: vi.fn(),
  success: vi.fn(),
  error: vi.fn(),
}));
vi.mock("../../adapters/marketplace-actions-api", async original => ({
  ...(await original<typeof import("../../adapters/marketplace-actions-api")>()),
  installMarketplaceExtension: io.install,
}));
vi.mock("@/systems/extensions/adapters/extensions-api", async original => ({
  ...(await original<typeof import("@/systems/extensions/adapters/extensions-api")>()),
  updateExtension: io.update,
  previewExtensionInstall: io.preview,
  setExtensionEnablement: io.toggle,
}));
vi.mock("sonner", () => ({ toast: { error: io.error, success: io.success } }));
vi.mock("@tanstack/react-router", async original => ({
  ...(await original<typeof import("@tanstack/react-router")>()),
  useNavigate: () => io.navigate,
}));
const server = setupServer(...profileHandlers, ...workspaceHandlers, ...statusHandlers);
const clients: QueryClient[] = [];
beforeAll(() => server.listen({ onUnhandledRequest: "error" }));
afterAll(() => server.close());
afterEach(() => {
  cleanup();
  clients.splice(0).forEach(client => client.clear());
  server.resetHandlers();
});
beforeEach(() => {
  vi.resetAllMocks();
  io.install.mockResolvedValue({ extension: extensionFixtures[0] });
  io.update.mockResolvedValue(undefined);
  io.toggle.mockResolvedValue({});
  io.preview.mockImplementation(async (request: { ref: string }) => ({
    inputs: [],
    declared_profiles: [{ create: false, credentials: [], name: "default" }],
    name: request.ref.split("/").pop(),
    placements: [],
  }));
});

const verified = marketplaceCatalogFixture.items[0]!;
const unverified = marketplaceCatalogFixture.items[1]!;
const blocked = marketplaceCatalogFixture.items[2]!;
function Harness({
  entries,
  item,
}: {
  entries: MarketplaceCatalogListing[];
  item?: InstalledExtensionView;
}) {
  const actions = useMarketplaceActionController();
  return (
    <>
      {entries.map((entry, index) => (
        <div key={`${entry.source_ref}:${entry.entry_id}`}>
          <button
            type="button"
            onClick={() =>
              entry.update_available ? actions.update(entry) : actions.install(entry)
            }
          >
            Run {index}
          </button>
          <output aria-label={`Pending ${index}`}>
            {actions.isEntryPending(entry) ? "pending" : "idle"}
          </output>
          <output aria-label={`Flash ${index}`}>
            {actions.isEntryFlashing(entry) ? "flashing" : "idle"}
          </output>
          <button type="button" onClick={() => actions.endEntryFlash(entry)}>
            End flash {index}
          </button>
        </div>
      ))}
      {item ? (
        <MarketplaceInstalledTrail
          item={item}
          pending={actions.isItemPending(item)}
          onUpdate={actions.updateInstalled}
          onToggleEnabled={actions.toggleEnabled}
        />
      ) : null}
      {actions.dialogs}
    </>
  );
}
function setup(entries: MarketplaceCatalogListing[], item?: InstalledExtensionView) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  clients.push(client);
  return render(
    <QueryClientProvider client={client}>
      <Harness entries={entries} item={item} />
    </QueryClientProvider>
  );
}
function installed(entry: MarketplaceCatalogListing): InstalledExtensionView {
  return {
    extension: { ...extensionFixtures[0]!, name: "local-kit", enabled: false, marketplace: entry },
    listing: entry,
    updateAvailable: true,
  };
}

describe("useMarketplaceActionController", () => {
  // Invariant: the shared form validates declared fields and submits the approved catalog acquisition once.
  // Owner: Marketplace controller/form integration; canonical suite: marketplace-action-controller.test.tsx.
  it("Should collect typed inputs and submit the approved digest with Enter [UT-039]", async () => {
    io.preview.mockResolvedValueOnce({
      name: "kit",
      inputs: [
        {
          id: "token",
          prompt: "API key",
          type: "secret",
          required: true,
          binding: { type: "env", name: "TOKEN" },
        },
        {
          id: "region",
          prompt: "Region",
          type: "identifier",
          required: false,
          default: "eu",
          binding: { type: "url_query", name: "region" },
        },
        {
          id: "enabled",
          prompt: "Enabled",
          type: "boolean",
          required: true,
          default: false,
          binding: { type: "env", name: "ENABLED" },
        },
      ],
      declared_profiles: [],
      placements: [],
    });
    setup([verified]);
    const user = userEvent.setup();
    await user.click(screen.getByRole("button", { name: "Run 0" }));
    const secret = await screen.findByLabelText("API key");
    expect(secret).toHaveAttribute("type", "password");
    expect(screen.getByLabelText("Region")).toHaveValue("eu");
    expect(screen.getByRole("switch", { name: "Enabled" })).not.toBeChecked();
    expect(screen.getByRole("button", { name: "Install" })).toBeDisabled();
    fireEvent.change(secret, { target: { value: "a".repeat(8193) } });
    expect(await screen.findByText("Too long (max 8 KB)")).toBeVisible();
    expect(secret).toHaveAttribute("aria-invalid", "true");
    expect(screen.getByRole("button", { name: "Install" })).toBeDisabled();
    fireEvent.change(secret, { target: { value: "literal-secret" } });
    await user.click(secret);
    await user.keyboard("{Enter}");
    await waitFor(() => expect(io.install).toHaveBeenCalledOnce());
    expect(io.install).toHaveBeenCalledWith(
      expect.objectContaining({
        source: "curated",
        expected_digest: verified.digest_sha256,
        inputs: {
          token: { value: "literal-secret" },
          region: { value: "eu" },
          enabled: { value: false },
        },
      })
    );
    expect(io.preview).toHaveBeenCalledOnce();
    expect(io.error).not.toHaveBeenCalled();
  });
  it("Should allow every optional field to remain empty [UT-039]", async () => {
    io.preview.mockResolvedValueOnce({
      name: "kit",
      inputs: [
        {
          id: "token",
          prompt: "Optional API key",
          type: "secret",
          required: false,
          binding: { type: "env", name: "TOKEN" },
        },
        {
          id: "enabled",
          prompt: "Enabled",
          type: "boolean",
          required: false,
          binding: { type: "env", name: "ENABLED" },
        },
      ],
      declared_profiles: [],
      placements: [],
    });
    setup([verified]);
    const user = userEvent.setup();
    await user.click(screen.getByRole("button", { name: "Run 0" }));
    expect(await screen.findByRole("button", { name: "Install" })).toBeEnabled();
    await user.click(screen.getByRole("button", { name: "Install" }));
    await waitFor(() => expect(io.install).toHaveBeenCalledOnce());
    expect(io.install.mock.calls[0]![0]).not.toHaveProperty("inputs");
  });
  it("Should open the shared input step after an update and preserve values on failure [UT-039]", async () => {
    const definition = {
      id: "region",
      prompt: "Region",
      type: "identifier",
      required: true,
      binding: { type: "url_query", name: "region" },
    };
    io.update
      .mockRejectedValueOnce(
        new ExtensionsApiError("Configure", 422, "daemon", {
          code: "extension_inputs_required",
          requiredInputs: ["region"],
          inputDefinitions: [definition],
        })
      )
      .mockRejectedValueOnce(new Error("publication failed"))
      .mockResolvedValue(undefined);
    setup([{ ...verified, installed: true, installed_name: "kit", update_available: true }]);
    const user = userEvent.setup();
    await user.click(screen.getByRole("button", { name: "Run 0" }));
    const field = await screen.findByLabelText("Region");
    expect(screen.getByRole("button", { name: "Update" })).toBeDisabled();
    await user.type(field, "eu");
    await user.keyboard("{Enter}");
    await waitFor(() => expect(io.error).toHaveBeenCalledWith("publication failed"));
    expect(screen.getByLabelText("Region")).toHaveValue("eu");
    await user.click(screen.getByRole("button", { name: "Update" }));
    await waitFor(() => expect(io.update).toHaveBeenCalledTimes(3));
    expect(io.update).toHaveBeenLastCalledWith("kit", {
      allow_unverified: false,
      version: verified.version,
      inputs: { region: { value: "eu" } },
    });
    expect(io.preview).not.toHaveBeenCalled();
  });
  it("Should show configuration readiness from the installed extension", () => {
    const item = installed(verified);
    item.extension = { ...item.extension, missing_inputs: ["token"] };
    setup([], item);
    expect(screen.getByText("Needs configuration")).toBeVisible();
    expect(screen.getByRole("switch")).toBeInTheDocument();
  });

  it("Should block forbidden acquisitions before preview or consent", async () => {
    setup([blocked, { ...verified, entry_id: "disabled", installable: false }]);
    const user = userEvent.setup();
    await user.click(screen.getByRole("button", { name: "Run 0" }));
    await user.click(screen.getByRole("button", { name: "Run 1" }));
    expect(io.preview).not.toHaveBeenCalled();
    expect(io.install).not.toHaveBeenCalled();
    expect(screen.queryByTestId("extension-trust-dialog")).not.toBeInTheDocument();
  });
  it("Should pin preview and confirmed install to the listed digest and show the installed destination", async () => {
    io.preview.mockResolvedValueOnce({
      inputs: [],
      declared_profiles: [{ create: true, credentials: [], name: "observability" }],
      name: "otel-bridge",
      network_requirement_digest: "sha256:network",
      placements: [],
    });
    setup([verified]);
    const user = userEvent.setup();
    await user.click(screen.getByRole("button", { name: "Run 0" }));
    expect(await screen.findByRole("heading", { name: "Install otel-bridge" })).toBeVisible();
    expect(screen.getByText("Creates profile observability")).toBeVisible();
    const request = {
      allow_unverified: false,
      expected_digest: verified.digest_sha256,
      ref: verified.install_slug,
      source: "curated",
      version: verified.version,
    };
    expect(io.preview).toHaveBeenCalledWith(request);
    expect(io.install).not.toHaveBeenCalled();
    await user.click(screen.getByRole("button", { name: "Install" }));
    await waitFor(() =>
      expect(io.install).toHaveBeenCalledWith({
        ...request,
        confirm_network_digest: "sha256:network",
      })
    );
    await waitFor(() =>
      expect(screen.getByRole("status", { name: "Flash 0" })).toHaveTextContent("flashing")
    );
    const toast = io.success.mock.calls.find(call => call[0] === "otel-bridge installed");
    expect(toast).toBeDefined();
    toast![1].action.onClick();
    expect(io.navigate).toHaveBeenCalledWith({ search: {}, to: "/marketplace/installed" });
    await user.click(screen.getByRole("button", { name: "End flash 0" }));
    expect(screen.getByRole("status", { name: "Flash 0" })).toHaveTextContent("idle");
  });
  it("Should refetch a changed acquisition and require confirmation of its new digest [UT-038]", async () => {
    const detail = marketplaceCatalogDetailFixture(verified.entry_id)!;
    const digest = "e".repeat(64);
    const current = {
      ...detail,
      entry: { ...detail.entry, digest_sha256: digest, version: "2.0.0" },
    };
    let reads = 0;
    server.use(
      http.get("*/api/marketplace/entries/:entryId", ({ params, request }) => {
        reads++;
        expect(params.entryId).toBe(verified.entry_id);
        expect(new URL(request.url).searchParams.get("source")).toBe(verified.source);
        return HttpResponse.json(current);
      })
    );
    io.install.mockRejectedValueOnce(
      new MarketplaceApiError("The acquired package changed", 409, "extension_source_changed")
    );
    setup([verified]);
    clients
      .at(-1)!
      .setQueryData(
        marketplaceCatalogEntryOptions({ entryId: verified.entry_id, source: verified.source })
          .queryKey,
        detail
      );
    const user = userEvent.setup();
    await user.click(screen.getByRole("button", { name: "Run 0" }));
    await user.click(await screen.findByRole("button", { name: "Install" }));
    await waitFor(() => expect(io.preview).toHaveBeenCalledTimes(2));
    expect(reads).toBe(1);
    expect(io.install).toHaveBeenCalledTimes(1);
    expect(io.preview).toHaveBeenLastCalledWith(
      expect.objectContaining({ expected_digest: digest, version: "2.0.0" })
    );
    expect(io.error).toHaveBeenCalledWith("The acquired package changed", {
      description: "extension_source_changed",
    });
    await waitFor(() => expect(screen.getByRole("button", { name: "Install" })).toBeEnabled());
    await user.click(screen.getByRole("button", { name: "Install" }));
    await waitFor(() => expect(io.install).toHaveBeenCalledTimes(2));
    expect(io.install).toHaveBeenLastCalledWith(
      expect.objectContaining({ expected_digest: digest, version: "2.0.0" })
    );
  });

  it("Should refresh a stale listing rejected during preview before any install", async () => {
    const detail = marketplaceCatalogDetailFixture(verified.entry_id)!;
    const digest = "e".repeat(64);
    let reads = 0;
    server.use(
      http.get("*/api/marketplace/entries/:entryId", () => {
        reads++;
        return HttpResponse.json({ ...detail, entry: { ...detail.entry, digest_sha256: digest } });
      })
    );
    io.preview.mockRejectedValueOnce(
      new ExtensionsApiError("The listing changed", 409, "daemon", {
        code: "extension_source_changed",
      })
    );
    setup([verified]);
    await userEvent.click(screen.getByRole("button", { name: "Run 0" }));
    expect(await screen.findByRole("button", { name: "Install" })).toBeVisible();
    expect(reads).toBe(1);
    expect(io.preview).toHaveBeenCalledTimes(2);
    expect(io.preview).toHaveBeenLastCalledWith(
      expect.objectContaining({ expected_digest: digest })
    );
    expect(io.install).not.toHaveBeenCalled();
    expect(io.error).toHaveBeenCalledWith("The listing changed", {
      description: "extension_source_changed",
    });
  });

  it.each(["blocked", "another origin"])(
    "Should refuse a refreshed acquisition that is %s",
    async reason => {
      const detail = marketplaceCatalogDetailFixture(verified.entry_id)!;
      const entry =
        reason === "blocked"
          ? { ...detail.entry, installable: false, install_blocker: "Publisher blocked by policy" }
          : { ...detail.entry, source_ref: "https://example.com/replaced-catalog" };
      server.use(
        http.get("*/api/marketplace/entries/:entryId", () =>
          HttpResponse.json({ ...detail, entry })
        )
      );
      io.install.mockRejectedValueOnce(
        new MarketplaceApiError("Changed", 409, "extension_source_changed")
      );
      setup([verified]);
      const user = userEvent.setup();
      await user.click(screen.getByRole("button", { name: "Run 0" }));
      await user.click(await screen.findByRole("button", { name: "Install" }));
      await waitFor(() =>
        expect(io.error).toHaveBeenLastCalledWith(
          reason === "blocked"
            ? "Publisher blocked by policy"
            : "The catalog entry now belongs to another origin. Review it before installing."
        )
      );
      expect(screen.queryByRole("button", { name: "Install" })).not.toBeInTheDocument();
      expect(io.install).toHaveBeenCalledTimes(1);
      expect(io.preview).toHaveBeenCalledTimes(1);
    }
  );

  it("Should reject duplicate preview and confirmation presses before a render [UT-038]", async () => {
    let finish!: (value: unknown) => void;
    io.install.mockImplementationOnce(
      () =>
        new Promise(resolve => {
          finish = resolve;
        })
    );
    setup([verified]);
    const start = screen.getByRole("button", { name: "Run 0" });
    act(() => {
      start.click();
      start.click();
    });
    const confirm = await screen.findByRole("button", { name: "Install" });
    expect(io.preview).toHaveBeenCalledTimes(1);
    act(() => {
      confirm.click();
      confirm.click();
    });
    await waitFor(() => expect(io.install).toHaveBeenCalledTimes(1));
    await act(async () => finish({ extension: extensionFixtures[0] }));
    await waitFor(() =>
      expect(screen.queryByRole("button", { name: "Install" })).not.toBeInTheDocument()
    );
    expect(io.success).toHaveBeenCalledTimes(1);
  });

  it("Should close a failed network confirmation attempt and retain its error code [UT-038]", async () => {
    io.install.mockRejectedValueOnce(
      new MarketplaceApiError("The source is unreachable", 503, "source_unreachable")
    );
    setup([verified]);
    const user = userEvent.setup();
    await user.click(screen.getByRole("button", { name: "Run 0" }));
    await user.click(await screen.findByRole("button", { name: "Install" }));
    await waitFor(() =>
      expect(io.error).toHaveBeenCalledWith("The source is unreachable", {
        description: "source_unreachable",
      })
    );
    expect(screen.queryByRole("button", { name: "Install" })).not.toBeInTheDocument();
    expect(screen.getByRole("status", { name: "Pending 0" })).toHaveTextContent("idle");
    expect(io.install).toHaveBeenCalledTimes(1);
    expect(io.preview).toHaveBeenCalledTimes(1);
  });

  it("Should require new trust consent when a refreshed acquisition becomes unverified", async () => {
    const detail = marketplaceCatalogDetailFixture(verified.entry_id)!;
    const entry = { ...detail.entry, digest_sha256: "e".repeat(64), trust: unverified.trust };
    server.use(
      http.get("*/api/marketplace/entries/:entryId", () => HttpResponse.json({ ...detail, entry }))
    );
    io.install.mockRejectedValueOnce(
      new MarketplaceApiError("Changed", 409, "extension_source_changed")
    );
    setup([verified]);
    const user = userEvent.setup();
    await user.click(screen.getByRole("button", { name: "Run 0" }));
    await user.click(await screen.findByRole("button", { name: "Install" }));
    expect(await screen.findByTestId("extension-trust-dialog")).toBeVisible();
    expect(io.install).toHaveBeenCalledTimes(1);
    expect(io.preview).toHaveBeenCalledTimes(1);
  });

  it("Should require unverified consent and preserve a failed preview for retry", async () => {
    setup([unverified]);
    const user = userEvent.setup();
    await user.click(screen.getByRole("button", { name: "Run 0" }));
    expect(await screen.findByTestId("extension-trust-dialog")).toBeVisible();
    expect(io.preview).not.toHaveBeenCalled();
    io.preview.mockRejectedValueOnce(new Error("policy changed"));
    await user.click(screen.getByTestId("extension-trust-confirm"));
    expect(await screen.findByRole("alert")).toHaveTextContent("policy changed");
    expect(io.install).not.toHaveBeenCalled();
    await user.click(screen.getByTestId("extension-trust-confirm"));
    expect(
      await screen.findByRole("heading", {
        name: `Install ${unverified.install_slug!.split("/").pop()}`,
      })
    ).toBeVisible();
    await user.click(screen.getByRole("button", { name: "Install" }));
    await waitFor(() =>
      expect(io.install).toHaveBeenCalledWith({
        allow_unverified: true,
        expected_digest: unverified.digest_sha256,
        ref: unverified.install_slug,
        source: "curated",
        version: unverified.version,
      })
    );
  });
  it("Should update the installed name and gate an unverified update before granting consent", async () => {
    setup([
      { ...verified, installed: true, installed_name: "manifest-otel", update_available: true },
      { ...unverified, installed: true, installed_name: "manifest-slack", update_available: true },
    ]);
    const user = userEvent.setup();
    await user.click(screen.getByRole("button", { name: "Run 0" }));
    await waitFor(() =>
      expect(io.update).toHaveBeenCalledWith("manifest-otel", {
        allow_unverified: false,
        version: verified.version,
      })
    );
    await user.click(screen.getByRole("button", { name: "Run 1" }));
    expect(await screen.findByTestId("extension-trust-dialog")).toBeVisible();
    expect(io.update).toHaveBeenCalledTimes(1);
    await user.click(screen.getByRole("button", { name: "Update anyway" }));
    await waitFor(() =>
      expect(io.update).toHaveBeenLastCalledWith("manifest-slack", {
        allow_unverified: true,
        version: unverified.version,
      })
    );
    expect(io.install).not.toHaveBeenCalled();
  });
  it("Should resume a refused update with the exact network digest", async () => {
    io.update.mockRejectedValueOnce(
      new ExtensionsApiError("network confirmation required", 409, "daemon", {
        code: "extension_network_confirmation_required",
        currentDigest: "sha256:quick-update",
      })
    );
    setup([
      { ...verified, installed: true, installed_name: "manifest-otel", update_available: true },
    ]);
    const user = userEvent.setup();
    await user.click(screen.getByRole("button", { name: "Run 0" }));
    expect(await screen.findByTestId("extension-network-confirm-dialog")).toBeVisible();
    await user.click(screen.getByTestId("extension-network-confirm-accept"));
    await waitFor(() =>
      expect(io.update).toHaveBeenLastCalledWith("manifest-otel", {
        allow_unverified: false,
        version: verified.version,
        confirm_network_digest: "sha256:quick-update",
      })
    );
    await waitFor(() =>
      expect(screen.queryByTestId("extension-network-confirm-dialog")).not.toBeInTheDocument()
    );
  });
  it("Should disable installed actions while an update is pending and toggle the local name", async () => {
    let finish!: () => void;
    io.update.mockReturnValueOnce(
      new Promise<void>(resolve => {
        finish = resolve;
      })
    );
    const item = installed(verified);
    setup([], item);
    const user = userEvent.setup();
    const update = screen.getByRole("button", { name: `Update ${verified.name}` });
    const toggle = screen.getByRole("switch", { name: `Enable ${verified.name}` });
    await user.click(update);
    await waitFor(() => expect(update).toBeDisabled());
    expect(toggle).toHaveAttribute("aria-disabled", "true");
    await user.click(toggle);
    expect(io.toggle).not.toHaveBeenCalled();
    await user.click(update);
    expect(io.update).toHaveBeenCalledTimes(1);
    expect(io.update).toHaveBeenCalledWith("local-kit", {
      allow_unverified: false,
      version: verified.version,
    });
    await act(async () => finish());
    await waitFor(() => expect(toggle).not.toHaveAttribute("aria-disabled", "true"));
    await user.click(toggle);
    await waitFor(() => expect(io.toggle).toHaveBeenCalledWith("local-kit", "default", true));
    expect(screen.queryByTestId("extension-network-confirm-dialog")).not.toBeInTheDocument();
  });
  it("Should require fresh consent for an unverified installed-row update", async () => {
    const item = installed(unverified);
    item.extension = { ...extensionFixtures[1]!, name: "local-kit", marketplace: unverified };
    setup([], item);
    const user = userEvent.setup();
    await user.click(screen.getByRole("button", { name: `Update ${unverified.name}` }));
    expect(io.update).not.toHaveBeenCalled();
    expect(await screen.findByRole("heading", { name: "Update local-kit?" })).toBeVisible();
    await user.click(screen.getByRole("button", { name: "Update anyway" }));
    await waitFor(() =>
      expect(io.update).toHaveBeenCalledWith("local-kit", {
        allow_unverified: true,
        version: unverified.version,
      })
    );
    await waitFor(() =>
      expect(screen.queryByTestId("extension-trust-dialog")).not.toBeInTheDocument()
    );
  });

  it("Should release pending state after acquisition failure", async () => {
    io.preview.mockRejectedValueOnce(new Error("registry unavailable"));
    setup([verified]);
    const user = userEvent.setup();
    await user.click(screen.getByRole("button", { name: "Run 0" }));
    await waitFor(() => expect(io.error).toHaveBeenCalledWith("registry unavailable"));
    expect(screen.getByRole("status", { name: "Pending 0" })).toHaveTextContent("idle");
  });
  it("Should isolate pending actions with the same entry id in different origins", async () => {
    let first!: (value: unknown) => void;
    let second!: (value: unknown) => void;
    io.preview
      .mockReturnValueOnce(
        new Promise(resolve => {
          first = resolve;
        })
      )
      .mockReturnValueOnce(
        new Promise(resolve => {
          second = resolve;
        })
      );
    setup([verified, { ...verified, source: "team", source_ref: "git:team" }]);
    const user = userEvent.setup();
    await user.click(screen.getByRole("button", { name: "Run 0" }));
    expect(screen.getByRole("status", { name: "Pending 0" })).toHaveTextContent("pending");
    expect(screen.getByRole("status", { name: "Pending 1" })).toHaveTextContent("idle");
    await user.click(screen.getByRole("button", { name: "Run 1" }));
    const preview = { name: "kit", inputs: [], declared_profiles: [], placements: [] };
    await act(async () => first(preview));
    await waitFor(() =>
      expect(screen.getByRole("status", { name: "Pending 0", hidden: true })).toHaveTextContent(
        "idle"
      )
    );
    expect(screen.getByRole("status", { name: "Pending 1", hidden: true })).toHaveTextContent(
      "pending"
    );
    await act(async () => second(preview));
  });
  it("Should show portable-package consent without permission grants", async () => {
    const portable = marketplaceCatalogFixture.items.find(
      entry => entry.format === "agent-plugin"
    )!;
    setup([portable]);
    const user = userEvent.setup();
    await user.click(screen.getByRole("button", { name: "Run 0" }));
    const dialog = await screen.findByTestId("extension-trust-dialog");
    expect(dialog).toHaveTextContent("Unsigned package");
    expect(dialog).not.toHaveTextContent(/permission|grant/i);
    expect(io.install).not.toHaveBeenCalled();
    await user.click(screen.getByTestId("extension-trust-confirm"));
    await screen.findByRole("heading", {
      name: `Install ${portable.install_slug!.split("/").pop()}`,
    });
    await user.click(screen.getByRole("button", { name: "Install" }));
    await waitFor(() =>
      expect(io.install).toHaveBeenCalledWith({
        allow_unverified: true,
        expected_digest: portable.digest_sha256,
        ref: portable.install_slug,
        source: "curated",
        version: portable.version,
      })
    );
  });
});

// Invariant: input/network recovery retries the same scoped update without losing prior fields or consent.
// Owner: Marketplace update controller; canonical suite: marketplace-action-controller.test.tsx.
describe("Marketplace update recovery", () => {
  const definitions = [
    {
      id: "region",
      prompt: "Region",
      type: "identifier",
      required: true,
      binding: { type: "url_query", name: "region" },
    },
  ];
  it("Should preserve scope and prior inputs across network and input confirmation", async () => {
    const mutate = vi
      .fn()
      .mockRejectedValueOnce(
        new ExtensionsApiError("Confirm network", 409, "daemon", {
          code: "extension_network_confirmation_required",
          currentDigest: "a".repeat(64),
        })
      )
      .mockRejectedValueOnce(
        new ExtensionsApiError("Configure", 422, "daemon", {
          code: "extension_inputs_required",
          requiredInputs: ["region"],
          inputDefinitions: definitions,
        })
      )
      .mockResolvedValue(undefined);
    const notify = vi.fn();
    const { result } = renderHook(() => useMarketplaceUpdateRecovery(mutate, notify));
    const request = {
      name: "kit",
      body: {
        scope: "workspace" as const,
        workspace_id: "ws-one",
        profile: "growth",
        allow_unverified: true,
        inputs: { token: { vault_ref: "vault:extensions/kit/TOKEN" } },
      },
    };
    await act(async () => {
      await result.current.runUpdate("Kit", request, action => action());
    });
    expect(result.current.recovery?.kind).toBe("network");
    await act(async () => {
      result.current.confirm();
    });
    expect(result.current.recovery?.kind).toBe("inputs");
    act(() => result.current.confirm({}));
    expect(mutate).toHaveBeenCalledTimes(2);
    await act(async () => {
      result.current.confirm({ region: "eu" });
      result.current.confirm({ region: "eu" });
    });
    expect(mutate).toHaveBeenCalledTimes(3);
    expect(mutate).toHaveBeenLastCalledWith({
      ...request,
      body: {
        ...request.body,
        confirm_network_digest: "a".repeat(64),
        inputs: { ...request.body.inputs, region: { value: "eu" } },
      },
    });
    expect(result.current.recovery).toBeNull();
    expect(notify).toHaveBeenCalledOnce();
  });
  it("Should preserve entered inputs when another required field appears", async () => {
    const missing = (id: string) =>
      new MarketplaceApiError("Configure", 422, "extension_inputs_required", false, {
        requiredInputs: [id],
        inputDefinitions: [{ ...definitions[0]!, id }],
      });
    const mutate = vi
      .fn()
      .mockRejectedValueOnce(missing("region"))
      .mockRejectedValueOnce(missing("project"))
      .mockResolvedValue(undefined);
    const { result } = renderHook(() => useMarketplaceUpdateRecovery(mutate, vi.fn()));
    await act(async () => {
      await result.current.runUpdate("Kit", { name: "kit", body: {} }, action => action());
    });
    await act(async () => {
      result.current.confirm({ region: "eu" });
    });
    await act(async () => {
      result.current.confirm({ project: "app" });
    });
    expect(mutate).toHaveBeenLastCalledWith({
      name: "kit",
      body: { inputs: { region: { value: "eu" }, project: { value: "app" } } },
    });
  });
  it("Should keep a refused update editable and prevent dismissal while submitting", async () => {
    let rejectUpdate!: (error: Error) => void;
    const mutate = vi
      .fn()
      .mockRejectedValueOnce(
        new ExtensionsApiError("Configure", 422, "daemon", {
          code: "extension_inputs_required",
          requiredInputs: ["region"],
          inputDefinitions: definitions,
        })
      )
      .mockImplementationOnce(
        () =>
          new Promise((_resolve, reject) => {
            rejectUpdate = reject;
          })
      );
    const { result } = renderHook(() => useMarketplaceUpdateRecovery(mutate, vi.fn()));
    await act(async () => {
      await result.current.runUpdate("Kit", { name: "kit", body: {} }, action => action());
    });
    act(() => {
      result.current.confirm({ region: "eu" });
      result.current.dismiss();
    });
    expect(result.current.pending).toBe(true);
    expect(result.current.recovery?.kind).toBe("inputs");
    await act(async () => {
      rejectUpdate(new Error("publication failed"));
    });
    expect(result.current.pending).toBe(false);
    expect(result.current.recovery?.kind).toBe("inputs");
    act(() => result.current.dismiss());
    expect(result.current.recovery).toBeNull();
  });
});
