// Live byte-stream grid. The emulator itself is fetched on first attach, so
// importing this file costs nothing until a terminal is actually shown.
export {
  TerminalView,
  type TerminalDimensions,
  type TerminalEngine,
  type TerminalEngineLoader,
  type TerminalRendererKind,
  type TerminalSelectionRange,
  type TerminalViewHandle,
  type TerminalViewProps,
} from "../components/terminal/terminal-view";
export {
  destroyTerminalInstance,
  destroyTerminalInstances,
} from "../components/terminal/terminal-registry";
export { TerminalWriteAbandonedError } from "../components/terminal/terminal-write-abandoned";
