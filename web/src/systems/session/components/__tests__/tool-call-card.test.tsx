import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, describe, expect, it, vi } from "vitest";

import { SessionRuntimeRenderProvider } from "../../lib/session-runtime-render-context";
import type { UIMessage } from "../../types";
import { SessionToolCallRow } from "../tool-call-card";

const ARTIFACT_ID_A = `art_${"a".repeat(64)}`;
const ARTIFACT_ID_B = `art_${"b".repeat(64)}`;
const ARTIFACT_URI_A = `agh://tool-artifacts/${ARTIFACT_ID_A}`;
const ARTIFACT_URI_B = `agh://tool-artifacts/${ARTIFACT_ID_B}`;

function createQueryClient() {
  return new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
}

function encodedBytes(bytes: Uint8Array): string {
  let binary = "";
  for (const byte of bytes) binary += String.fromCharCode(byte);
  return globalThis.btoa(binary);
}

function requestURL(input: RequestInfo | URL): URL {
  if (input instanceof Request) return new URL(input.url);
  return new URL(String(input), "http://localhost");
}

function truncatedMessage(uri = ARTIFACT_URI_A, preview = "bounded preview"): UIMessage {
  return makeToolMessage({
    toolName: "agh__memory_recall",
    toolResult: {
      preview,
      truncated: true,
      artifacts: [
        {
          uri,
          name: "tool-result.json",
          mime_type: "application/vnd.agh.tool-result+json",
          bytes: 256,
          sha256: uri.slice(-64),
        },
      ],
    },
  });
}

function artifactRow(workspaceId: string, message: UIMessage, queryClient = createQueryClient()) {
  return (
    <QueryClientProvider client={queryClient}>
      <SessionRuntimeRenderProvider sessionId="session-artifact-test" workspaceId={workspaceId}>
        <SessionToolCallRow message={message} defaultExpanded />
      </SessionRuntimeRenderProvider>
    </QueryClientProvider>
  );
}

function makeToolMessage(overrides: Partial<UIMessage> = {}): UIMessage {
  return {
    id: "tc-1",
    role: "tool_call",
    content: "",
    toolName: "Read",
    toolInput: { file_path: "/src/main.ts" },
    timestamp: Date.now(),
    ...overrides,
  };
}

function queryRoot(): HTMLElement | null {
  return document.querySelector<HTMLElement>('[data-slot="tool-call-row"]');
}

function queryStatusIndicator(): HTMLElement | null {
  return document.querySelector<HTMLElement>('[data-slot="tool-call-row-status"]');
}

function queryToolName(): HTMLElement | null {
  return document.querySelector<HTMLElement>('[data-slot="tool-call-row-tool"]');
}

function queryPreview(): HTMLElement | null {
  return document.querySelector<HTMLElement>('[data-slot="tool-call-row-preview"]');
}

function queryIcon(): HTMLElement | null {
  return document.querySelector<HTMLElement>('[data-slot="tool-call-row-icon"]');
}

function queryBody(): HTMLElement | null {
  return document.querySelector<HTMLElement>('[data-slot="tool-call-row-body"]');
}

describe("Session SessionToolCallRow — wraps <SessionToolCallRow> from @agh/ui", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("Should surface the tense-aware verb (not the raw tool name) in the row heading slot", () => {
    render(<SessionToolCallRow message={makeToolMessage()} />);
    // Read fixture is in-flight (no result) → active verb.
    expect(queryToolName()).toHaveTextContent("Reading...");
    expect(queryToolName()).not.toHaveTextContent("Read file");
  });

  it("Should render the mapped per-tool icon and never the terminal fallback for a known tool", () => {
    render(<SessionToolCallRow message={makeToolMessage({ toolResult: { content: "file" } })} />);
    const iconClass = queryIcon()?.getAttribute("class") ?? "";
    expect(iconClass).toContain("lucide-file-text");
    expect(iconClass).not.toContain("lucide-terminal");
  });

  it("Should show the compact input summary in the row preview slot", () => {
    render(<SessionToolCallRow message={makeToolMessage()} />);
    expect(queryPreview()).toHaveTextContent("/src/main.ts");
  });

  it("Should map Bash command summaries to the row preview slot", () => {
    const longCommand = "agh tool invoke agh__tool_info --input " + '{"tool_id":"agh__skill_view"}';
    render(
      <SessionToolCallRow
        message={makeToolMessage({
          toolName: "Bash",
          toolInput: { command: longCommand },
        })}
      />
    );
    expect(queryToolName()).toHaveTextContent("Running...");
    const preview = queryPreview();
    expect(preview).not.toBeNull();
    expect(preview?.textContent).toContain("agh tool invoke");
    expect(preview?.className).toContain("truncate");
  });

  it("Should render the pending row state (muted, no glyph) for an in-flight tool with empty input", () => {
    render(<SessionToolCallRow message={makeToolMessage({ toolInput: {} })} />);
    expect(queryRoot()?.getAttribute("data-status")).toBe("pending");
    expect(queryStatusIndicator()).toBeNull();
  });

  it("Should map an in-flight tool with input to the running row state with a Spinner", () => {
    render(<SessionToolCallRow message={makeToolMessage()} />);
    expect(queryRoot()?.getAttribute("data-status")).toBe("running");
    const indicator = queryStatusIndicator();
    expect(indicator?.getAttribute("data-status")).toBe("running");
    expect(indicator?.getAttribute("aria-label")).toBe("Running");
    expect(indicator?.getAttribute("class")).toContain("text-muted");
    expect(screen.getByRole("status", { name: "Running" })).toBe(indicator);
    expect(queryToolName()).toHaveTextContent("Reading...");
  });

  it("Should map meaningful output to the success row state (Check, success tone)", () => {
    render(<SessionToolCallRow message={makeToolMessage({ toolResult: { content: "file" } })} />);
    expect(queryRoot()?.getAttribute("data-status")).toBe("success");
    const indicator = queryStatusIndicator();
    expect(indicator?.getAttribute("data-status")).toBe("success");
    expect(indicator?.getAttribute("aria-label")).toBe("Done");
    expect(indicator?.getAttribute("class")).toContain("text-success");
    expect(screen.getByRole("img", { name: "Done" })).toBe(indicator);
    expect(queryToolName()).toHaveTextContent("Read file");
  });

  it("Should render the empty row state (Minus, faint tone) for empty output mid-stream", () => {
    render(
      <SessionToolCallRow message={makeToolMessage({ toolResult: {} })} turnSettled={false} />
    );
    expect(queryRoot()?.getAttribute("data-status")).toBe("empty");
    const indicator = queryStatusIndicator();
    expect(indicator?.getAttribute("data-status")).toBe("empty");
    expect(indicator?.getAttribute("aria-label")).toBe("Empty");
    expect(indicator?.getAttribute("class")).toContain("text-subtle");
  });

  it("Should promote a neutral tool to success only once the turn settles, never before", () => {
    const message = makeToolMessage({ toolResult: {} });
    const { rerender } = render(<SessionToolCallRow message={message} turnSettled={false} />);
    expect(queryRoot()?.getAttribute("data-status")).toBe("empty");

    rerender(<SessionToolCallRow message={message} turnSettled />);
    expect(queryRoot()?.getAttribute("data-status")).toBe("success");
    expect(queryStatusIndicator()?.getAttribute("aria-label")).toBe("Done");
  });

  it("Should map a runtime error to failed, a danger heading, and the real error detail (not the verb)", () => {
    render(
      <SessionToolCallRow
        message={makeToolMessage({ toolResult: { error: "not found" }, toolError: true })}
      />
    );
    expect(queryRoot()?.getAttribute("data-status")).toBe("failed");
    const indicator = queryStatusIndicator();
    expect(indicator?.getAttribute("data-status")).toBe("failed");
    expect(indicator?.getAttribute("aria-label")).toBe("Error");
    expect(indicator?.getAttribute("class")).toContain("text-danger");
    expect(screen.getByRole("img", { name: "Error" })).toBe(indicator);
    expect(queryToolName()).toHaveTextContent("Failed to read file");
    expect(queryToolName()?.className).toContain("text-danger");
    expect(document.querySelector('[data-slot="tool-call-row-error"]')).toHaveTextContent(
      "not found"
    );
  });

  it("Should flag error-shaped output as failed while keeping the heading neutral (not a runtime error)", () => {
    render(
      <SessionToolCallRow
        message={makeToolMessage({
          toolName: "Bash",
          toolInput: { command: "deploy" },
          toolResult: { stderr: "bash: deploy: command not found" },
        })}
      />
    );
    expect(queryRoot()?.getAttribute("data-status")).toBe("failed");
    expect(queryStatusIndicator()?.getAttribute("aria-label")).toBe("Error");
    const headingEl = queryToolName();
    expect(headingEl).toHaveTextContent("Ran command");
    expect(headingEl?.className).not.toContain("text-danger");
    expect(headingEl?.className).toContain("text-muted");
  });

  it("Should toggle the specialized output body by click and keyboard", async () => {
    const user = userEvent.setup();
    render(<SessionToolCallRow message={makeToolMessage()} />);
    const rowTrigger = document.querySelector<HTMLElement>('[data-slot="tool-call-row-trigger"]');
    expect(rowTrigger).not.toBeNull();
    expect(rowTrigger).toHaveAttribute("aria-expanded", "false");
    expect(queryBody()).toBeNull();

    await user.click(rowTrigger as HTMLElement);
    expect(queryBody()).not.toBeNull();
    expect(document.querySelector('[data-slot="tool-call-row-output"]')).not.toBeNull();
    expect(screen.getByTestId("read-content")).toHaveTextContent("/src/main.ts");

    // SUT_IS_CORRECT_BECAUSE native buttons own Enter/Space activation; userEvent
    // exercises the browser interaction instead of bypassing it with keydown only.
    (rowTrigger as HTMLElement).focus();
    await user.keyboard("{Enter}");
    expect(queryBody()).toBeNull();
    await user.keyboard(" ");
    expect(queryBody()).not.toBeNull();
  });

  it("Should keep the body open when interacting inside it so text selection stays safe", () => {
    render(<SessionToolCallRow message={makeToolMessage({ toolResult: { content: "abc" } })} />);
    const rowTrigger = document.querySelector<HTMLElement>('[data-slot="tool-call-row-trigger"]');
    fireEvent.click(rowTrigger as HTMLElement);
    expect(rowTrigger).toHaveAttribute("aria-expanded", "true");
    expect(queryBody()).not.toBeNull();

    const output = document.querySelector<HTMLElement>('[data-slot="tool-call-row-output"]');
    expect(output).not.toBeNull();
    fireEvent.pointerDown(output as HTMLElement);
    fireEvent.click(output as HTMLElement);

    expect(rowTrigger).toHaveAttribute("aria-expanded", "true");
    expect(queryBody()).not.toBeNull();
  });

  it("Should render the existing Output dispatcher inside the inline body", () => {
    render(
      <SessionToolCallRow
        message={makeToolMessage({
          toolName: "Bash",
          toolInput: { command: "printf abc" },
          toolResult: { stdout: "abc" },
        })}
      />
    );
    const rowTrigger = document.querySelector<HTMLElement>('[data-slot="tool-call-row-trigger"]');
    fireEvent.click(rowTrigger as HTMLElement);

    expect(document.querySelector('[data-slot="tool-call-row-output"]')).not.toBeNull();
    expect(queryBody()).toHaveTextContent("abc");
  });

  it("Should preserve the Edit output renderer inside the inline body", () => {
    render(
      <SessionToolCallRow
        message={makeToolMessage({
          toolName: "Edit",
          toolInput: {
            file_path: "/src/app.ts",
            old_string: "const oldValue = true;",
            new_string: "const oldValue = false;",
          },
          toolResult: { content: "Applied patch successfully." },
        })}
      />
    );

    const rowTrigger = document.querySelector<HTMLElement>('[data-slot="tool-call-row-trigger"]');
    fireEvent.click(rowTrigger as HTMLElement);

    expect(screen.getByTestId("edit-content")).toBeInTheDocument();
    expect(queryBody()).toHaveTextContent("/src/app.ts");
    expect(queryBody()).toHaveTextContent("const oldValue = true;");
    expect(queryBody()).toHaveTextContent("const oldValue = false;");
  });

  it("Should keep MCP and dynamic tools on the generic output renderer", () => {
    render(
      <SessionToolCallRow
        message={makeToolMessage({
          toolName: "mcp__context7__resolve-library-id",
          toolInput: { libraryName: "react" },
          toolResult: { content: "/websites/react_dev" },
        })}
      />
    );

    const rowTrigger = document.querySelector<HTMLElement>('[data-slot="tool-call-row-trigger"]');
    fireEvent.click(rowTrigger as HTMLElement);

    expect(queryToolName()).toHaveTextContent("mcp__context7__resolve-library-id");
    expect(queryBody()).toHaveTextContent('"libraryName": "react"');
    expect(queryBody()).toHaveTextContent("/websites/react_dev");
  });

  it("Should render specialized tool output while the tool is still running", () => {
    render(<SessionToolCallRow message={makeToolMessage()} />);
    const rowTrigger = document.querySelector<HTMLElement>('[data-slot="tool-call-row-trigger"]');
    fireEvent.click(rowTrigger as HTMLElement);

    expect(document.querySelector('[data-slot="tool-call-row-output"]')).not.toBeNull();
    expect(document.querySelector('[data-slot="tool-call-row-input"]')).toBeNull();
    expect(screen.getByTestId("read-content")).toHaveTextContent("/src/main.ts");
  });

  it("Should copy the structured tool payload from the trailing actions cluster", async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: { writeText },
    });
    render(<SessionToolCallRow message={makeToolMessage({ toolResult: { content: "abc" } })} />);

    fireEvent.click(screen.getByRole("button", { name: "Copy tool payload" }));

    await waitFor(() => expect(writeText).toHaveBeenCalledTimes(1));
    const payload = JSON.parse(writeText.mock.calls[0]?.[0] as string) as {
      tool: string;
      input: Record<string, unknown>;
      output: Record<string, unknown>;
    };
    expect(payload.tool).toBe("Read");
    expect(payload.input.file_path).toBe("/src/main.ts");
    expect(payload.output.content).toBe("abc");
  });

  it("Should append each artifact byte page once and decode UTF-8 only after concatenation", async () => {
    const user = userEvent.setup();
    const fullResult = JSON.stringify({ content: [{ type: "text", text: "ação D6 complete" }] });
    const resultBytes = new TextEncoder().encode(fullResult);
    const multibyteStart = resultBytes.findIndex(byte => byte === 0xc3);
    expect(multibyteStart).toBeGreaterThan(0);
    const splitOffset = multibyteStart + 1;
    const chunks = [resultBytes.slice(0, splitOffset), resultBytes.slice(splitOffset)];
    const fetchMock = vi.spyOn(globalThis, "fetch").mockImplementation(async input => {
      const url = requestURL(input);
      const offset = Number(url.searchParams.get("offset") ?? "0");
      const chunk = offset === 0 ? chunks[0] : chunks[1];
      if (!chunk) throw new Error(`Unexpected artifact offset ${offset}`);
      const nextOffset = offset + chunk.byteLength;
      return new Response(
        JSON.stringify({
          artifact: { uri: ARTIFACT_URI_A, bytes: resultBytes.byteLength },
          offset,
          bytes: chunk.byteLength,
          total_bytes: resultBytes.byteLength,
          data_base64: encodedBytes(chunk),
          next_offset: nextOffset,
          eof: nextOffset === resultBytes.byteLength,
        }),
        { headers: { "Content-Type": "application/json" } }
      );
    });

    render(artifactRow("ws-artifact-a", truncatedMessage()));
    await user.click(screen.getByRole("button", { name: "Open full result" }));
    const loadMore = await screen.findByRole("button", { name: "Load more" });
    await user.click(loadMore);

    await waitFor(() => {
      const code = screen
        .getByTestId("full-tool-result")
        .querySelector<HTMLElement>('[data-slot="code-block-code"]');
      expect(code?.textContent).toBe(fullResult);
    });
    expect(screen.queryByRole("button", { name: "Load more" })).not.toBeInTheDocument();
    expect(fetchMock).toHaveBeenCalledTimes(2);
    expect(requestURL(fetchMock.mock.calls[0]![0]).pathname).toBe(
      `/api/workspaces/ws-artifact-a/tool-artifacts/${ARTIFACT_ID_A}`
    );
    expect(requestURL(fetchMock.mock.calls[0]![0]).searchParams.get("offset")).toBe("0");
    expect(requestURL(fetchMock.mock.calls[1]![0]).searchParams.get("offset")).toBe(
      String(splitOffset)
    );
  });

  it("Should isolate retained-result pages by workspace and artifact identity", async () => {
    const user = userEvent.setup();
    const queryClient = createQueryClient();
    const fetchMock = vi.spyOn(globalThis, "fetch").mockImplementation(async input => {
      const url = requestURL(input);
      const workspace = url.pathname.includes("ws-artifact-b") ? "ws-artifact-b" : "ws-artifact-a";
      const uri = workspace === "ws-artifact-b" ? ARTIFACT_URI_B : ARTIFACT_URI_A;
      const bytes = new TextEncoder().encode(JSON.stringify({ workspace }));
      return new Response(
        JSON.stringify({
          artifact: { uri, bytes: bytes.byteLength },
          offset: 0,
          bytes: bytes.byteLength,
          total_bytes: bytes.byteLength,
          data_base64: encodedBytes(bytes),
          next_offset: bytes.byteLength,
          eof: true,
        }),
        { headers: { "Content-Type": "application/json" } }
      );
    });
    const { rerender } = render(
      artifactRow("ws-artifact-a", truncatedMessage(ARTIFACT_URI_A), queryClient)
    );
    await user.click(screen.getByRole("button", { name: "Open full result" }));
    await waitFor(() =>
      expect(screen.getByTestId("full-tool-result")).toHaveTextContent("ws-artifact-a")
    );

    rerender(artifactRow("ws-artifact-b", truncatedMessage(ARTIFACT_URI_B), queryClient));
    await waitFor(() =>
      expect(screen.getByTestId("full-tool-result")).toHaveTextContent("ws-artifact-b")
    );
    expect(screen.getByTestId("full-tool-result")).not.toHaveTextContent("ws-artifact-a");
    expect(fetchMock).toHaveBeenCalledTimes(2);
    expect(requestURL(fetchMock.mock.calls[1]![0]).pathname).toBe(
      `/api/workspaces/ws-artifact-b/tool-artifacts/${ARTIFACT_ID_B}`
    );
  });

  it("Should keep the bounded preview visible when the retained result is unavailable", async () => {
    const user = userEvent.setup();
    vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(JSON.stringify({ error: { code: "tool_not_found", message: "not found" } }), {
        status: 404,
        headers: { "Content-Type": "application/json" },
      })
    );

    render(artifactRow("ws-artifact-a", truncatedMessage()));
    expect(screen.getByText("bounded preview")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Open full result" }));

    expect(await screen.findByText("Full result unavailable")).toBeInTheDocument();
    expect(screen.getByText("This retained result is no longer available")).toBeInTheDocument();
    expect(screen.getByText("bounded preview")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Retry full result" })).not.toBeInTheDocument();
  });
});
