// Invariant: Setup labels key off the daemon `state`, saves only typed fields, and reports restart,
// remaining missing inputs, daemon-env inputs and field errors (T-13).
// Owner: the Setup dialog composition; HTTP is the mocked I/O boundary (MSW).
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { HttpResponse, http } from "msw";
import { setupServer } from "msw/node";
import { afterAll, afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";

import { toast } from "@compozy/ui";

import type { ExtensionEntry } from "@/systems/extensions";
import { extensionFixtures } from "@/systems/extensions/mocks";
import { ExtensionSetupDialog } from "../extension-setup-dialog";

type SetupInput = {
  id: string;
  type: string;
  required: boolean;
  state: string;
  set?: boolean;
  targets?: string[];
};

const binding = (name: string) => ({ type: "env", name });

function inputsPayload(inputs: SetupInput[], extra: Record<string, unknown> = {}) {
  return {
    extension: "acme-tool",
    workspace_id: "",
    profile: "default",
    enabled: true,
    inputs: inputs.map(({ targets = ["subprocess"], ...input }) => ({
      ...input,
      prompt: `Prompt ${input.id}`,
      binding: binding(input.id.toUpperCase()),
      targets,
    })),
    missing: inputs.filter(input => input.state === "missing").map(input => input.id),
    invalid: inputs.filter(input => input.state === "invalid").map(input => input.id),
    daemon_env: inputs.filter(input => input.state === "daemon_env").map(input => input.id),
    missing_env: [],
    mcp_auth_servers: [],
    ...extra,
  };
}

const TOKEN: SetupInput = { id: "api_token", type: "secret", required: true, state: "set" };
const REGION: SetupInput = { id: "region", type: "string", required: true, state: "missing" };
// The status payload's legacy boolean says `set`; the label must follow `state`.
const LEGACY: SetupInput = {
  id: "legacy_key",
  type: "secret",
  required: true,
  state: "daemon_env",
  set: true,
};

const requests = { put: [] as unknown[], get: [] as URL[] };
let readPayload = inputsPayload([TOKEN, REGION, LEGACY]);
let writeResponse: () => Response | Promise<Response> = () => HttpResponse.json(readPayload);

const server = setupServer(
  http.get("*/api/extensions/:name/inputs", ({ request }) => {
    requests.get.push(new URL(request.url));
    return HttpResponse.json(readPayload);
  }),
  http.put("*/api/extensions/:name/inputs", async ({ request }) => {
    requests.put.push(await request.json());
    return writeResponse();
  })
);

const clients: QueryClient[] = [];
beforeAll(() => server.listen({ onUnhandledFrame: "error" }));
afterAll(() => server.close());
afterEach(() => {
  cleanup();
  clients.splice(0).forEach(client => client.clear());
  server.resetHandlers();
  vi.restoreAllMocks();
});
beforeEach(() => {
  requests.put = [];
  requests.get = [];
  readPayload = inputsPayload([TOKEN, REGION, LEGACY]);
  writeResponse = () => HttpResponse.json(readPayload);
});

function extension(): ExtensionEntry {
  return { ...extensionFixtures[0]!, name: "acme-tool", profile: "default", mcp_servers: [] };
}

async function renderDialog() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  clients.push(client);
  const onClose = vi.fn();
  const view = (entry: ExtensionEntry) => (
    <QueryClientProvider client={client}>
      <ExtensionSetupDialog extension={entry} onClose={onClose} />
    </QueryClientProvider>
  );
  const { rerender } = render(view(extension()));
  await screen.findByTestId("extension-input-region");
  return {
    onClose,
    rerenderWith: (entry: ExtensionEntry) => rerender(view(entry)),
    user: userEvent.setup(),
  };
}

function field(id: string): HTMLElement {
  const control = screen.getByTestId(`extension-input-${id}`);
  const container = control.closest<HTMLElement>('[data-slot="field"]');
  if (!container) throw new Error(`extension-input-${id} must render inside a Field`);
  return container;
}

describe("ExtensionSetupDialog", () => {
  it("Should key every label off state so a daemon_env input is marked missing, never Set", async () => {
    await renderDialog();

    expect(requests.get[0]?.searchParams.get("scope")).toBe("global");
    expect(within(field("api_token")).getByText("Set")).toBeInTheDocument();
    expect(screen.getByTestId("extension-input-api_token")).toHaveValue("");
    expect(within(field("region")).getByText("Missing")).toBeInTheDocument();
    const legacy = field("legacy_key");
    expect(within(legacy).getByText("Missing")).toBeInTheDocument();
    expect(within(legacy).queryByText("Set")).not.toBeInTheDocument();
    expect(legacy).toHaveTextContent(
      "Uses the daemon environment; the extension process does not receive it."
    );
  });

  it("Should save only edited fields and report the restart", async () => {
    const success = vi.spyOn(toast, "success");
    writeResponse = () =>
      HttpResponse.json(
        inputsPayload([TOKEN, { ...REGION, state: "set" }], { changed: ["region"], reloaded: true })
      );
    const { onClose, user } = await renderDialog();

    await user.type(screen.getByTestId("extension-input-region"), "eu-west");
    await user.click(screen.getByTestId("extension-setup-save"));

    await waitFor(() => expect(onClose).toHaveBeenCalledOnce());
    expect(requests.put).toEqual([{ inputs: { region: { value: "eu-west" } } }]);
    expect(success).toHaveBeenCalledWith("Setup saved");
    expect(success).toHaveBeenCalledWith("Restarted acme-tool");
  });

  it("Should keep a daemon_env input unbound when saved without a value and say so", async () => {
    const info = vi.spyOn(toast, "info");
    readPayload = inputsPayload([TOKEN, LEGACY, { ...REGION, state: "set" }]);
    const { onClose, user } = await renderDialog();

    await user.click(screen.getByTestId("extension-setup-save"));

    await waitFor(() => expect(onClose).toHaveBeenCalledOnce());
    expect(requests.put).toEqual([{ inputs: {} }]);
    expect(info).toHaveBeenCalledWith(
      "legacy_key still using the daemon environment; the extension process does not receive it."
    );
  });

  it("Should say an MCP-only daemon_env value is stored for the MCP server", async () => {
    const info = vi.spyOn(toast, "info");
    readPayload = inputsPayload([
      TOKEN,
      { ...LEGACY, targets: ["mcp:acme"] },
      { ...REGION, state: "set" },
    ]);
    const { onClose, user } = await renderDialog();

    const legacy = field("legacy_key");
    expect(legacy).toHaveTextContent(
      "Uses the daemon environment. A value saved here is stored for the MCP server."
    );
    expect(legacy).not.toHaveTextContent("the extension process does not receive it");

    await user.click(screen.getByTestId("extension-setup-save"));

    await waitFor(() => expect(onClose).toHaveBeenCalledOnce());
    expect(info).not.toHaveBeenCalled();
  });

  it("Should stay open and list required inputs that are still missing", async () => {
    const { onClose, user } = await renderDialog();

    await user.type(screen.getByTestId("extension-input-api_token"), "rotated");
    await user.click(screen.getByTestId("extension-setup-save"));

    expect(await screen.findByTestId("extension-setup-remaining")).toHaveTextContent("region");
    expect(requests.put).toEqual([{ inputs: { api_token: { value: "rotated" } } }]);
    expect(onClose).not.toHaveBeenCalled();
    expect(screen.getByTestId("extension-setup-dialog")).toBeInTheDocument();
  });

  it("Should show the daemon's input error on the field it names", async () => {
    writeResponse = () =>
      HttpResponse.json(
        {
          error: "region must be an identifier",
          code: "extension_input_invalid",
          input_id: "region",
        },
        { status: 422 }
      );
    const { onClose, user } = await renderDialog();

    await user.type(screen.getByTestId("extension-input-region"), "eu west");
    await user.click(screen.getByTestId("extension-setup-save"));

    expect(await screen.findByTestId("extension-input-region-error")).toHaveTextContent(
      "region must be an identifier"
    );
    expect(screen.queryByTestId("extension-setup-save-error")).not.toBeInTheDocument();
    expect(onClose).not.toHaveBeenCalled();

    await user.type(screen.getByTestId("extension-input-region"), "x");

    expect(screen.queryByTestId("extension-input-region-error")).not.toBeInTheDocument();
  });

  it("Should start another installed instance with an empty draft and no save feedback", async () => {
    const { rerenderWith, user } = await renderDialog();
    await user.type(screen.getByTestId("extension-input-api_token"), "rotated");
    await user.click(screen.getByTestId("extension-setup-save"));
    await screen.findByTestId("extension-setup-remaining");
    await user.type(screen.getByTestId("extension-input-region"), "draft");

    rerenderWith({ ...extension(), workspace_id: "ws-other" });

    await waitFor(() => expect(screen.getByTestId("extension-input-region")).toHaveValue(""));
    expect(screen.queryByTestId("extension-setup-remaining")).not.toBeInTheDocument();
  });

  it("Should keep a failed save of one instance off another instance", async () => {
    writeResponse = () => HttpResponse.json({ error: "vault is locked" }, { status: 500 });
    const { rerenderWith, user } = await renderDialog();
    await user.type(screen.getByTestId("extension-input-api_token"), "rotated");
    await user.click(screen.getByTestId("extension-setup-save"));
    expect(await screen.findByTestId("extension-setup-save-error")).toBeInTheDocument();

    rerenderWith({ ...extension(), workspace_id: "ws-other" });

    await waitFor(() =>
      expect(screen.queryByTestId("extension-setup-save-error")).not.toBeInTheDocument()
    );
  });

  it("Should ignore a save that completes after the dialog switched instance", async () => {
    let release = () => {};
    writeResponse = () =>
      new Promise<Response>(resolve => {
        release = () => resolve(HttpResponse.json(inputsPayload([{ ...REGION, state: "set" }])));
      });
    const success = vi.spyOn(toast, "success");
    const { onClose, rerenderWith, user } = await renderDialog();
    await user.type(screen.getByTestId("extension-input-region"), "eu-west");
    await user.click(screen.getByTestId("extension-setup-save"));
    await waitFor(() => expect(requests.put).toHaveLength(1));

    rerenderWith({ ...extension(), workspace_id: "ws-other" });
    await waitFor(() => expect(screen.getByTestId("extension-input-region")).toHaveValue(""));
    release();

    await waitFor(() => expect(success).toHaveBeenCalledWith("Setup saved"));
    expect(onClose).not.toHaveBeenCalled();
    expect(screen.getByTestId("extension-setup-dialog")).toBeInTheDocument();
  });
});
