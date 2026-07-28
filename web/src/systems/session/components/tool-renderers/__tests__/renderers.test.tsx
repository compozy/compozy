import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import type { UIMessage } from "../../../types";

vi.mock("@/lib/utils", () => ({
  cn: (...args: unknown[]) => args.filter(Boolean).join(" "),
}));

import { BashContent } from "../bash-content";
import { ReadContent } from "../read-content";
import { WriteContent } from "../write-content";
import { EditContent } from "../edit-content";
import { SearchContent } from "../search-content";
import { GenericContent } from "../generic-content";
import { SessionToolCallRow } from "../../tool-call-card";

function makeMessage(overrides: Partial<UIMessage> = {}): UIMessage {
  return {
    id: "tc-1",
    role: "tool_call",
    content: "",
    timestamp: Date.now(),
    ...overrides,
  };
}

describe("BashContent", () => {
  it("renders command and stdout from toolResult", () => {
    render(
      <BashContent
        message={makeMessage({
          toolName: "Bash",
          toolInput: { command: "echo hello" },
          toolResult: { stdout: "hello\nworld" },
        })}
      />
    );
    expect(screen.getByText("echo hello")).toBeInTheDocument();
    // stdout is rendered in a pre element; getByText normalizes whitespace so use getAllByText
    const matches = screen.getAllByText(/hello/);
    expect(matches.length).toBeGreaterThanOrEqual(1);
  });

  it("renders stderr in error styling when present", () => {
    render(
      <BashContent
        message={makeMessage({
          toolName: "Bash",
          toolInput: { command: "bad-cmd" },
          toolResult: { stderr: "command not found" },
        })}
      />
    );
    expect(screen.getByText("command not found")).toBeInTheDocument();
    const stderrEl = screen.getByText("command not found");
    expect(stderrEl.closest("pre")).toHaveClass("text-danger");
  });

  it("renders without command when toolInput has no command", () => {
    render(
      <BashContent
        message={makeMessage({
          toolName: "Bash",
          toolResult: { stdout: "output" },
        })}
      />
    );
    expect(screen.getByText("output")).toBeInTheDocument();
    expect(screen.queryByText("$")).not.toBeInTheDocument();
  });
});

describe("ReadContent", () => {
  it("renders file path and line count from stdout", () => {
    render(
      <ReadContent
        message={makeMessage({
          toolName: "Read",
          toolInput: { file_path: "/src/main.ts" },
          toolResult: { stdout: "line1\nline2\nline3" },
        })}
      />
    );
    expect(screen.getByText("/src/main.ts")).toBeInTheDocument();
    expect(screen.getByText("3 lines")).toBeInTheDocument();
  });

  it("renders file path and line count from content", () => {
    render(
      <ReadContent
        message={makeMessage({
          toolName: "Read",
          toolInput: { file_path: "/src/app.tsx" },
          toolResult: { content: "a\nb" },
        })}
      />
    );
    expect(screen.getByText("/src/app.tsx")).toBeInTheDocument();
    expect(screen.getByText("2 lines")).toBeInTheDocument();
  });

  it("renders file path only when no result content", () => {
    render(
      <ReadContent
        message={makeMessage({
          toolName: "Read",
          toolInput: { file_path: "/src/utils.ts" },
        })}
      />
    );
    expect(screen.getByText("/src/utils.ts")).toBeInTheDocument();
  });
});

describe("WriteContent", () => {
  it("renders file path and content preview", () => {
    render(
      <WriteContent
        message={makeMessage({
          toolName: "Write",
          toolInput: { file_path: "/out.txt", content: "hello world" },
        })}
      />
    );
    expect(screen.getByText("/out.txt")).toBeInTheDocument();
    expect(screen.getByText("hello world")).toBeInTheDocument();
  });

  it("truncates long content", () => {
    const longContent = "a".repeat(2500);
    render(
      <WriteContent
        message={makeMessage({
          toolName: "Write",
          toolInput: { file_path: "/big.txt", content: longContent },
        })}
      />
    );
    const pre = screen.getByText(/a+\u2026/);
    expect(pre).toBeInTheDocument();
  });
});

describe("EditContent", () => {
  it("renders file path with old → new diff", () => {
    render(
      <EditContent
        message={makeMessage({
          toolName: "Edit",
          toolInput: {
            file_path: "/src/app.ts",
            old_string: "const x = 1;",
            new_string: "const x = 2;",
          },
        })}
      />
    );
    expect(screen.getByText("/src/app.ts")).toBeInTheDocument();
    expect(screen.getByText("- const x = 1;")).toBeInTheDocument();
    expect(screen.getByText("+ const x = 2;")).toBeInTheDocument();
  });

  it("renders only new string when no old string", () => {
    render(
      <EditContent
        message={makeMessage({
          toolName: "Edit",
          toolInput: {
            file_path: "/new.ts",
            new_string: "export const y = 3;",
          },
        })}
      />
    );
    expect(screen.getByText("+ export const y = 3;")).toBeInTheDocument();
  });
});

describe("SearchContent", () => {
  it("renders pattern and matched files list", () => {
    render(
      <SearchContent
        message={makeMessage({
          toolName: "Grep",
          toolInput: { pattern: "TODO" },
          toolResult: { stdout: "src/a.ts\nsrc/b.ts\nsrc/c.ts" },
        })}
      />
    );
    expect(screen.getByText("TODO")).toBeInTheDocument();
    // shortenPath returns paths with ≤3 segments as-is
    expect(screen.getByText("src/a.ts")).toBeInTheDocument();
    expect(screen.getByText("src/b.ts")).toBeInTheDocument();
    expect(screen.getByText("src/c.ts")).toBeInTheDocument();
  });

  it("shows 'No matches' when result exists but empty", () => {
    render(
      <SearchContent
        message={makeMessage({
          toolName: "Grep",
          toolInput: { pattern: "NOEXIST" },
          toolResult: { stdout: "" },
        })}
      />
    );
    expect(screen.getByText("No matches")).toBeInTheDocument();
  });
});

describe("GenericContent", () => {
  it("renders toolInput as formatted JSON", () => {
    render(
      <GenericContent
        message={makeMessage({
          toolName: "CustomTool",
          toolInput: { key: "value", num: 42 },
        })}
      />
    );
    expect(screen.getByText(/"key": "value"/)).toBeInTheDocument();
    expect(screen.getByText(/"num": 42/)).toBeInTheDocument();
  });

  it("renders toolResult content as text", () => {
    render(
      <GenericContent
        message={makeMessage({
          toolName: "CustomTool",
          toolResult: { content: "some output" },
        })}
      />
    );
    expect(screen.getByText("some output")).toBeInTheDocument();
  });

  it("renders toolResult error", () => {
    render(
      <GenericContent
        message={makeMessage({
          toolName: "CustomTool",
          toolResult: { error: "something went wrong" },
        })}
      />
    );
    expect(screen.getByText("something went wrong")).toBeInTheDocument();
  });
});

describe("Per-tool renderers inside the SessionToolCallRow inline body (task 25 — no regression)", () => {
  const cases: Array<{ name: string; message: UIMessage; expected: string }> = [
    {
      name: "Bash",
      message: makeMessage({
        toolName: "Bash",
        toolInput: { command: "echo hi" },
        toolResult: { stdout: "bash-output-marker" },
      }),
      expected: "bash-output-marker",
    },
    {
      name: "Read",
      message: makeMessage({
        toolName: "Read",
        toolInput: { file_path: "/src/read-marker.ts" },
        toolResult: { content: "a\nb" },
      }),
      expected: "/src/read-marker.ts",
    },
    {
      name: "Write",
      message: makeMessage({
        toolName: "Write",
        toolInput: { file_path: "/out.txt", content: "write-body-marker" },
        toolResult: { content: "ok" },
      }),
      expected: "write-body-marker",
    },
    {
      name: "Edit",
      message: makeMessage({
        toolName: "Edit",
        toolInput: { file_path: "/src/edit-marker.ts", old_string: "a", new_string: "b" },
        toolResult: { content: "ok" },
      }),
      expected: "/src/edit-marker.ts",
    },
    {
      name: "Grep",
      message: makeMessage({
        toolName: "Grep",
        toolInput: { pattern: "TODO" },
        toolResult: { stdout: "src/grep-marker.ts" },
      }),
      expected: "src/grep-marker.ts",
    },
    {
      name: "generic MCP",
      message: makeMessage({
        toolName: "mcp__context7__resolve-library-id",
        toolInput: { libraryName: "react" },
        toolResult: { content: "/websites/generic-marker" },
      }),
      expected: "/websites/generic-marker",
    },
  ];

  for (const { name, message, expected } of cases) {
    it(`Should mount the ${name} renderer unchanged inside the row's expanded body`, () => {
      const { container } = render(<SessionToolCallRow message={message} defaultExpanded />);
      const body = container.querySelector<HTMLElement>('[data-slot="tool-call-row-body"]');
      expect(body).not.toBeNull();
      expect(body).toHaveTextContent(expected);
    });
  }
});
