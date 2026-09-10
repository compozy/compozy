import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { SessionRuntimeRenderProvider } from "../../lib/session-runtime-render-context";
import type { UIMessage } from "../../types";
import { SessionToolCallRow } from "../tool-call-card";

const ARTIFACT_ID_A = `art_${"a".repeat(64)}`;
const ARTIFACT_ID_B = `art_${"b".repeat(64)}`;
const ARTIFACT_URI_A = `compozy://tool-artifacts/${ARTIFACT_ID_A}`;
const ARTIFACT_URI_B = `compozy://tool-artifacts/${ARTIFACT_ID_B}`;

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
    toolName: "compozy__memory_recall",
    toolResult: {
      preview,
      truncated: true,
      artifacts: [
        {
          uri,
          name: "tool-result.json",
          mime_type: "application/vnd.compozy.tool-result+json",
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

describe("Session SessionToolCallRow — wraps <SessionToolCallRow> from @compozy/ui", () => {
  // This suite owns terminal-vs-tool rendering. Canvas drawing is browser I/O;
  // model an unavailable drawing context while retaining the real terminal block.
  beforeEach(() => {
    vi.spyOn(HTMLCanvasElement.prototype, "getContext").mockReturnValue(null);
  });

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
    const longCommand =
      "compozy tool invoke compozy__tool_info --input " + '{"tool_id":"compozy__skill_view"}';
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
    expect(preview?.textContent).toContain("compozy tool invoke");
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
    expect(indicator).not.toHaveClass("text-success");
    expect(indicator).not.toHaveClass("text-danger");
    expect(screen.getByRole("status", { name: "Running" })).toBe(indicator);
    expect(queryToolName()).toHaveTextContent("Reading...");
  });

  it("Should read a resultless tool as an absorbed failure once the owning turn settles", () => {
    render(<SessionToolCallRow message={makeToolMessage()} turnSettled />);

    // No result after the turn settled is a failure the turn moved past, not
    // the failure that ended it (ADR-009): subtle ×, never danger.
    expect(queryRoot()).toHaveAttribute("data-status", "absorbed");
    expect(queryStatusIndicator()).toHaveAttribute("aria-label", "Failed");
    expect(queryToolName()).toHaveTextContent("Read file");
    expect(queryToolName()).not.toHaveTextContent("Reading...");
    expect(queryPreview()).toHaveTextContent("Tool call failed");
  });

  it("Should map meaningful output to the success row state (grey check)", () => {
    render(<SessionToolCallRow message={makeToolMessage({ toolResult: { content: "file" } })} />);
    expect(queryRoot()?.getAttribute("data-status")).toBe("success");
    const indicator = queryStatusIndicator();
    expect(indicator?.getAttribute("data-status")).toBe("success");
    expect(indicator?.getAttribute("aria-label")).toBe("Done");
    expect(indicator).not.toHaveClass("text-success");
    expect(indicator).not.toHaveClass("text-danger");
    expect(screen.getByRole("img", { name: "Done" })).toBe(indicator);
    expect(queryToolName()).toHaveTextContent("Read file");
  });

  it("Should expose a successful file diff through the expandable trigger description", () => {
    render(
      <SessionToolCallRow
        message={makeToolMessage({
          toolName: "Edit",
          toolInput: {
            file_path: "/src/main.ts",
            old_string: "before",
            new_string: "after",
          },
          toolResult: { content: "updated" },
        })}
      />
    );

    expect(document.querySelector('[data-slot="tool-call-row-stat"]')).toHaveTextContent("+1−1");
    expect(screen.getByRole("button", { name: /1 addition, 1 deletion/ })).toBeInTheDocument();
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

  it("Should map a runtime error to failed with a neutral heading and the error-first-line preview", async () => {
    const user = userEvent.setup();
    // Danger only when the failure ended the turn (ADR-009).
    render(
      <SessionToolCallRow
        message={makeToolMessage({ toolResult: { error: "not found" }, toolError: true })}
        turnFailed
      />
    );
    expect(queryRoot()?.getAttribute("data-status")).toBe("failed");
    const indicator = queryStatusIndicator();
    expect(indicator?.getAttribute("data-status")).toBe("failed");
    expect(indicator?.getAttribute("aria-label")).toBe("Error");
    expect(indicator?.getAttribute("class")).toContain("text-danger");
    expect(screen.getByRole("img", { name: "Error" })).toBe(indicator);
    // The verb keeps its tense and the row text never turns danger — the ×
    // glyph plus the error-first-line preview carry the failure.
    expect(queryToolName()).toHaveTextContent("Read file");
    expect(queryToolName()?.className).not.toContain("text-danger");
    expect(document.querySelector('[data-slot="tool-call-row-preview"]')).toHaveTextContent(
      "not found"
    );
    // Failed rows stay collapsed; expanding reveals the error detail.
    expect(document.querySelector('[data-slot="tool-call-row-error"]')).toBeNull();
    await user.click(document.querySelector('[data-slot="tool-call-row-trigger"]') as HTMLElement);
    expect(document.querySelector('[data-slot="tool-call-row-error"]')).toHaveTextContent(
      "not found"
    );
  });

  // ADR-009 / US-026.AC-3: a failure the turn absorbed is information — subtle ×
  // plus the word "failed" — never the danger glyph.
  it("Should read an absorbed failure as a subtle × with the word failed, not danger", () => {
    render(
      <SessionToolCallRow
        message={makeToolMessage({
          toolName: "Bash",
          toolInput: { command: "deploy" },
          toolResult: { stderr: "bash: deploy: command not found" },
        })}
      />
    );
    expect(queryRoot()?.getAttribute("data-status")).toBe("absorbed");
    const indicator = queryStatusIndicator();
    expect(indicator?.getAttribute("aria-label")).toBe("Failed");
    expect(indicator?.getAttribute("class")).toContain("text-subtle");
    expect(indicator?.getAttribute("class")).not.toContain("text-danger");
    expect(screen.getByTestId("tool-call-state-word")).toHaveTextContent("failed");
    const headingEl = queryToolName();
    expect(headingEl).toHaveTextContent("Ran command");
    expect(headingEl?.className).not.toContain("text-danger");
    expect(headingEl?.className).toContain("text-muted");
    expect(queryPreview()).toHaveTextContent("bash: deploy: command not found");
  });

  it("Should render a non-empty generic failure preview when the runtime supplies no error body", async () => {
    const user = userEvent.setup();
    render(<SessionToolCallRow message={makeToolMessage({ toolError: true })} />);

    expect(queryRoot()).toHaveAttribute("data-status", "absorbed");
    expect(queryPreview()).toHaveTextContent("Tool call failed");
    await user.click(document.querySelector('[data-slot="tool-call-row-trigger"]') as HTMLElement);
    expect(document.querySelector('[data-slot="tool-call-row-error"]')).toHaveTextContent(
      "Tool call failed"
    );
  });

  it("Should use stderr as the failure preview for a non-Bash tool", () => {
    render(
      <SessionToolCallRow
        message={makeToolMessage({
          toolName: "Read",
          toolError: true,
          toolResult: { stderr: "read denied\nworkspace is read-only" },
        })}
      />
    );

    expect(queryPreview()).toHaveTextContent("read denied workspace is read-only");
  });

  // ADR-009: the call that was running when the operator stopped the turn reads
  // "stopped" in settled ink, with no glyph at all.
  it("Should read a call cut by the operator's stop as stopped with no glyph", () => {
    render(
      <SessionToolCallRow
        message={makeToolMessage({ toolName: "Bash", toolInput: { command: "go test" } })}
        interrupted
        turnSettled
      />
    );
    expect(queryRoot()).toHaveAttribute("data-status", "stopped");
    expect(queryStatusIndicator()).toBeNull();
    expect(screen.getByTestId("tool-call-state-word")).toHaveTextContent("stopped");
    expect(queryToolName()).toHaveTextContent("Ran command");
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

  it("Should render a supervised terminal as its own block, not a generic tool row", async () => {
    render(
      artifactRow(
        "ws-terminal",
        makeToolMessage({
          toolName: "compozy__terminal_exec",
          toolInput: { command: "bun run dev" },
          toolResult: {
            rawOutput: {
              terminal_id: "term-4f21c9a03b7e",
              output: "VITE ready in 412 ms",
              still_running: true,
            },
          },
        })
      )
    );

    expect(await screen.findByTestId("terminal-content")).toBeInTheDocument();
    expect(queryRoot()).toBeNull();
    expect(screen.queryByRole("button", { name: "Copy tool payload" })).not.toBeInTheDocument();
    expect(screen.queryByText("Output")).not.toBeInTheDocument();
  });

  it("Should intercept a hosted-MCP terminal open carrying a string envelope", async () => {
    render(
      artifactRow(
        "ws-terminal",
        makeToolMessage({
          toolName: "mcp__compozy-hosted-tools__compozy__terminal_open",
          toolInput: {},
          toolResult: {
            rawOutput: {
              content: '{"terminal_id":"term-51e7b88e4515"}',
              raw_output: '{"terminal_id":"term-51e7b88e4515"}',
            },
          },
        })
      )
    );

    expect(await screen.findByTestId("terminal-content")).toBeInTheDocument();
    expect(queryRoot()).toBeNull();
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
      // Pages append into the same plain mono pre — content loads in place.
      expect(screen.getByTestId("full-tool-result").textContent).toBe(fullResult);
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

    // Load failure is a danger text line — never an Alert card — and a gone
    // artifact (404) offers no Retry.
    expect(await screen.findByTestId("artifact-error")).toHaveTextContent(
      "This retained result is no longer available"
    );
    expect(screen.getByText("bounded preview")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Retry" })).not.toBeInTheDocument();
  });

  // Invariant (BUG-20260906-find-specialized-tool-field): a find reveal names the field the
  // daemon matched; the row opens and renders that field's whole raw payload beside the
  // tool's own display, so the searched text is on screen even when the specialized
  // renderer never shows it or the bounded preview would have cut it. Without a reveal the
  // specialized display is unchanged. Owner: SessionToolCallRow. Canonical suite: this file.
  it("Should render the matched raw input beside a specialized Read display on a find reveal", () => {
    const message = makeToolMessage({
      toolInput: { file_path: "notes/ação-café.md", query: "ação λ café" },
      toolResult: { stdout: "# café\n" },
    });
    const revealed = render(
      <SessionToolCallRow message={message} revealOpen revealField="input" />
    );
    expect(screen.getByTestId("read-content")).toHaveTextContent("notes/ação-café.md");
    const matched = screen.getByTestId("tool-matched-field");
    expect(matched).toHaveAttribute("data-field", "input");
    expect(matched).toHaveTextContent("Input");
    expect(matched).toHaveTextContent('"query": "ação λ café"');
    expect(queryRoot()).toHaveAttribute("data-expanded", "true");

    revealed.unmount();

    // Outside navigation the specialized display stands alone.
    render(<SessionToolCallRow message={message} defaultExpanded />);
    expect(screen.getByTestId("read-content")).toBeInTheDocument();
    expect(screen.queryByTestId("tool-matched-field")).not.toBeInTheDocument();
  });

  it("Should reveal a matched output past the bounded preview in full, keeping the strip", () => {
    const lines = Array.from({ length: 260 }, (_, index) =>
      index === 250 ? "needle λ line" : `line ${index + 1}`
    );
    const message = makeToolMessage({
      toolName: "Bash",
      toolInput: { command: "cat notes.txt" },
      toolResult: { stdout: lines.join("\n") },
    });
    render(<SessionToolCallRow message={message} revealOpen revealField="output" />);
    const matched = screen.getByTestId("tool-matched-field");
    expect(matched).toHaveAttribute("data-field", "output");
    expect(matched).toHaveTextContent("needle λ line");
    // The matched payload starts whole; the tool's own bounded display keeps its preview.
    expect(within(matched).getByTestId("detail-payload-note")).toHaveTextContent("All 260 lines");
    expect(within(matched).getByTestId("detail-payload-show-all")).toHaveTextContent("Show less");
    // A header field needs no body payload: title/tool name/file already show.
    render(<SessionToolCallRow message={message} revealOpen revealField="tool_name" />);
    expect(screen.getAllByTestId("tool-matched-field")).toHaveLength(1);
  });
});

// Invariant: title summaries leave the exact title, input, and output available for keyboard disclosure/copy/find.
// Owner: session tool card; canonical suite: tool-call-card.
it("Should disclose a long provider title and copy its exact original payload", async () => {
  const user = userEvent.setup();
  const title = "Inspect fixture\n" + "ação 👩🏽‍💻 ".repeat(80) + "title-tail";
  const message = makeToolMessage({
    toolName: "Bash",
    toolTitle: title,
    toolInput: { command: "printf 'input-tail'" },
    toolResult: { stdout: "output-tail" },
  });
  render(<SessionToolCallRow message={message} turnSettled />);
  expect(queryToolName()).toHaveTextContent("Ran command");
  expect(queryPreview()).not.toHaveTextContent("title-tail");
  const trigger = screen.getByRole("button", { name: /Ran command.*Toggle tool call/ });
  trigger.focus();
  await user.keyboard("{Enter}");
  expect(screen.getByLabelText("Tool title").textContent).toBe(title);
  const writeText = vi.spyOn(navigator.clipboard, "writeText");
  await user.click(screen.getByRole("button", { name: "Copy tool payload" }));
  await waitFor(() => expect(writeText).toHaveBeenCalled());
  expect(JSON.parse(writeText.mock.calls[0]![0])).toMatchObject({
    tool: "Bash",
    title,
    input: message.toolInput,
    output: message.toolResult,
  });
});

it("Should show a title-only unknown tool in full when find reveals its title", () => {
  const title = "provider description ".repeat(80) + "title-only-tail";
  render(
    <SessionToolCallRow
      message={makeToolMessage({ toolName: title, toolInput: undefined })}
      turnSettled
      revealOpen
      revealField="title"
    />
  );
  expect(screen.getByLabelText("Tool title").textContent).toBe(title);
  expect(queryToolName()).toHaveTextContent("Used tool");
});
