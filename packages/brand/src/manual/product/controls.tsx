// 04 Controls: live specimens built only from tokens, rendered in both themes. Each stage sets
// the --k-* scope from @compozy/ui tokens; the light stage mixes them the way the paper band does.
import { cn } from "@compozy/ui/utils";
import type { CSSProperties } from "react";

import { Icon } from "../graphics/icon";
import { Section } from "../parts/section";
import { SpecCard } from "../parts/spec-card";

type Scope = Record<`--k-${string}`, string>;
const mix = (a: string, pct: number, b: string) =>
  `color-mix(in oklab, var(${a}) ${pct}%, var(${b}))`;

const THEMES: { t: "dark" | "light"; name: string; scope: Scope }[] = [
  {
    t: "dark",
    name: "Dark",
    scope: {
      "--k-ink": "var(--color-rail)",
      "--k-ink-2": "var(--color-canvas)",
      "--k-ink-3": "var(--color-elevated)",
      "--k-line": "var(--color-line)",
      "--k-line-strong": "var(--color-line-strong)",
      "--k-fg": "var(--color-fg)",
      "--k-muted": "var(--color-muted-foreground)",
      "--k-faint": "var(--color-disabled)",
      "--k-volt": "var(--color-brand)",
      "--k-volt-hover": "var(--color-brand-hover)",
      "--k-volt-ink": "var(--color-brand-ink)",
      "--k-cyan": "var(--color-brand-cyan)",
      "--k-danger": "var(--color-destructive)",
      "--k-warning": "var(--color-attn)",
    },
  },
  {
    t: "light",
    name: "Light",
    scope: {
      "--k-ink": mix("--color-rail", 4, "--color-fg-strong"),
      "--k-ink-2": "var(--color-fg-strong)",
      "--k-ink-3": mix("--color-rail", 7, "--color-fg-strong"),
      "--k-line": mix("--color-rail", 12, "--color-fg-strong"),
      "--k-line-strong": mix("--color-rail", 20, "--color-fg-strong"),
      "--k-fg": "var(--color-rail)",
      "--k-muted": mix("--color-rail", 62, "--color-fg"),
      "--k-faint": mix("--color-rail", 35, "--color-fg"),
      "--k-volt": mix("--color-brand-volt", 88, "--color-rail"),
      "--k-volt-hover": mix("--color-brand-volt", 76, "--color-rail"),
      "--k-volt-ink": "var(--color-fg-strong)",
      "--k-cyan": mix("--color-brand-cyan", 45, "--color-rail"),
      "--k-danger": mix("--color-destructive", 64, "--color-rail"),
      "--k-warning": mix("--color-attn", 62, "--color-rail"),
    },
  },
];

const spec: [string, string][] = [
  ["Button", "Height 32 px (sm 28, lg 40) · radius 10 · Figtree 450 13.5 px · padding 12 px"],
  [
    "Primary",
    "Volt fill, white label; one per view region. Hover volt-hover, press 120 ms scale .98",
  ],
  ["Secondary", "1 px line-strong, fg label; hover border cyan"],
  ["Danger", "Danger text on transparent; fill only inside a confirm dialog"],
  ["Input", "Height 32 · radius 10 · ink-3 fill · 1 px line-strong · placeholder muted"],
  ["Focus", "2 px cyan ring, 2 px offset, on every focusable element; never removed"],
  ["Toggle", "32 × 18 track; on = volt, off = line-strong; knob 14 px fg"],
  ["Tabs", "Segmented in toolbars, underline in panes; selected = fg + 2 px volt bar"],
  ["Menu", "ink-3, radius 10, 1 px line-strong, rows 28 px, shortcut in mono muted"],
];

const group = "border-t border-(--k-line) py-4";
const capCls = "mb-2.5 block font-mono text-[11px] font-medium text-(--k-muted)";
const btn =
  "inline-flex h-8 cursor-pointer items-center gap-1.5 rounded-[10px] border border-transparent px-3 text-[13.5px] font-[450] transition-[background-color,border-color,transform] duration-(--duration-base) ease-(--ease-out) active:scale-[.98] focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-(--k-cyan) disabled:cursor-not-allowed disabled:border-(--k-line) disabled:text-(--k-faint)";
const sm = "h-7 px-2.5 text-[12.5px]";
const primary = "bg-(--k-volt) text-(--k-volt-ink) hover:bg-(--k-volt-hover)";
const secondary = "border-(--k-line-strong) bg-transparent text-(--k-fg) hover:border-(--k-cyan)";
const ghost = "bg-transparent text-(--k-fg) hover:bg-(--k-ink-3)";
const danger =
  "border-[color-mix(in_oklab,var(--k-danger)_40%,transparent)] bg-transparent text-(--k-danger)";
const input =
  "h-8 min-w-0 rounded-[10px] border border-(--k-line-strong) bg-(--k-ink-3) px-2.5 font-mono text-[13.5px] text-(--k-fg) disabled:bg-(--k-ink-2) disabled:text-(--k-faint)";
const fieldLabel = "text-[12px] font-medium text-(--k-muted)";
const on = "data-[on=true]:bg-(--k-volt)";
const menuItem =
  "flex min-h-7 items-center gap-2 rounded-[6px] px-2 [&_svg]:shrink-0 [&_svg]:text-(--k-muted)";
const kbd = "ml-auto font-mono text-[11.5px] text-(--k-muted)";

function Toggle({ checked = false }: { checked?: boolean }) {
  return (
    <span
      data-on={checked}
      className={cn("relative h-[18px] w-8 flex-none rounded-full bg-(--k-line-strong)", on)}
    >
      <i
        className={cn(
          "absolute top-0.5 left-0.5 size-3.5 rounded-full",
          checked ? "translate-x-3.5 bg-(--color-fg-strong)" : "bg-(--k-fg)"
        )}
      />
    </span>
  );
}

function Stage({ t, name, scope }: (typeof THEMES)[number]) {
  return (
    <div
      data-theme-specimen={t}
      className="min-w-0 rounded-card bg-(--k-ink-2) p-6 text-[14px] text-(--k-fg) shadow-[inset_0_0_0_1px_var(--k-line)]"
      style={scope as CSSProperties}
    >
      <p className="m-0 mb-[18px] mono-label text-[11px] font-medium text-(--k-muted)">
        {name} · data-theme="{t}"
      </p>
      <div className={group}>
        <span className={capCls}>Buttons</span>
        <div className="flex flex-wrap items-center gap-2">
          <button type="button" className={cn(btn, primary)}>
            Start the daemon
          </button>
          <button type="button" className={cn(btn, secondary)}>
            Open session
          </button>
          <button type="button" className={cn(btn, ghost)}>
            <Icon name="loop" size={16} />
            Run Loop
          </button>
          <button type="button" className={cn(btn, danger)}>
            Stop
          </button>
          <button type="button" className={cn(btn, secondary)} disabled>
            Disabled
          </button>
          <button
            type="button"
            className={cn(btn, secondary, "w-8 justify-center px-0")}
            aria-label="New session"
          >
            <Icon name="session" size={16} />
          </button>
        </div>
      </div>
      <div className={group}>
        <span className={capCls}>Inputs</span>
        <div className="grid gap-2 sm:grid-cols-2">
          <label className="grid gap-1">
            <span className={fieldLabel}>Goal</span>
            <input className={input} defaultValue="fix flaky auth tests" />
          </label>
          <label className="grid gap-1">
            <span className={fieldLabel}>Agent · focus</span>
            <input
              className={cn(input, "outline-2 outline-offset-2 outline-(--k-cyan)")}
              defaultValue="claude-code"
            />
          </label>
          <label className="grid gap-1">
            <span className={fieldLabel}>Cron</span>
            <input
              className={cn(input, "border-(--k-danger)")}
              defaultValue="*/5 * * *"
              aria-invalid="true"
            />
            <span className="text-[12px] text-(--k-danger)">
              Use five fields: minute hour day month weekday.
            </span>
          </label>
          <label className="grid gap-1">
            <span className={fieldLabel}>Profile</span>
            <input className={input} defaultValue="default" disabled />
          </label>
        </div>
      </div>
      <div className={cn(group, "grid gap-4 sm:grid-cols-2")}>
        <div>
          <span className={capCls}>Toggles</span>
          <div className="grid gap-2.5 text-[13.5px]">
            <span className="flex items-center gap-2.5">
              <Toggle checked />
              Always-on
            </span>
            <span className="flex items-center gap-2.5">
              <Toggle />
              Ask before running tools
            </span>
            <span className="flex items-center gap-2.5">
              <span className="grid size-4 flex-none place-items-center rounded-[4px] border border-(--k-volt) bg-(--k-volt) text-[11px] text-(--k-volt-ink)">
                ✓
              </span>
              Use a worktree
            </span>
          </div>
        </div>
        <div>
          <span className={capCls}>Tabs</span>
          <div className="inline-flex gap-0.5 rounded-[10px] bg-(--k-ink) p-0.5 text-[13px] shadow-[inset_0_0_0_1px_var(--k-line)]">
            {["Sessions", "Loops", "Automations"].map((x, i) => (
              <span
                key={x}
                className={cn(
                  "rounded-[8px] px-2.5 py-[5px]",
                  i === 0
                    ? "bg-(--k-ink-3) text-(--k-fg) shadow-[inset_0_0_0_1px_var(--k-line-strong)]"
                    : "text-(--k-muted)"
                )}
              >
                {x}
              </span>
            ))}
          </div>
          <div className="mt-3 flex gap-4 border-b border-(--k-line) text-[13px]">
            {["Timeline", "Diff", "Terminal"].map((x, i) => (
              <span
                key={x}
                className={cn(
                  "pt-1.5 pb-2",
                  i === 0
                    ? "text-(--k-fg) shadow-[inset_0_-2px_0_var(--k-volt)]"
                    : "text-(--k-muted)"
                )}
              >
                {x}
              </span>
            ))}
          </div>
        </div>
      </div>
      <div className={cn(group, "grid gap-4 sm:grid-cols-[minmax(0,1fr)_minmax(0,1fr)]")}>
        <div>
          <span className={capCls}>Menu</span>
          <div className="grid rounded-[10px] bg-(--k-ink-3) p-1 text-[13px] shadow-[inset_0_0_0_1px_var(--k-line-strong),0_24px_48px_-24px_rgb(0_0_0/.4)]">
            <span className={menuItem}>
              <Icon name="session" size={16} />
              Continue with another agent…
            </span>
            <span className={cn(menuItem, "bg-(--k-ink-2)")}>
              <Icon name="network" size={16} />
              Fork from here<kbd className={kbd}>⌘ D</kbd>
            </span>
            <span className={menuItem}>
              <Icon name="memory" size={16} />
              Move to Later…<kbd className={kbd}>⌘ L</kbd>
            </span>
            <hr className="my-1 border-0 border-t border-(--k-line)" />
            <span className={cn(menuItem, "text-(--k-danger)")}>
              Stop session<kbd className={kbd}>⌘ .</kbd>
            </span>
          </div>
        </div>
        <div>
          <span className={capCls}>Approval</span>
          <div className="grid gap-2.5 rounded-[14px] bg-(--k-ink-3) p-3 shadow-[inset_0_0_0_1px_var(--k-line-strong),inset_2px_0_0_var(--k-warning)]">
            <p className="m-0 flex items-center gap-2 text-[13.5px] font-medium">
              <span className="text-(--k-warning)">◐</span>codex wants to run a command
            </p>
            <code className="block overflow-x-auto rounded-[6px] bg-(--k-ink) px-2 py-1.5 font-mono text-[12px] whitespace-nowrap">
              bun run db:migrate --env staging
            </code>
            <div className="flex flex-wrap gap-2">
              <button type="button" className={cn(btn, sm, primary)}>
                Allow once
              </button>
              <button type="button" className={cn(btn, sm, secondary)}>
                Always allow
              </button>
              <button type="button" className={cn(btn, sm, ghost)}>
                Deny
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}

export function Controls() {
  return (
    <Section
      id="controls"
      n="04"
      file="controls.tsx"
      title={
        <>
          Controls made of <em>tokens</em>
        </>
      }
      bg="ink2"
      status="proposal"
      intro={
        <p>
          Every specimen below is live HTML that reads only <code>--color-*</code> variables. The
          right column is the same markup inside <code>data-theme="light"</code>: nothing else
          changes, which is the test that the tokens are complete.
        </p>
      }
    >
      <div className="grid gap-4 xl:grid-cols-2">
        {THEMES.map(th => (
          <Stage key={th.t} {...th} />
        ))}
      </div>
      <SpecCard title="Spec" rows={spec} className="mt-6" listClassName="mt-3" />
    </Section>
  );
}
