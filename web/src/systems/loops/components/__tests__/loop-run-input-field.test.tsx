import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { LoopInputCatalogBoundary } from "../input/loop-input-catalogs";

import { LoopRunInputField } from "../run-form/loop-run-input-field";
import type { LoopInputSchemaField } from "../../types";

function field(overrides: Partial<LoopInputSchemaField> = {}): LoopInputSchemaField {
  return { type: "string", ...overrides };
}

describe("LoopRunInputField", () => {
  it("Should render a mono text control for string/file/ref types", () => {
    for (const type of ["string", "file", "ref"] as const) {
      const { unmount } = render(
        <LoopRunInputField name="x" field={field({ type })} value="" onChange={vi.fn()} />
      );
      expect(screen.getByTestId("loop-run-field-x")).toHaveAttribute("data-input-type", type);
      expect(screen.getByTestId("loop-run-field-input-x")).toHaveAttribute("type", "text");
      unmount();
    }
  });

  it("Should render a numeric control that emits numbers and clears to undefined", () => {
    const onChange = vi.fn();
    render(
      <LoopRunInputField
        name="max_files"
        field={field({ type: "number" })}
        value={5}
        onChange={onChange}
      />
    );
    const input = screen.getByTestId("loop-run-field-input-max_files");
    expect(input).toHaveAttribute("type", "number");
    fireEvent.change(input, { target: { value: "12" } });
    expect(onChange).toHaveBeenCalledWith(12);
    fireEvent.change(input, { target: { value: "" } });
    expect(onChange).toHaveBeenCalledWith(undefined);
  });

  it("Should render a switch for boolean types and emit booleans", () => {
    const onChange = vi.fn();
    render(
      <LoopRunInputField
        name="auto_commit"
        field={field({ type: "boolean", default: false })}
        value={false}
        onChange={onChange}
      />
    );
    fireEvent.click(screen.getByTestId("loop-run-switch-auto_commit"));
    expect(onChange).toHaveBeenCalledWith(true);
    expect(screen.getByTestId("loop-run-field-auto_commit")).toHaveAttribute(
      "data-input-type",
      "boolean"
    );
  });

  it("Should render an agent picker and keep an unresolved value visible", () => {
    render(
      <LoopRunInputField
        name="implementer"
        field={field({ type: "agent", default: "code_implementer" })}
        value="code_implementer"
        onChange={vi.fn()}
      />
    );
    const wrapper = screen.getByTestId("loop-run-field-implementer");
    expect(wrapper).toHaveAttribute("data-input-type", "agent");
    expect(screen.getByTestId("loop-run-field-input-implementer").tagName).toBe("BUTTON");
    expect(wrapper).toHaveTextContent("code_implementer");
    expect(wrapper).toHaveTextContent("Not available");
  });

  it("Should render enum and runtime declarations as closed typed controls", () => {
    const enumRender = render(
      <LoopRunInputField
        name="environment"
        field={field({ type: "string", enum: ["dev", "staging", "prod"] })}
        value="staging"
        onChange={vi.fn()}
      />
    );
    expect(screen.getByTestId("loop-run-field-input-environment").tagName).toBe("BUTTON");
    expect(screen.getByTestId("loop-run-field-environment")).toHaveTextContent("staging");
    enumRender.unmount();

    const onRuntimeChange = vi.fn();
    // Invariant: typed runtime fields expose their caption and selected value.
    // Owner: Loop input composition; canonical suite: LoopRunInputField.
    render(
      <>
        <LoopRunInputField
          name="backend_runtime"
          field={field({ type: "runtime" })}
          value={{ provider: "codex", model: "gpt-5.6", speed: "normal" }}
          onChange={onRuntimeChange}
        />
        <LoopRunInputField
          name="default_runtime"
          field={field({ type: "runtime" })}
          value={{ provider: "codex", model: "gpt-5.6", speed: "normal" }}
          onChange={vi.fn()}
        />
      </>
    );
    expect(screen.getByRole("button", { name: /Default runtime.*gpt-5\.6/ })).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /Backend runtime.*gpt-5\.6/ }));
    fireEvent.click(screen.getByTestId("runtime-selector-speed"));
    expect(onRuntimeChange).toHaveBeenCalledWith({
      provider: "codex",
      model: "gpt-5.6",
      speed: "fast",
    });
  });

  it("Should label an input in plain words and keep the key and type off screen", () => {
    render(
      <LoopRunInputField
        name="auto_commit"
        field={field({ type: "boolean" })}
        value={false}
        onChange={vi.fn()}
      />
    );
    const wrapper = screen.getByTestId("loop-run-field-auto_commit");
    expect(wrapper).toHaveAttribute("data-input-type", "boolean");
    expect(wrapper).toHaveTextContent("Auto commit");
    expect(wrapper).not.toHaveTextContent("boolean");
    expect(screen.getByTitle("auto_commit")).toBeInTheDocument();
  });

  it("Should surface the required marker and inline error", () => {
    render(
      <LoopRunInputField
        name="slug"
        field={field({ type: "string", required: true })}
        value=""
        error="Slug is required to run this Loop."
        onChange={vi.fn()}
      />
    );
    expect(screen.getByText("required")).toBeInTheDocument();
    expect(screen.getByTestId("loop-run-field-error-slug")).toHaveTextContent("Slug is required");
  });

  it("Should mark a required boolean with the same required affix", () => {
    render(
      <LoopRunInputField
        name="auto_commit"
        field={field({ type: "boolean", required: true })}
        value={false}
        onChange={vi.fn()}
      />
    );
    expect(screen.getByText("required")).toBeInTheDocument();
  });
});

afterEach(() => vi.unstubAllGlobals());

describe("Loop session reference paging", () => {
  it("Should page older sessions and retain an off-page selected label with scoped detail", async () => {
    const requests: URL[] = [];
    const first = {
      id: "sess-new",
      name: "Recent session",
      agent_name: "coder",
      workspace_id: "ws-a",
      state: "stopped",
    };
    const older = {
      id: "sess-old",
      name: "Older reviewed session",
      agent_name: "reviewer",
      workspace_id: "ws-a",
      state: "stopped",
    };
    vi.stubGlobal(
      "fetch",
      vi.fn(async (request: Request) => {
        const url = new URL(request.url);
        requests.push(url);
        let body: unknown = {};
        if (url.pathname === "/api/sessions") {
          const olderPage = url.searchParams.get("cursor") === "older";
          const search = url.searchParams.get("q") === "reviewer";
          body = {
            sessions: olderPage || search ? [older] : [first],
            page: {
              has_more: !olderPage && !search,
              next_cursor: !olderPage && !search ? "older" : null,
              limit: 100,
            },
          };
        } else if (url.pathname === "/api/sessions/sess-old") {
          body = { session: older };
        } else if (url.pathname === "/api/profiles") {
          body = [];
        } else {
          body = { profile: "default" };
        }
        return new Response(JSON.stringify(body), {
          status: 200,
          headers: { "Content-Type": "application/json" },
        });
      })
    );
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const onChange = vi.fn();
    const view = (value: string) => (
      <QueryClientProvider client={client}>
        <LoopInputCatalogBoundary
          workspaceId="ws-a"
          needs={{ entities: new Set(["session"]), runtime: false }}
        >
          <LoopRunInputField
            name="session"
            field={field({ type: "ref", ref: { kind: "session" } })}
            value={value}
            onChange={onChange}
          />
        </LoopInputCatalogBoundary>
      </QueryClientProvider>
    );
    const rendered = render(view(""));
    await waitFor(() =>
      expect(requests.filter(url => url.pathname === "/api/sessions")).toHaveLength(1)
    );
    fireEvent.click(screen.getByTestId("loop-run-field-input-session"));
    await screen.findByRole("option", { name: /Recent session/ });
    expect(requests.some(url => url.searchParams.has("cursor"))).toBe(false);
    fireEvent.click(screen.getByRole("button", { name: "Next" }));
    fireEvent.click(await screen.findByRole("option", { name: /Older reviewed session/ }));
    expect(onChange).toHaveBeenCalledWith("sess-old");
    rendered.rerender(view("sess-old"));
    fireEvent.click(screen.getByTestId("loop-run-field-input-session"));
    fireEvent.click(screen.getByRole("button", { name: "Previous" }));
    await screen.findByRole("option", { name: /Recent session/ });
    await waitFor(() =>
      expect(screen.getByTestId("loop-run-field-input-session")).toHaveTextContent(
        "Older reviewed session"
      )
    );
    expect(screen.queryByText("Not available")).not.toBeInTheDocument();
    expect(
      requests.find(url => url.pathname === "/api/sessions/sess-old")?.searchParams.get("profile")
    ).toBe("default");
    // Remote search coalesces a typing burst while the input remains immediate.
    const searchInput = screen.getByRole("combobox", { name: "Search sessions" });
    fireEvent.change(searchInput, { target: { value: "rev" } });
    fireEvent.change(searchInput, { target: { value: "reviewer" } });
    expect(searchInput).toHaveValue("reviewer");
    const previousRow = screen.getByRole("option", { name: /Recent session/ });
    expect(previousRow).toHaveAttribute("aria-disabled", "true");
    const selectedBeforeSearch = onChange.mock.calls.length;
    fireEvent.click(previousRow);
    expect(onChange).toHaveBeenCalledTimes(selectedBeforeSearch);
    expect(
      requests.some(url => url.pathname === "/api/sessions" && url.searchParams.has("q"))
    ).toBe(false);
    await screen.findByRole("option", { name: /Older reviewed session/ });
    expect(
      requests.some(
        url =>
          url.pathname === "/api/sessions" &&
          url.searchParams.get("q") === "reviewer" &&
          url.searchParams.get("search_fields") === "title_agent" &&
          !url.searchParams.has("cursor")
      )
    ).toBe(true);
    expect(requests.some(url => url.searchParams.get("q") === "rev")).toBe(false);
    expect(requests.some(url => url.pathname === "/api/sessions/facets")).toBe(false);
    expect(
      requests
        .filter(url => url.pathname === "/api/sessions")
        .every(
          url =>
            url.searchParams.get("workspace_id") === "ws-a" &&
            url.searchParams.get("skip_total") === "true"
        )
    ).toBe(true);
    client.clear();
  });
});
