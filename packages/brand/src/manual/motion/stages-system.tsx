// Signature stages built from atmosphere, runs and text: converge, light sweep, lane growth,
// state change, log line and counter. Each renders its final frame; demos.ts plays it.
const foot = "absolute bottom-3 left-4 m-0 mono-label text-[11px] text-muted-foreground";

// converge: scattered offsets (px) per particle, deterministic; final position = on the stream line
let seed = 11;
const rnd = () => (seed = (seed * 16807) % 2147483647) / 2147483647;
const pts = Array.from({ length: 34 }, (_, i) => ({
  x: 24 + (i / 33) * 56,
  dx: -(60 + rnd() * 160),
  dy: (rnd() - 0.5) * 150,
}));

export function ConvergeStage() {
  return (
    <>
      <div
        data-stream
        className="absolute inset-x-[24%] top-1/2 h-px bg-[linear-gradient(90deg,transparent,var(--color-brand-volt),var(--color-brand-cyan))]"
      />
      {pts.map(p => (
        <span
          key={p.x}
          data-pt
          data-dx={p.dx}
          data-dy={p.dy}
          className="absolute block size-[4px] rounded-full bg-(image:--gradient-live)"
          style={{ left: `${p.x}%`, top: "calc(50% - 2px)" }}
        />
      ))}
      <p className={foot}>34 tasks → one stream</p>
    </>
  );
}

export function SweepStage() {
  return (
    <>
      <div className="absolute inset-0 bg-[repeating-linear-gradient(180deg,color-mix(in_oklab,var(--color-fg-strong)_12%,transparent)_0_1px,transparent_1px_9px)]" />
      <div
        data-lit
        className="absolute inset-0 bg-[repeating-linear-gradient(180deg,var(--color-brand-cyan)_0_1px,transparent_1px_9px)] [mask-image:linear-gradient(90deg,transparent,black_50%,transparent)] [mask-position:50%_0] [mask-size:40%_100%] [mask-repeat:no-repeat]"
      />
    </>
  );
}

export function LaneGrowStage() {
  return (
    <>
      <div className="absolute inset-[14%_10%_22%_26%]">
        {[0.28, 0.6, 0.38].map((w, i) => (
          <div
            key={w}
            className="absolute h-[18%] rounded-[3px] bg-surface-2"
            style={{
              top: `${6 + i * 32}%`,
              left: `${i === 1 ? 0 : 12 * i}%`,
              width: `${w * 100}%`,
            }}
          />
        ))}
        <div
          data-bar
          className="absolute top-[70%] left-0 h-[18%] w-full origin-left rounded-[3px] bg-(image:--gradient-live)"
        />
        <div className="absolute -top-[8%] right-0 h-[112%] w-px bg-(--color-brand-cyan)" />
        <span className="absolute -top-[16%] right-0 font-mono text-[10px] text-(--color-brand-cyan)">
          now
        </span>
      </div>
      <span className="absolute top-[76%] left-[7%] font-mono text-[10px] text-muted-foreground">
        codex
      </span>
      <p data-label className={foot}>
        running · 4 m
      </p>
    </>
  );
}

export function StateChangeStage() {
  const pill = "absolute inset-0 grid place-items-center rounded-full border border-line-strong";
  return (
    <>
      <div className="absolute inset-[16%_8%_auto] flex items-center justify-between rounded-[10px] border border-line bg-canvas px-4 py-3 font-mono text-[12.5px]">
        <span className="flex items-center gap-3">
          <span className="relative block size-[10px]">
            <span
              data-s="idle"
              className="absolute inset-0 rounded-full border border-muted-foreground opacity-0"
            />
            <span
              data-s="running"
              className="absolute inset-0 rounded-full bg-(--color-brand-cyan) opacity-0"
            />
            <span data-s="done" className="absolute inset-0 rounded-full bg-success" />
          </span>
          review-pr · claude-code
        </span>
        <span className="relative block h-7 w-[112px]">
          <span data-p="idle" className={`${pill} text-muted-foreground opacity-0`}>
            ○ idle
          </span>
          <span data-p="running" className={`${pill} text-(--color-brand-cyan) opacity-0`}>
            ● running
          </span>
          <span data-p="done" className={`${pill} text-success`}>
            ✓ done
          </span>
        </span>
      </div>
      <p className={foot}>session · idle → running → done</p>
    </>
  );
}

const log = [
  "12:04:11  read  src/auth/session.ts",
  "12:04:19  edit  src/auth/session.ts +14 −3",
  "12:04:31  run   bun test auth · 42 passed",
  "12:04:38  commit  fix: refresh token on expiry",
];

export function LogLineStage() {
  return (
    <div className="absolute inset-[10%_7%] rounded-[10px] border border-line bg-sunken p-4 font-mono text-[12px] leading-[1.8] whitespace-pre">
      {log.map((l, i) => (
        <p
          key={l}
          data-ll
          className={
            i === log.length - 1 ? "m-0 text-(--color-brand-cyan)" : "m-0 text-muted-foreground"
          }
        >
          {l}
        </p>
      ))}
    </div>
  );
}

const counters: [value: string, label: string][] = [
  ["12", "running"],
  ["03", "need you"],
  ["41", "done today"],
];

export function CounterStage() {
  return (
    <div className="absolute inset-0 grid grid-cols-3 content-center gap-2 px-[7%]">
      {counters.map(([v, l]) => (
        <div key={l}>
          <p className="m-0 flex h-[48px] overflow-hidden font-display text-[44px] leading-[48px] tracking-[-0.04em]">
            {v.split("").map((d, ci) => (
              <span key={ci} className="block h-[48px] overflow-hidden">
                <span
                  data-col={d}
                  className="block"
                  style={{ transform: `translateY(-${Number(d) * 10}%)` }}
                >
                  {Array.from({ length: 10 }, (_, i) => (
                    <span key={i} className="block h-[48px]">
                      {i}
                    </span>
                  ))}
                </span>
              </span>
            ))}
          </p>
          <p className="m-0 mt-2 mono-label text-[10.5px] text-muted-foreground">{l}</p>
        </div>
      ))}
    </div>
  );
}
