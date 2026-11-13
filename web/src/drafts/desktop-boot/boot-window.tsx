import type { CSSProperties } from "react";

import { Logo } from "@compozy/ui";

// The brand's `horizon` Atmosphere visual; the boot page would ship its own self-hosted copy.
import horizon from "../../../../packages/brand/public/brand/visuals/horizon-1280.jpg";
import { BootRunner } from "@/drafts/desktop-boot/boot-runner";
import { useElapsed, useWork } from "@/drafts/desktop-boot/hooks/use-boot-live";
import type { BootAction, BootScene, BootStep, BootWork } from "@/drafts/desktop-boot/scenes";

const NUMBER = new Intl.NumberFormat("en-US");
const FIELD = { cols: 52, rows: 18 };
const PULSE = { cols: 24, rows: 1 };

const ACTION_LABEL: Record<BootAction, string> = {
  retry: "Try again",
  logs: "Open logs",
  quit: "Quit",
  diagnose: "Diagnostics",
};

const STATUS_WORD: Record<string, string> = {
  product: "Ready",
  disconnected: "Reconnecting",
  skew: "Needs you",
  error: "Failed",
};

/** `m:ss` under an hour, `h:mm:ss` above: a clock reads as "still counting" at a glance. */
function clock(seconds: number) {
  const hours = Math.floor(seconds / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  const rest = String(seconds % 60).padStart(2, "0");
  return hours > 0 ? `${hours}:${String(minutes).padStart(2, "0")}:${rest}` : `${minutes}:${rest}`;
}

function estimate(work: BootWork) {
  if (work.moving === false) return "Waiting for the runtime";
  if (!work.rate) return "";
  const minutes = (work.total - work.completed) / work.rate;
  return minutes < 1 ? "Less than a minute left" : `About ${Math.round(minutes)} min left`;
}

/**
 * The brand's point field. With a known total it fills column by column in grey and only the
 * column being worked on is lit; without one, a lit band keeps crossing it.
 */
function Points({
  cols,
  rows,
  fraction,
  className,
}: {
  cols: number;
  rows: number;
  fraction: number | null;
  className?: string;
}) {
  const total = cols * rows;
  const done = fraction === null ? 0 : Math.floor(fraction * total);
  const liveColumn = Math.floor(done / rows);
  return (
    <div
      className={`boot__points ${className ?? ""}`}
      data-mode={fraction === null ? "sweep" : "fill"}
      style={{ "--rows": rows, "--cols": cols } as CSSProperties}
      aria-hidden="true"
    >
      {Array.from({ length: total }, (_, index) => {
        const column = Math.floor(index / rows);
        const on =
          fraction === null || fraction >= 1
            ? fraction === null
              ? undefined
              : "done"
            : index < done
              ? column === liveColumn
                ? "live"
                : "done"
              : column === liveColumn && index === done
                ? "live"
                : undefined;
        return (
          <span
            key={index}
            className="boot__point"
            data-on={on}
            style={{ "--c": column } as CSSProperties}
          />
        );
      })}
    </div>
  );
}

function Versions({ versions }: { versions: NonNullable<BootScene["versions"]> }) {
  return (
    <div className="boot__versions">
      <span className="boot__version-pill">
        {versions.from ? (
          <>
            {versions.from}
            <svg viewBox="0 0 10 10" fill="none" aria-hidden="true">
              <path
                d="M1.5 5h7M6 2.5L8.5 5 6 7.5"
                stroke="currentColor"
                strokeWidth="1.3"
                strokeLinecap="round"
                strokeLinejoin="round"
              />
            </svg>
          </>
        ) : null}
        <b>{versions.to}</b>
      </span>
      <button className="boot__link" type="button">
        What’s new
      </button>
    </div>
  );
}

function ActionButton({ action, size }: { action: BootAction; size?: "sm" }) {
  return (
    <button
      className="boot__btn"
      type="button"
      data-size={size}
      data-variant={action === "retry" ? "primary" : undefined}
    >
      {ACTION_LABEL[action]}
    </button>
  );
}

function Steps({ steps }: { steps: BootStep[] }) {
  return (
    <ol className="boot__steps" aria-label="Steps">
      {steps.map(step => (
        <li
          key={step.id}
          className="boot__step"
          data-status={step.status}
          aria-current={step.status === "current" ? "step" : undefined}
        >
          <span>
            {step.label}
            <span className="boot__step-meta">{step.meta ?? " "}</span>
          </span>
        </li>
      ))}
    </ol>
  );
}

function Work({ scene, work }: { scene: BootScene; work: BootWork | undefined }) {
  const current = scene.steps?.find(step => step.status === "current");
  if (!current) return null;
  const fraction = work ? work.completed / work.total : null;
  const percent = fraction === null ? null : Math.floor(fraction * 100);
  return (
    <section className="boot__work" aria-live="off">
      <div className="boot__work-head">
        <h2 className="boot__work-name">{scene.workName ?? current.label}</h2>
        {work ? (
          <span className="boot__work-count">
            <b>{NUMBER.format(work.completed)}</b> of {NUMBER.format(work.total)} {work.unit}
          </span>
        ) : null}
      </div>
      <div
        role="progressbar"
        aria-label={scene.workName ?? current.label}
        aria-valuemin={0}
        aria-valuemax={100}
        aria-valuenow={percent ?? undefined}
        aria-valuetext={
          work
            ? `${NUMBER.format(work.completed)} of ${NUMBER.format(work.total)} ${work.unit}`
            : undefined
        }
      >
        <Points className="boot__field" cols={FIELD.cols} rows={FIELD.rows} fraction={fraction} />
      </div>
      <div className="boot__work-foot">
        <span>{work ? estimate(work) : scene.hint}</span>
        {percent === null ? null : <span>{percent}%</span>}
      </div>
      {scene.notice ? (
        <div className="boot__notice" data-tone={scene.notice.tone} role="status">
          <p>
            <strong>{scene.notice.title}</strong>
            {scene.notice.text}
          </p>
          <div className="boot__notice-actions">
            {scene.notice.actions.map(action => (
              <ActionButton key={action} action={action} size="sm" />
            ))}
          </div>
        </div>
      ) : null}
    </section>
  );
}

function Diagnostics({ scene }: { scene: BootScene }) {
  if (!scene.diagnostics) return null;
  const rows = scene.diagnostics.open ? scene.diagnostics.rows : scene.diagnostics.rows.slice(0, 2);
  return (
    <>
      <h2 className="boot__panel-title">Technical details</h2>
      <dl className="boot__diagnostics">
        {rows.map(([term, value]) => (
          <div key={term}>
            <dt>{term}</dt>
            <dd>{value}</dd>
          </div>
        ))}
      </dl>
      <div className="boot__panel-foot">
        {scene.diagnostics.open ? (
          <>
            <button className="boot__btn" type="button" data-size="sm">
              Copy diagnostics
            </button>
            <button className="boot__btn" type="button" data-size="sm">
              Export diagnostics
            </button>
          </>
        ) : (
          <button className="boot__btn" type="button" data-size="sm">
            Load diagnostics
          </button>
        )}
        {scene.result ? (
          <p className="boot__result" role="status">
            {scene.result}
          </p>
        ) : null}
      </div>
    </>
  );
}

function LeadMessage({ scene }: { scene: BootScene }) {
  return (
    <>
      {scene.detail ? <p className="boot__detail">{scene.detail}</p> : null}
      {scene.error ? (
        <>
          <p className="boot__detail" role="alert">
            {scene.error.message}
          </p>
          <p className="boot__error-code">{scene.error.code}</p>
        </>
      ) : null}
    </>
  );
}

/**
 * The one mono line. Only the word and the activity are live: the clock ticks every second, so it
 * stays readable outside the status region, and it is total boot time, never time without progress.
 */
function StatusLine({ scene, alive }: { scene: BootScene; alive: boolean }) {
  const elapsed = useElapsed(scene.elapsed);
  const word = STATUS_WORD[scene.state] ?? (scene.work?.moving === false ? "Waiting" : "Working");
  return (
    <p className="boot__status">
      <span className="boot__status-mark" aria-hidden="true" />
      <span className="boot__status-word" role="status">
        {word}
      </span>
      {alive && elapsed !== null ? <span aria-label="Boot time">{clock(elapsed)}</span> : null}
      {scene.activity ? (
        <span className="boot__status-activity" role="status">
          {scene.activity}
        </span>
      ) : null}
    </p>
  );
}

/** The boot page body: what `desktop/pages/boot.html` renders for one boot snapshot. */
export function BootWindow({ scene }: { scene: BootScene }) {
  const work = useWork(scene.work);
  const alive = scene.tone === "progress" || scene.state === "disconnected";
  const journey = scene.layout === "journey";
  return (
    <main
      className="boot"
      data-tone={scene.tone}
      data-layout={scene.layout}
      data-state={scene.state}
      data-alive={alive}
    >
      <img className="boot__atmosphere" src={horizon} alt="" />
      <div className="boot__lead">
        <Logo variant="symbol" decorative className="boot__symbol" />
        <h1 className="boot__title" aria-live="polite">
          {scene.title}
        </h1>
        {scene.versions ? <Versions versions={scene.versions} /> : null}
        <LeadMessage scene={scene} />
        {!journey && alive ? (
          <Points className="boot__pulse" cols={PULSE.cols} rows={PULSE.rows} fraction={null} />
        ) : null}
        {scene.actions ? (
          <div className="boot__actions" role="group" aria-label="Available actions">
            {scene.actions.map(action => (
              <ActionButton key={action} action={action} />
            ))}
          </div>
        ) : null}
        {journey ? <BootRunner autoplay={alive} /> : null}
        <StatusLine scene={scene} alive={alive} />
      </div>
      {journey ? (
        <div className="boot__panel">
          {scene.steps ? <Steps steps={scene.steps} /> : null}
          <Work scene={scene} work={work} />
          <Diagnostics scene={scene} />
        </div>
      ) : null}
    </main>
  );
}
