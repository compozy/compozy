import { KnowledgeLocation } from "./knowledge-location";

/** Knowledge has one route location; its local panels own internal selection state. */
export function KnowledgeWindow(_props: { windowId: string }) {
  return <KnowledgeLocation />;
}
