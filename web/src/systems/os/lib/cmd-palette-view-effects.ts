import type { CmdPaletteViewEffect } from "./cmd-palette-types";

interface ViewEffectFailureDetails {
  readonly effect_id: string;
  readonly error: unknown;
}

type ViewEffectFailureReporter = (message: string, details: ViewEffectFailureDetails) => void;

export function finalizeCmdPaletteViewEffects(
  effects: readonly CmdPaletteViewEffect[],
  results: readonly PromiseSettledResult<void>[],
  reportFailure: ViewEffectFailureReporter = defaultViewEffectFailureReporter
): readonly string[] {
  for (const [index, result] of results.entries()) {
    const effect = effects[index];
    if (effect && result.status === "rejected") {
      reportFailure("Command palette view effect failed", {
        effect_id: effect.id,
        error: result.reason,
      });
    }
  }
  return effects.map(effect => effect.id);
}

function defaultViewEffectFailureReporter(
  message: string,
  details: ViewEffectFailureDetails
): void {
  console.warn(message, details);
}
