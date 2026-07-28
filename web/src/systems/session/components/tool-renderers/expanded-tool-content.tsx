import type { UIMessage } from "../../types";
import { BashContent } from "./bash-content";
import { ReadContent } from "./read-content";
import { WriteContent } from "./write-content";
import { EditContent } from "./edit-content";
import { SearchContent } from "./search-content";
import { GenericContent } from "./generic-content";

/** Routes a UIMessage to its tool-specific expanded renderer. */
export function ExpandedToolContent({ message }: { message: UIMessage }) {
  switch (message.toolName) {
    case "Bash":
      return <BashContent message={message} />;
    case "Read":
      return <ReadContent message={message} />;
    case "Write":
      return <WriteContent message={message} />;
    case "Edit":
      return <EditContent message={message} />;
    case "Grep":
    case "Glob":
      return <SearchContent message={message} />;
    default:
      return <GenericContent message={message} />;
  }
}
