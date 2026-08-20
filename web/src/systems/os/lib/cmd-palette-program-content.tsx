import {
  OsPaletteProgramBand,
  OsPaletteProgramFailure,
  OsPaletteProgramReloaded,
} from "../components/os-palette-program-status";
import type { CmdPaletteViewProgramPhase } from "../stores/cmd-palette-view-program-store";
import type { PaletteViewContent } from "./palette-view-registry";

export function programViewContentForPhase(
  content: PaletteViewContent,
  phase: CmdPaletteViewProgramPhase,
  reloaded: boolean,
  title: string,
  viewId: string,
  error: string | null,
  retry: () => void
): PaletteViewContent {
  if (phase === "busy" || phase === "degraded") {
    return {
      ...content,
      header: (
        <>
          {content.header}
          <OsPaletteProgramBand phase={phase} onRetry={retry} />
        </>
      ),
    };
  }
  if (phase === "circuit-open" || phase === "unavailable") {
    const sourceMatch = /^ext\.([^.]+)\./.exec(viewId);
    return {
      kind: "list",
      rows: [],
      header: null,
      empty: (
        <OsPaletteProgramFailure
          error={error}
          phase={phase}
          source={`${title} (${sourceMatch?.[1] ? `ext.${sourceMatch[1]}` : viewId})`}
        />
      ),
      note: null,
      backHint: "back",
      resetKey: phase,
      onEmptyQueryBackspace: () => false,
    };
  }
  if (reloaded) {
    return {
      ...content,
      header: (
        <>
          {content.header}
          <OsPaletteProgramReloaded />
        </>
      ),
    };
  }
  return content;
}
