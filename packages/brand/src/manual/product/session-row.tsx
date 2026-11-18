// One session row as the product draws it: glyph + name + agent + state word + age.
// theme picks the AA colours for the state (spec values in data.ts).
import { cn } from "@compozy/ui/utils";

import { rowTheme, states, type Session } from "./data";

interface SessionRowProps extends Session {
  theme?: "dark" | "light";
  selected?: boolean;
  compact?: boolean;
}

export function SessionRow({
  name,
  agent,
  state,
  age,
  note,
  theme = "dark",
  selected = false,
  compact = false,
}: SessionRowProps) {
  const s = states.find(x => x.key === state) ?? states[0]!;
  const c = theme === "dark" ? s.dark : s.light;
  const t = rowTheme[theme];
  return (
    <div
      className={cn(
        "grid grid-cols-[16px_minmax(0,1fr)_auto] items-center gap-x-2.5 rounded-[6px] px-2.5",
        compact ? "min-h-7" : "min-h-8 py-1"
      )}
      style={{
        color: t.fg,
        ...(selected ? { background: t.sel, boxShadow: `inset 2px 0 0 ${t.bar}` } : {}),
      }}
      aria-description={s.word}
    >
      <span
        className={cn(
          "text-center font-mono text-[12px]",
          state === "working" && "motion-safe:animate-pulse"
        )}
        style={{ color: c }}
        aria-hidden="true"
      >
        {s.glyph}
      </span>
      <span className="min-w-0 truncate text-[13.5px]">
        <span className="font-medium">{name}</span>{" "}
        <span className="font-mono text-[11.5px]" style={{ color: t.muted }}>
          · {agent}
        </span>
        {note && (
          <span className="block truncate text-[12px]" style={{ color: t.muted }}>
            {note}
          </span>
        )}
      </span>
      <span className="flex items-center gap-2 font-mono text-[11.5px]">
        <span style={{ color: state === "idle" || state === "later" ? t.muted : c }}>{s.word}</span>
        <span style={{ color: t.muted }}>{age}</span>
      </span>
    </div>
  );
}
