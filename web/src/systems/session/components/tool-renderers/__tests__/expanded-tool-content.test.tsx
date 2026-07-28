import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import type { UIMessage } from "../../../types";

vi.mock("@/lib/utils", () => ({
  cn: (...args: unknown[]) => args.filter(Boolean).join(" "),
}));

import { ExpandedToolContent } from "../expanded-tool-content";

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
