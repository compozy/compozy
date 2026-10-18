// Suite: Continue dialog (S4)
// Invariant: the dialog states what travels from the daemon's own preview before anything is
// created, gates Continue on that measurement, preselects another agent, offers declared routes
// only when the agent has them (route XOR runtime on the wire), and never opens a window for an
// outcome whose child was deleted.
// Owning layer: SessionContinueDialog with its hooks over MSW. Canonical suite: this file.
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { HttpHandler } from "msw";
import { afterEach, beforeEach, describe, expect, it, onTestFinished, vi } from "vitest";

import { UIProvider } from "@compozy/ui";
import type { AgentPayload } from "@/systems/agent";
import type { RuntimeModelOption, RuntimeProviderOption } from "@/systems/runtime";
import { createMswFetch } from "@/test/msw-fetch";

import type { ContinueSessionRequest } from "../../adapters/session-derive-api";
import {
  continuedSessionFixture,
  deriveAgentsFixture,
  deriveResultFixture,
  deriveRoutedAgentsFixture,
  deriveSourceSessionFixture,
  deriveTruncatedPreviewFixture,
} from "../../mocks/derive-fixtures";
import {
  sessionDeriveHandlers,
  type SessionDeriveHandlerOptions,
} from "../../mocks/derive-handlers";
import type { SessionPayload } from "../../types";
import { SessionContinueDialog } from "../session-continue-dialog";

const agents = vi.hoisted(() => ({ data: [] as AgentPayload[] }));
const runtimeCatalog = vi.hoisted(() => ({
  providers: [] as RuntimeProviderOption[],
  models: [] as RuntimeModelOption[],
}));

vi.mock("@/systems/agent", async importOriginal => ({
  ...(await importOriginal<object>()),
  useAgents: () => ({ data: agents.data, isLoading: false }),
}));
vi.mock("../../hooks/use-session-continue-runtime-options", () => ({
  useSessionContinueRuntimeOptions: () => ({
    providers: runtimeCatalog.providers,
    models: runtimeCatalog.models,
    loading: false,
    refreshing: false,
    refresh: () => undefined,
    catalogError: null,
  }),
}));

const source = deriveSourceSessionFixture;
const workspaceId = source.workspace_id ?? "";
let handlers: HttpHandler[] = [];

function renderDialog(options: SessionDeriveHandlerOptions = {}) {
  handlers = sessionDeriveHandlers(options);
  const openInNewWindow = vi.fn<(child: SessionPayload) => void>();
  const openInThisWindow = vi.fn<(child: SessionPayload) => void>();
  const onOpenChange = vi.fn();
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  render(
    <QueryClientProvider client={queryClient}>
      <UIProvider reducedMotion="never" skipAnimations>
        <SessionContinueDialog
          onOpenChange={onOpenChange}
          open
          placement={{ openInNewWindow, openInThisWindow }}
          source={source}
          workspaceId={workspaceId}
        />
      </UIProvider>
    </QueryClientProvider>
  );
  return { openInNewWindow, openInThisWindow, onOpenChange };
}

describe("SessionContinueDialog", () => {
  beforeEach(() => {
    agents.data = deriveAgentsFixture;
    runtimeCatalog.providers = [];
    runtimeCatalog.models = [];
    vi.stubGlobal(
      "fetch",
      createMswFetch(() => handlers)
    );
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("Should preselect another agent and enable Continue once the preview is measured", async () => {
    renderDialog();

    expect(screen.getByTestId("session-derive-preview")).toHaveTextContent("Measuring…");
    expect(screen.getByTestId("session-continue-submit")).toBeDisabled();

    await waitFor(() =>
      expect(screen.getByTestId("session-derive-preview")).toHaveTextContent(
        "Carries over 42 messages · 61.3 KiB"
      )
    );
    expect(screen.getByTestId("session-continue-agent-select")).toHaveTextContent("codex");
    expect(screen.getByTestId("session-continue-runtime-select")).toBeInTheDocument();
    expect(screen.queryByTestId("session-continue-route-select")).not.toBeInTheDocument();
    expect(screen.getByTestId("session-continue-submit")).toBeEnabled();
    expect(screen.getByTestId("session-continue-source-note")).toHaveTextContent(
      "From Refactor flaky manager tests · claude"
    );
  });

  it("Should disable Continue when the preview cannot be measured", async () => {
    renderDialog({ preview: "error" });

    await waitFor(() =>
      expect(screen.getByTestId("session-derive-preview")).toHaveTextContent(
        "Couldn't measure this session's context."
      )
    );
    expect(screen.getByTestId("session-continue-submit")).toBeDisabled();
  });

  it("Should state the omitted count and keep Continue enabled for a truncated preview", async () => {
    renderDialog({ preview: deriveTruncatedPreviewFixture });

    await waitFor(() =>
      expect(screen.getByTestId("session-derive-preview")).toHaveTextContent(
        "Carries over 30 of 42 messages · 128 KiB"
      )
    );
    expect(screen.getByTestId("session-derive-preview-omitted")).toHaveTextContent(
      "12 earlier messages omitted to fit the context budget."
    );
    expect(screen.getByTestId("session-continue-submit")).toBeEnabled();
  });

  it("Should label colliding routes by account and send the route without a runtime", async () => {
    const user = userEvent.setup();
    agents.data = [...deriveRoutedAgentsFixture].reverse();
    const sent: ContinueSessionRequest[] = [];
    renderDialog({ onContinue: request => sent.push(request) });

    await waitFor(() => expect(screen.getByTestId("session-continue-submit")).toBeEnabled());
    // codex is first now; claude is the source's own agent, so pick it explicitly.
    await user.click(screen.getByTestId("session-continue-agent-select"));
    await user.click(await screen.findByRole("option", { name: /claude/ }));

    await user.click(screen.getByTestId("session-continue-route-select"));
    const options = await screen.findAllByRole("option");
    expect(options.map(option => option.textContent)).toEqual([
      "Default",
      "Route 1 · claude · opus",
      "Route 2 · claude · opus · 3f9a…",
    ]);
    await user.click(options[2]!);
    expect(screen.queryByTestId("session-continue-runtime-select")).not.toBeInTheDocument();

    await user.click(screen.getByTestId("session-continue-submit"));
    await waitFor(() => expect(sent).toHaveLength(1));
    expect(sent[0]).toMatchObject({ agent_name: "claude", route: 2 });
    expect(sent[0]).not.toHaveProperty("runtime");
  });

  // Invariant: an explicit speed choice travels as-is. Omitting `normal` would let the
  // daemon restore a fast-default agent's speed, so the child would run fast anyway.
  it("Should send normal speed when a fast-default agent is switched to normal", async () => {
    const user = userEvent.setup();
    agents.data = deriveAgentsFixture.map(agent =>
      agent.name === "codex"
        ? {
            ...agent,
            model: "gpt-5.4",
            speed: "fast",
            effective_runtime: {
              provider: "codex",
              model: "gpt-5.4",
              speed: "fast",
              sources: { provider: "agent", model: "agent", speed: "agent" },
            },
          }
        : agent
    );
    runtimeCatalog.providers = [{ id: "codex", name: "Codex", runtime_provider: "codex" }];
    runtimeCatalog.models = [
      {
        id: "gpt-5.4",
        provider: "codex",
        name: "gpt-5.4",
        efforts: [],
        availability: "live",
        curated: true,
        configurations: [{ reasoning_effort: "", fast: true }],
      },
    ];
    const sent: ContinueSessionRequest[] = [];
    renderDialog({ onContinue: request => sent.push(request) });

    await waitFor(() => expect(screen.getByTestId("session-continue-submit")).toBeEnabled());
    await user.click(screen.getByTestId("session-continue-runtime-select"));
    const speedSwitch = await screen.findByTestId("runtime-selector-speed");
    expect(speedSwitch).toHaveAttribute("aria-checked", "true");
    await user.click(speedSwitch);
    await user.keyboard("{Escape}");

    await user.click(screen.getByTestId("session-continue-submit"));
    await waitFor(() => expect(sent).toHaveLength(1));
    expect(sent[0]?.runtime).toMatchObject({ provider: "codex", speed: "normal" });
  });

  it("Should open a new window by default and never the current one", async () => {
    const user = userEvent.setup();
    const sent: ContinueSessionRequest[] = [];
    const { openInNewWindow, openInThisWindow, onOpenChange } = renderDialog({
      onContinue: request => sent.push(request),
    });

    await waitFor(() => expect(screen.getByTestId("session-continue-submit")).toBeEnabled());
    await user.type(screen.getByTestId("session-continue-message"), "  pick up the store tests ");
    await user.click(screen.getByTestId("session-continue-submit"));

    await waitFor(() => expect(openInNewWindow).toHaveBeenCalledTimes(1));
    expect(openInNewWindow.mock.calls[0]?.[0]).toMatchObject({ id: "sess_continued_child" });
    expect(openInThisWindow).not.toHaveBeenCalled();
    expect(onOpenChange).toHaveBeenCalledWith(false);
    expect(sent[0]).toMatchObject({ agent_name: "codex", message: "pick up the store tests" });
    expect(sent[0]?.idempotency_key).toMatch(/^web-derive-/);
  });

  it("Should open nothing when the recorded child was deleted", async () => {
    const user = userEvent.setup();
    const deleted = deriveResultFixture(continuedSessionFixture(), { child_deleted: true });
    const { openInNewWindow, openInThisWindow } = renderDialog({
      result: { derived: { ...deleted.derived, replayed: true } },
    });

    await waitFor(() => expect(screen.getByTestId("session-continue-submit")).toBeEnabled());
    await user.click(screen.getByTestId("session-continue-submit"));

    expect(await screen.findByTestId("session-continue-submit-error")).toHaveTextContent(
      "Nothing new was created."
    );
    expect(openInNewWindow).not.toHaveBeenCalled();
    expect(openInThisWindow).not.toHaveBeenCalled();
  });

  it("Should retry with the same idempotency key after a failure", async () => {
    // A 422 can follow the commit (the first message's admission failed after the child
    // exists); a new key on retry would create a second child.
    const user = userEvent.setup();
    const sent: ContinueSessionRequest[] = [];
    renderDialog({
      onContinue: request => sent.push(request),
      result: { status: 422, error: "admit first message failed", code: "model_unavailable" },
    });

    await waitFor(() => expect(screen.getByTestId("session-continue-submit")).toBeEnabled());
    await user.click(screen.getByTestId("session-continue-submit"));
    expect(await screen.findByTestId("session-continue-submit-error")).toHaveTextContent(
      "admit first message failed"
    );
    await user.click(screen.getByTestId("session-continue-submit"));

    await waitFor(() => expect(sent).toHaveLength(2));
    expect(sent[1]?.idempotency_key).toBe(sent[0]?.idempotency_key);
  });

  it("Should offer to open the session a post-commit refusal already created", async () => {
    // The daemon created the child, then refused its first message: the error names the child
    // (child_session_id) and the dialog opens it where the operator chose instead of retrying.
    const user = userEvent.setup();
    const child = continuedSessionFixture(source);
    const { openInNewWindow, onOpenChange } = renderDialog({
      result: {
        status: 422,
        error: "provider not authenticated",
        child_session_id: child.id,
      },
      committedChild: child,
    });

    await waitFor(() => expect(screen.getByTestId("session-continue-submit")).toBeEnabled());
    await user.click(screen.getByTestId("session-continue-submit"));
    expect(await screen.findByTestId("session-continue-submit-error")).toHaveTextContent(
      "provider not authenticated"
    );
    expect(screen.getByTestId("session-continue-committed-child")).toHaveTextContent(
      "The new session was already created."
    );
    await user.click(screen.getByTestId("session-continue-open-committed-child"));

    await waitFor(() => expect(openInNewWindow).toHaveBeenCalledTimes(1));
    expect(openInNewWindow.mock.calls[0]?.[0].id).toBe(child.id);
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });

  // Invariant: a post-commit runtime refusal reads as its structured diagnostic, never the
  // daemon's wrapped error chain or the agent's stderr, above the offer to open the child.
  it("Should show a post-commit refusal's diagnostic instead of the raw error chain", async () => {
    const user = userEvent.setup();
    const child = continuedSessionFixture(source);
    renderDialog({
      result: {
        status: 422,
        error:
          'session: admit first message of derived session "sess-x": acp: start failed after ' +
          'session acceptance: acp: model "gone" is unavailable: stderr=INFO connection closed',
        code: "model_unavailable",
        child_session_id: child.id,
        diagnostic: {
          id: "provider.negotiation.model_unavailable",
          code: "model_unavailable",
          category: "provider",
          severity: "error",
          data_freshness: "live",
          title: "Provider configuration is unavailable",
          message: 'acp: model "gone" is unavailable',
        },
      },
      committedChild: child,
    });

    await waitFor(() => expect(screen.getByTestId("session-continue-submit")).toBeEnabled());
    await user.click(screen.getByTestId("session-continue-submit"));
    const refusal = await screen.findByTestId("session-continue-submit-error");
    expect(refusal).toHaveTextContent(
      'Provider configuration is unavailable: acp: model "gone" is unavailable'
    );
    expect(refusal).not.toHaveTextContent("stderr");
    expect(refusal).not.toHaveTextContent("admit first message");
    expect(screen.getByTestId("session-continue-open-committed-child")).toBeEnabled();
  });

  // Invariant: once a refusal named the committed child, that child stays reachable for the
  // rest of the dialog: editing the form (which clears the refusal) and resubmitting the edited
  // request under the same key (an idempotency conflict that names no child) never hide it.
  it("Should keep the committed session reachable through an edit and a conflicting retry", async () => {
    const user = userEvent.setup();
    const child = continuedSessionFixture(source);
    const sent: ContinueSessionRequest[] = [];
    const { openInNewWindow } = renderDialog({
      onContinue: request => sent.push(request),
      result: [
        {
          status: 422,
          error: 'acp: model "gone" is unavailable',
          code: "model_unavailable",
          child_session_id: child.id,
        },
        {
          status: 409,
          error: "idempotency key was used with a different request",
          code: "idempotency_conflict",
        },
      ],
      committedChild: child,
    });

    await waitFor(() => expect(screen.getByTestId("session-continue-submit")).toBeEnabled());
    await user.click(screen.getByTestId("session-continue-submit"));
    expect(await screen.findByTestId("session-continue-submit-error")).toHaveTextContent(
      'acp: model "gone" is unavailable'
    );

    await user.click(screen.getByTestId("session-continue-agent-select"));
    await user.click(await screen.findByRole("option", { name: /claude/ }));
    expect(screen.queryByTestId("session-continue-submit-error")).not.toBeInTheDocument();
    expect(screen.getByTestId("session-continue-committed-child")).toHaveTextContent(
      "The new session was already created."
    );

    await user.click(screen.getByTestId("session-continue-submit"));
    expect(await screen.findByTestId("session-continue-submit-error")).toHaveTextContent(
      "idempotency key was used with a different request"
    );
    expect(sent).toHaveLength(2);
    expect(sent[1]).toMatchObject({
      agent_name: "claude",
      idempotency_key: sent[0]?.idempotency_key,
    });
    await user.click(screen.getByTestId("session-continue-open-committed-child"));

    await waitFor(() => expect(openInNewWindow).toHaveBeenCalledTimes(1));
    expect(openInNewWindow.mock.calls[0]?.[0].id).toBe(child.id);
  });

  // Invariant: the body scrolls inside the session window, so a refusal (and the committed
  // session it names) is brought into view instead of landing below the fold.
  it("Should bring a post-commit refusal into view when it appears", async () => {
    const user = userEvent.setup();
    // jsdom has no layout, so it implements no scrollIntoView.
    const scrollIntoView = vi.fn();
    Object.defineProperty(HTMLElement.prototype, "scrollIntoView", {
      configurable: true,
      value: scrollIntoView,
    });
    onTestFinished(() => {
      Reflect.deleteProperty(HTMLElement.prototype, "scrollIntoView");
    });
    const child = continuedSessionFixture(source);
    renderDialog({
      result: { status: 422, error: "provider not authenticated", child_session_id: child.id },
      committedChild: child,
    });

    await waitFor(() => expect(screen.getByTestId("session-continue-submit")).toBeEnabled());
    await user.click(screen.getByTestId("session-continue-submit"));
    const outcome = await screen.findByTestId("session-continue-submit-outcome");

    expect(outcome).toContainElement(screen.getByTestId("session-continue-open-committed-child"));
    expect(scrollIntoView.mock.contexts).toContain(outcome);
    expect(scrollIntoView).toHaveBeenCalledWith({ block: "nearest" });
  });

  it("Should offer no committed session for a refusal before the commit", async () => {
    const user = userEvent.setup();
    renderDialog({ result: { status: 409, error: "route_not_found", code: "route_not_found" } });

    await waitFor(() => expect(screen.getByTestId("session-continue-submit")).toBeEnabled());
    await user.click(screen.getByTestId("session-continue-submit"));
    await screen.findByTestId("session-continue-submit-error");
    expect(screen.queryByTestId("session-continue-committed-child")).not.toBeInTheDocument();
  });

  it("Should show the daemon's refusal verbatim", async () => {
    const user = userEvent.setup();
    const { openInNewWindow } = renderDialog({
      result: { status: 404, error: 'no agent named "codex"', code: "agent_not_found" },
    });

    await waitFor(() => expect(screen.getByTestId("session-continue-submit")).toBeEnabled());
    await user.click(screen.getByTestId("session-continue-submit"));

    expect(await screen.findByTestId("session-continue-submit-error")).toHaveTextContent(
      'no agent named "codex"'
    );
    expect(openInNewWindow).not.toHaveBeenCalled();
  });
});
