/**
 * The session inspector surface: which tabs exist, who asks for one, and the
 * panel that renders them.
 */
export {
  SESSION_INSPECTOR_TAB_IDS,
  SESSION_INSPECTOR_TAB_TESTIDS,
  SESSION_INSPECTOR_TABS,
  isInspectorTabId,
  type InspectorTabId,
} from "./lib/session-inspector-tabs";
export {
  requestSessionInspectorTab,
  useSessionInspectorState,
  type UseSessionInspectorStateResult,
} from "./hooks/use-session-inspector-state";
export {
  SessionInspector,
  type InspectorMemoryState,
  type InspectorSessionLedger,
  type InspectorUsage,
  type SessionInspectorProps,
} from "./components/session-inspector";
export { deriveFileReads, type InspectorFileEntry } from "./components/session-inspector.logic";
