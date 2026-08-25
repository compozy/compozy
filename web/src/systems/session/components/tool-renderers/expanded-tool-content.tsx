import type { UIMessage } from "../../types";
import { BashContent } from "./bash-content";
import { ReadContent } from "./read-content";
import { WriteContent } from "./write-content";
import { EditContent } from "./edit-content";
import { SearchContent } from "./search-content";
import { GenericContent } from "./generic-content";
import { TerminalContent } from "./terminal-content";
import { TodoContent } from "./todo-content";

/** Routes a UIMessage to its tool-specific expanded renderer. */
export function ExpandedToolContent({ message }: { message: UIMessage }) {
  switch (message.toolName) {
    case "Bash":
      return <BashContent message={message} />;
    // Deliberate terminal use renders as the terminal it ran in, not as generic
    // tool output — the contrast with an agent's own reported output is the
    // point (US-011 vs US-025).
    case "compozy__terminal_exec":
    case "compozy__terminal_open":
      return <TerminalContent message={message} />;
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
      return <GenericContent message={message} />;
  }
}
