import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render as renderBare, screen, waitFor } from "@testing-library/react";
import type { ReactElement } from "react";
import { describe, expect, it, vi } from "vitest";

import { SessionRuntimeRenderProvider } from "../../../lib/session-runtime-render-context";
import { sessionDetailOptions } from "../../../lib/query-options";
import type { SessionPayload, UIMessage } from "../../../types";

vi.mock("@/lib/utils", () => ({
  cn: (...args: unknown[]) => args.filter(Boolean).join(" "),
}));

/** Captures what the transcript hands the terminal block, without an emulator. */
const blockProps = vi.fn();
vi.mock("@/systems/terminal", () => ({
  SessionTerminalBlock: (props: Record<string, unknown>) => {
    blockProps(props);
    return <div data-testid="session-terminal-block-stub" />;
  },
}));

import { ExpandedToolContent } from "../expanded-tool-content";

const WORKSPACE_ID = "ws-atlas";
const SESSION_ID = "sess-77ab";

/**
 * The transcript's own surroundings.
 *
 * The terminal renderer learns a terminal's scope from the session it is inside
 * — the workspace from the render context, the profile from the session itself
 * — so every renderer here is rendered the way the transcript renders it.
 */
function render(ui: ReactElement, options: { profileName?: string } = {}) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  if (options.profileName) {
    // Only the two fields this renderer reads; the rest of the session payload
    // is irrelevant to which terminal it attaches to.
    client.setQueryData(sessionDetailOptions(WORKSPACE_ID, SESSION_ID).queryKey, {
      id: SESSION_ID,
      profile_name: options.profileName,
    } as unknown as SessionPayload);
  }
  return renderBare(
    <QueryClientProvider client={client}>
      <SessionRuntimeRenderProvider sessionId={SESSION_ID} workspaceId={WORKSPACE_ID}>
        {ui}
      </SessionRuntimeRenderProvider>
    </QueryClientProvider>
  );
}

function makeMessage(overrides: Partial<UIMessage> = {}): UIMessage {
  return {
    id: "tc-1",
    role: "tool_call",
    content: "",
    timestamp: Date.now(),
    ...overrides,
  };
}

describe("ExpandedToolContent", () => {
  it("routes Bash tool to bash-content renderer", () => {
    render(
      <ExpandedToolContent
        message={makeMessage({
          toolName: "Bash",
          toolInput: { command: "echo hello" },
          toolResult: { stdout: "hello" },
        })}
      />
    );
    expect(screen.getByTestId("bash-content")).toBeInTheDocument();
  });

  it("routes a deliberate terminal run to the terminal block", async () => {
    render(
      <ExpandedToolContent
        message={makeMessage({
          toolName: "compozy__terminal_exec",
          toolInput: { command: "bun run dev" },
          toolResult: {
            rawOutput: {
              terminal_id: "term-4f21c9a03b7e",
              output: "VITE ready in 412 ms",
              still_running: true,
            },
          },
        })}
      />
    );

    // The emulator arrives behind a lazy boundary, so the output stands in
    // until it lands — the transcript is never blank.
    expect(await screen.findByTestId("terminal-content")).toBeInTheDocument();
    expect(screen.getByTestId("terminal-content-fallback")).toHaveTextContent(
      "VITE ready in 412 ms"
    );
  });

  it("routes terminal open as a live terminal even without an explicit running flag", async () => {
    blockProps.mockClear();
    render(
      <ExpandedToolContent
        message={makeMessage({
          toolName: "compozy__terminal_open",
          toolInput: { title: "Build logs" },
          toolResult: {
            rawOutput: {
              terminal_id: "term-open",
              output: "waiting for input",
            },
          },
        })}
      />
    );

    await waitFor(() => expect(blockProps).toHaveBeenCalled());
    expect(blockProps.mock.calls.at(-1)?.[0]).toMatchObject({
      stillRunning: true,
      terminalId: "term-open",
    });
  });

  it("scopes a still-running terminal to the session's own profile", async () => {
    blockProps.mockClear();
    render(
      <ExpandedToolContent
        message={makeMessage({
          toolName: "compozy__terminal_exec",
          toolInput: { command: "bun run dev" },
          toolResult: {
            rawOutput: {
              terminal_id: "term-4f21c9a03b7e",
              output: "VITE ready in 412 ms",
              still_running: true,
            },
          },
        })}
      />,
      // The conversation belongs to `work`, whatever profile the shell is
      // currently showing — a link or the all-profiles view changes neither.
      { profileName: "work" }
    );

    await waitFor(() => expect(blockProps).toHaveBeenCalled());
    expect(blockProps.mock.calls.at(-1)?.[0]).toMatchObject({
      scope: { workspaceId: WORKSPACE_ID, profile: "work" },
    });
  });

  it("shows the recorded screen when the session's profile is not known yet", async () => {
    blockProps.mockClear();
    render(
      <ExpandedToolContent
        message={makeMessage({
          toolName: "compozy__terminal_exec",
          toolInput: { command: "bun run dev" },
          toolResult: {
            rawOutput: {
              terminal_id: "term-4f21c9a03b7e",
              output: "VITE ready in 412 ms",
              still_running: true,
            },
          },
        })}
      />
    );

    // Attaching with a guessed profile is `terminal_not_found`, so the block
    // paints what was recorded instead of pretending to follow a stream.
    await waitFor(() => expect(blockProps).toHaveBeenCalled());
    expect(blockProps.mock.calls.at(-1)?.[0].scope).toBeUndefined();
  });

  it("keeps a pipe exec with no terminal as plain output", () => {
    render(
      <ExpandedToolContent
        message={makeMessage({
          toolName: "compozy__terminal_exec",
          toolInput: { command: "bun test" },
          toolResult: { rawOutput: { output: "41 passed", exit_code: 0 } },
        })}
      />
    );

    // A command that finished without a terminal object never had a window;
    // dressing it as one would claim a surface that does not exist.
    expect(screen.getByTestId("terminal-content-plain")).toHaveTextContent("41 passed");
    expect(screen.queryByTestId("terminal-content")).not.toBeInTheDocument();
  });

  it("routes Read tool to read-content renderer", () => {
    render(
      <ExpandedToolContent
        message={makeMessage({
          toolName: "Read",
          toolInput: { file_path: "/src/main.ts" },
          toolResult: { stdout: "const x = 1;\n" },
        })}
      />
    );
    expect(screen.getByTestId("read-content")).toBeInTheDocument();
  });

  it("routes Write tool to write-content renderer", () => {
    render(
      <ExpandedToolContent
        message={makeMessage({
          toolName: "Write",
          toolInput: { file_path: "/out.txt", content: "hello" },
          toolResult: { content: "ok" },
        })}
      />
    );
    expect(screen.getByTestId("write-content")).toBeInTheDocument();
  });

  it("routes Edit tool to edit-content renderer", () => {
    render(
      <ExpandedToolContent
        message={makeMessage({
          toolName: "Edit",
          toolInput: { file_path: "/a.ts", old_string: "foo", new_string: "bar" },
          toolResult: { content: "ok" },
        })}
      />
    );
    expect(screen.getByTestId("edit-content")).toBeInTheDocument();
  });

  it("routes Grep tool to search-content renderer", () => {
    render(
      <ExpandedToolContent
        message={makeMessage({
          toolName: "Grep",
          toolInput: { pattern: "TODO" },
          toolResult: { stdout: "file1.ts\nfile2.ts" },
        })}
      />
    );
    expect(screen.getByTestId("search-content")).toBeInTheDocument();
  });

  it("routes Glob tool to search-content renderer", () => {
    render(
      <ExpandedToolContent
        message={makeMessage({
          toolName: "Glob",
          toolInput: { pattern: "**/*.ts" },
          toolResult: { stdout: "src/a.ts\nsrc/b.ts" },
        })}
      />
    );
    expect(screen.getByTestId("search-content")).toBeInTheDocument();
  });

  it("routes unknown tool to generic-content fallback", () => {
    render(
      <ExpandedToolContent
        message={makeMessage({
          toolName: "SomeUnknownTool",
          toolInput: { key: "value" },
          toolResult: { content: "result text" },
        })}
      />
    );
    // GenericContent renders pre-formatted JSON, check for input content
    expect(screen.getByText(/"key": "value"/)).toBeInTheDocument();
  });
});
