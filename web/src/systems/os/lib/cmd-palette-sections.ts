import type {
  CmdPaletteRankSignals,
  PaletteRegistry,
  ResolvedPaletteCommand,
} from "./cmd-palette-types";
import { ghostCompletion } from "./ranking/ghost";
import { normalizeRankingText } from "./ranking/normalize";
import { rankCandidates } from "./ranking/rank";
import { assembleRankingResults } from "./ranking/sections";
import type { RankingCandidate } from "./ranking/types";

/**
 * Section assembly for the palette root and destination mode.
 *
 * The daemon-served snapshot supplies every ranking constant. The cold path is
 * deliberately unranked and uncapped while that session-only snapshot loads.
 */

export interface PaletteSection {
  readonly title: string;
  readonly commands: readonly ResolvedPaletteCommand[];
  /** Total matches before the cap; equal to `commands.length` when nothing was withheld. */
  readonly total: number;
}

export const PALETTE_AGENT_FALLBACK_VALUE = "fallback:agent";

export interface PaletteAgentFallback {
  readonly value: typeof PALETTE_AGENT_FALLBACK_VALUE;
  readonly query: string;
}

export interface PaletteResultAssembly {
  readonly sections: readonly PaletteSection[];
  readonly fallback: PaletteAgentFallback | null;
}

function normalize(value: string): string {
  return normalizeRankingText(value).text;
}

/** Substring match over title, id, alias and keywords; the scorer lands in P2. */
export function commandMatchesQuery(command: ResolvedPaletteCommand, query: string): boolean {
  const trimmed = query.trim();
  if (trimmed === "") return true;
  const needle = normalize(trimmed);
  if (needle === "") return false;
  const haystack = [command.title, command.id, command.alias ?? "", ...(command.keywords ?? [])];
  return haystack.some(entry => normalize(entry).includes(needle));
}

/**
 * Available commands sort above disabled ones inside a group: a blocked row
 * stays visible and honest (BR-8) without pushing runnable commands out of the
 * cap.
 */
function compareCommands(left: ResolvedPaletteCommand, right: ResolvedPaletteCommand): number {
  if (left.available !== right.available) return left.available ? -1 : 1;
  return left.title.localeCompare(right.title) || left.id.localeCompare(right.id);
}

export interface PaletteSectionInput {
  readonly registry: PaletteRegistry;
  readonly query: string;
  /**
   * Destination mode offers only what a new tab can become: navigable app
   * targets. Ineligible groups are absent, not disabled (`_uiux.md` S10).
   */
  readonly destination: boolean;
  readonly signals: CmdPaletteRankSignals | null;
  readonly fallbackAgentEnabled: boolean;
}

interface CommandRankingCandidate extends RankingCandidate {
  readonly command: ResolvedPaletteCommand;
}

function commandRankingCandidates(
  registry: PaletteRegistry,
  destination: boolean
): readonly CommandRankingCandidate[] {
  const candidates: CommandRankingCandidate[] = [];
  for (const command of registry.commands) {
    if (destination && command.action.kind !== "navigate") continue;
    candidates.push({
      stableKey: command.id,
      id: command.id,
      label: command.title,
      group: command.section.trim() || "Commands",
      aliases: command.alias ? [command.alias] : [],
      keywords: command.keywords ?? [],
      contextual: command.available && (command.when?.length ?? 0) > 0,
      available: command.available,
      curated: true,
      subtype: "command",
      command,
    });
  }
  return candidates;
}

export function paletteGhostCompletion(
  registry: PaletteRegistry,
  query: string,
  destination: boolean,
  signals: CmdPaletteRankSignals | null
): string | null {
  if (signals === null) return null;
  return ghostCompletion(
    query,
    rankCandidates(query, commandRankingCandidates(registry, destination), signals),
    signals.weights
  );
}

export function assemblePaletteResults({
  registry,
  query,
  destination,
  signals,
  fallbackAgentEnabled,
}: PaletteSectionInput): PaletteResultAssembly {
  const eligible = registry.commands.filter(
    command => !destination || command.action.kind === "navigate"
  );
  if (signals !== null) {
    const candidates = commandRankingCandidates(registry, destination);
    const assembled = assembleRankingResults(query, candidates, signals);
    return {
      sections: assembled.sections.map(section => ({
        title: section.title,
        commands: section.candidates.map(candidate => candidate.candidate.command),
        total: section.total,
      })),
      fallback:
        fallbackAgentEnabled && !destination && assembled.fallback
          ? { value: PALETTE_AGENT_FALLBACK_VALUE, query }
          : null,
    };
  }

  const grouped = new Map<string, ResolvedPaletteCommand[]>();
  for (const command of eligible) {
    if (!commandMatchesQuery(command, query)) continue;
    const title = command.section.trim() || "Commands";
    const bucket = grouped.get(title);
    if (bucket) bucket.push(command);
    else grouped.set(title, [command]);
  }
  const sections: PaletteSection[] = [];
  for (const [title, commands] of grouped) {
    const sorted = [...commands].sort(compareCommands);
    sections.push({ title, commands: sorted, total: sorted.length });
  }
  return {
    sections,
    fallback:
      fallbackAgentEnabled && !destination && query.trim() !== "" && sections.length === 0
        ? { value: PALETTE_AGENT_FALLBACK_VALUE, query }
        : null,
  };
}

/** The exact overflow note for a capped group — never a vague "and more". */
export function overflowNote(section: PaletteSection): string | null {
  return section.total > section.commands.length
    ? `showing ${section.commands.length} of ${section.total}`
    : null;
}
