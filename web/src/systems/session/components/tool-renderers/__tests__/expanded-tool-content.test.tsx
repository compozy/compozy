import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render as renderBare, screen } from "@testing-library/react";
import type { ReactElement } from "react";
import { describe, expect, it, vi } from "vitest";

import { SessionRuntimeRenderProvider } from "../../../lib/session-runtime-render-context";
import type { UIMessage } from "../../../types";

vi.mock("@/lib/utils", () => ({
  cn: (...args: unknown[]) => args.filter(Boolean).join(" "),
}));

import { ExpandedToolContent } from "../expanded-tool-content";

/** The transcript's own surroundings, as the dispatcher is rendered inside a row body. */
function render(ui: ReactElement) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return renderBare(
    <QueryClientProvider client={client}>
      <SessionRuntimeRenderProvider sessionId="sess-77ab" workspaceId="ws-atlas">
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

  it("keeps a pipe exec with no terminal as plain output", async () => {
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
    expect(await screen.findByTestId("terminal-content-plain")).toHaveTextContent("41 passed");
    expect(screen.queryByTestId("terminal-content")).not.toBeInTheDocument();
  });

  it("Should never paint the tool envelope as terminal output when the output is empty", () => {
    // Invariant: an empty `output` stays empty. MCP results also serialize the
    // envelope into `content`; reading it would show raw JSON as if printed.
    const envelope = { command_id: "cmd-1", exit_code: 0, output: "", untrusted: true };
    const { container } = render(
      <ExpandedToolContent
        message={makeMessage({
          toolName: "compozy__terminal_exec",
          toolInput: { command: "true" },
          toolResult: { content: JSON.stringify(envelope), rawOutput: { structured: envelope } },
        })}
      />
    );

    expect(screen.queryByTestId("terminal-content-plain")).not.toBeInTheDocument();
    expect(container).not.toHaveTextContent("command_id");
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
