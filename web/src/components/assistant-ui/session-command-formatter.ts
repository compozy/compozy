import type { Unstable_DirectiveFormatter, Unstable_DirectiveSegment } from "@assistant-ui/react";

import {
  formatCommandChipLabel,
  type SessionComposerCommandCatalog,
} from "./session-command-menu-model";

export const SKILL_DIRECTIVE_TYPE = "session-command-skill";

const COMMAND_TOKEN_RE = /(^|\s)(\/[a-zA-Z][\w:.-]*)(?=\s|$)/gu;

/** Every skill token the daemon catalog exposes, across both placements. */
export function skillChipTokens(catalog: SessionComposerCommandCatalog): ReadonlySet<string> {
  const tokens = new Set<string>();
  for (const command of catalog.inlineSkills) tokens.add(command.token);
  for (const section of catalog.standaloneSections) {
    for (const command of section.commands) {
      if (command.lane === "skill") tokens.add(command.token);
    }
  }
  return tokens;
}

/**
 * Directive formatter over the raw prompt text. `serialize` always emits the
 * canonical token — the daemon wire format — so the composer text stays plain.
 * `parse` re-materializes inline chips only for skill tokens the daemon
 * catalog actually knows, mirroring the transcript where only parsed skill
 * invocations render as pills.
 */
export function createSessionCommandFormatter(
  catalog: SessionComposerCommandCatalog
): Unstable_DirectiveFormatter {
  const chipTokens = skillChipTokens(catalog);
  return {
    serialize(item) {
      if (typeof item.metadata?.token === "string") return item.metadata.token;
      if (item.id.startsWith("/")) return item.id;
      return item.label;
    },
    parse(text) {
      const segments: Unstable_DirectiveSegment[] = [];
      let lastIndex = 0;
      for (const match of text.matchAll(COMMAND_TOKEN_RE)) {
        const token = match[2]!;
        if (!chipTokens.has(token)) continue;
        const tokenStart = match.index + match[1]!.length;
        if (tokenStart > lastIndex) {
          segments.push({ kind: "text", text: text.slice(lastIndex, tokenStart) });
        }
        segments.push({
          kind: "mention",
          type: SKILL_DIRECTIVE_TYPE,
          label: formatCommandChipLabel(token),
          id: token,
        });
        lastIndex = tokenStart + token.length;
      }
      if (segments.length === 0) return [{ kind: "text", text }];
      if (lastIndex < text.length) {
        segments.push({ kind: "text", text: text.slice(lastIndex) });
      }
      return segments;
    },
  };
}
