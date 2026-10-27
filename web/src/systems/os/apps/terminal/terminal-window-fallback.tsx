import { TerminalAggregateSkeleton, TerminalWindowSkeleton } from "@/systems/terminal/skeleton";

import type { OsWindowFallbackProps } from "../../lib/os-types";

/**
 * The Terminal window's body while its code loads: the shape the window loads into once its code
 * is there, which its scope chooses and its route does not. An unbound window on the All projects
 * desktop lists every project's terminals, so it holds that list; any other window attaches to
 * one terminal of its project, so it holds the emulator's dark ground. Both are placeholders: the
 * live grid keeps zero animation and mounts with the window.
 */
export function TerminalWindowFallback({ aggregate }: OsWindowFallbackProps) {
  return aggregate ? (
    <TerminalAggregateSkeleton />
  ) : (
    <TerminalWindowSkeleton label="Loading terminals" />
  );
}
