import { Suspense, lazy } from "react";

import type { UIMessage } from "../../types";
import { BashContent } from "./bash-content";
import { ReadContent } from "./read-content";
import { WriteContent } from "./write-content";
import { EditContent } from "./edit-content";
import { SearchContent } from "./search-content";
import { GenericContent } from "./generic-content";
import { TodoContent } from "./todo-content";

const TerminalContent = lazy(async () => {
  const { TerminalContent: Content } = await import("./terminal-content");
  return { default: Content };
});

/** Input and result fields the Bash view renders; anything else needs the generic view. */
const BASH_INPUT_FIELDS = new Set(["command"]);
const BASH_RESULT_FIELDS = new Set(["stdout", "stderr", "content", "error"]);

function isPresent(value: unknown): boolean {
  if (Array.isArray(value)) return value.length > 0;
  return value !== undefined && value !== null && value !== "" && value !== false;
}

function bashViewHoldsPayload(message: UIMessage): boolean {
  const input = message.toolInput;
  if (typeof input?.command !== "string") return false;
  if (Object.keys(input).some(key => !BASH_INPUT_FIELDS.has(key) && isPresent(input[key]))) {
    return false;
  }
  const result = message.toolResult ?? {};
  return Object.entries(result).every(
    ([key, value]) => BASH_RESULT_FIELDS.has(key) || !isPresent(value)
  );
}

/** Routes a UIMessage to its tool-specific expanded renderer. */
export function ExpandedToolContent({ message }: { message: UIMessage }) {
  switch (message.toolName) {
    case "Bash":
      return <BashContent message={message} />;
    // Deliberate terminal use renders as the terminal it ran in, not as generic
    // tool output — the contrast with an agent's own reported output is the
    // point (US-011 vs US-025). Pipe exec (no terminal object) stays in this
    // dispatcher; a supervised id is intercepted earlier as its own block.
    case "compozy__terminal_exec":
    case "compozy__terminal_open":
      return (
        <Suspense fallback={null}>
          <TerminalContent message={message} />
        </Suspense>
      );
    case "Read":
      return <ReadContent message={message} />;
    case "Write":
      return <WriteContent message={message} />;
    case "Edit":
      return <EditContent message={message} />;
    case "Grep":
    case "Glob":
      return <SearchContent message={message} />;
    case "TodoWrite":
      return <TodoContent message={message} />;
    default:
      // A provider that names its shell tool by title ("Terminal") still sent
      // a command: it reads as the command and its output when that view holds
      // the whole payload; any other field keeps the generic Input/Output.
      return bashViewHoldsPayload(message) ? (
        <BashContent message={message} />
      ) : (
        <GenericContent message={message} />
      );
  }
}
