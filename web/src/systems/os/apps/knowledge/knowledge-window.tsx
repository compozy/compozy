import { KnowledgeLocation } from "./knowledge-location";

/** Knowledge reads the window route so a palette row can focus a memory. */
export function KnowledgeWindow({ windowId }: { windowId: string }) {
  return <KnowledgeLocation windowId={windowId} />;
}
