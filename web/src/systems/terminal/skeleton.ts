/**
 * The Terminal app's window-body loading shapes, on their own entry so a caller that shows them
 * before the terminal code has loaded (the Terminal window's loading fallback) does not pull the
 * emulator, the connection or the barrel in with them.
 */
export { TerminalAggregateSkeleton, TerminalWindowSkeleton } from "./components/terminal-skeletons";
