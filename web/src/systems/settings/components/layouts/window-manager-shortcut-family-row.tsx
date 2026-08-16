import { ChevronRight } from "lucide-react";

import { cn } from "@compozy/ui";

import { ShortcutBindingKeys, type ShortcutConflict, type ShortcutMap } from "@/systems/os";

export function WindowManagerShortcutFamilyRow({
  familyId,
  label,
  memberIds,
  defaults,
  effective,
  conflicts,
  expanded,
  onExpand,
}: {
  familyId: "desktop.switch" | "window.tab.jump";
  label: string;
  memberIds: readonly string[];
  defaults: ShortcutMap;
  effective: ShortcutMap;
  conflicts: readonly ShortcutConflict[];
  expanded: boolean;
  onExpand: () => void;
}) {
  const memberIdSet = new Set(memberIds);
  const changed = memberIds.filter(
    id => JSON.stringify(defaults[id] ?? []) !== JSON.stringify(effective[id] ?? [])
  );
  const conflict = conflicts.find(entry => entry.actionIds.some(id => memberIdSet.has(id)));
  const layers = Math.max(...memberIds.map(id => effective[id]?.length ?? 0), 0);

  return (
    <div
      className={cn(
        "border-t border-line-soft first:border-t-0",
        conflict?.kind === "blocked" && "bg-danger-tint"
      )}
      data-testid={`window-manager-shortcut-${familyId}`}
    >
      <button
        aria-expanded={expanded}
        className="flex min-h-setting-row w-full items-center gap-2.5 px-4 py-2 text-left hover:bg-row-hover focus-visible:outline-none focus-visible:shadow-focus-ring"
        type="button"
        onClick={onExpand}
      >
        <ChevronRight
          aria-hidden="true"
          className={cn(
            "size-3 text-faint transition-transform duration-base",
            expanded && "rotate-90"
          )}
        />
        <span className="min-w-0 text-small-body text-fg">{label}</span>
        <span className="hidden font-mono text-mono-id text-faint lg:inline">{familyId}</span>
        {changed.length > 0 ? (
          <span className="font-mono text-badge text-accent-strong">{changed.length} changed</span>
        ) : null}
        <span className="flex-1" />
        <span className="flex flex-col items-end gap-1">
          {Array.from({ length: layers }, (_, layer) => {
            const first = effective[memberIds[0] ?? ""]?.[layer];
            const last = effective[memberIds.at(-1) ?? ""]?.[layer];
            if (!first || !last) return null;
            return (
              <span className="inline-flex items-center gap-1" key={layer}>
                <ShortcutBindingKeys bindings={[first]} overridden={changed.length > 0} compact />
                <span className="text-faint">…</span>
                <ShortcutBindingKeys bindings={[last]} overridden={changed.length > 0} compact />
              </span>
            );
          })}
        </span>
      </button>
      {conflict ? (
        <p
          className={cn(
            "px-10 pb-2 text-form-hint",
            conflict.kind === "blocked" ? "text-danger" : "text-warning"
          )}
          role="status"
        >
          {conflict.kind === "blocked"
            ? `Blocked — ${conflict.chord} conflicts at ${conflict.actionIds.join(" and ")}.`
            : `Shadowed in ${conflict.winner?.surface ?? "a focused surface"} — ${conflict.winner?.label ?? "the local action"} wins there.`}
        </p>
      ) : null}
    </div>
  );
}
