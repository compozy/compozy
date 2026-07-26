import Image from "next/image";
import { ArrowUpRight } from "lucide-react";
import Link from "next/link";
import { LandingCodeBlock } from "./primitives/code-block";
import { SectionFrame } from "./primitives/section-frame";
import { SectionHeader } from "./primitives/section-header";
import { Eyebrow } from "@agh/ui";

const AUTONOMY_CODE = `agh task create
agh task list --status queued
agh task next --wait                # claimed by an idle agent
agh task heartbeat <run-id>         # held by claim_token
agh task complete <run-id>`;

export function AutonomyKernelSection() {
  return (
    <SectionFrame
      background="deep"
      padY="lg"
      className="border-b border-line"
      ariaLabel="AGH autonomy kernel"
    >
      <SectionHeader
        align="start"
        eyebrow="Autonomy"
        size="lg"
        title="A real autonomy kernel, not a fork-and-pray loop."
        description="AGH owns the loop. Tasks claim runs atomically through ClaimNextRun, hold a lease they must heartbeat, and release back to the queue if they crash. One queue. Shared between humans and agents. Claim tokens never logged in raw form."
      />

      {/* Wide landscape storyboard — three-act lifecycle: ONE QUEUE → TOKEN-FENCED → LEASE RECOVERY. */}
      <figure className="mt-12">
        <Image
          src="/images/runtime/autonomy-overview-storyboard-v1.png"
          alt="AGH autonomy storyboard, task_runs queue, an agent claiming a run with a claim_token and heartbeat, and lease recovery on daemon restart."
          width={1440}
          height={760}
          decoding="async"
          sizes="100vw"
          quality={90}
          className="block select-none opacity-95"
        />
      </figure>

      {/* Asymmetric 2-col below — large narrative + code, narrow value list. No 3-col card grid. */}
      <div className="mt-12 grid min-w-0 gap-10 lg:grid-cols-[minmax(0,1.4fr)_minmax(0,1fr)] lg:items-start lg:gap-12">
        <div className="flex min-w-0 flex-col gap-6">
          <div className="min-w-0 rounded-diagram border border-line bg-canvas-soft p-5 sm:p-7">
            <Eyebrow className="text-accent">Token-fenced ownership</Eyebrow>
            <h3 className="mt-3 font-display text-2xl leading-tight tracking-tight text-fg">
              No double-execution, ever.
            </h3>
            <p className="mt-3 max-w-[60ch] text-sm leading-relaxed text-muted">
              Only the agent holding the claim token can heartbeat or complete a run. Sessions
              cannot reach into runs they don&apos;t own. Tokens are hashed before they touch the
              event ledger; raw values never leave the daemon.
            </p>
          </div>
          <LandingCodeBlock code={AUTONOMY_CODE} caption="agh task" shell />
        </div>

        <ul className="flex min-w-0 flex-col divide-y divide-line border-y border-line">
          <li className="py-6">
            <Eyebrow className="text-accent">Lease recovery</Eyebrow>
            <h4 className="mt-2 text-base font-medium leading-snug text-fg">
              Daemon crashes don&apos;t orphan work.
            </h4>
            <p className="mt-2 text-sm leading-relaxed text-muted">
              Leases expire on a TTL. Runs re-enter the queue automatically. The next idle agent
              picks them up.
            </p>
          </li>
          <li className="py-6">
            <Eyebrow className="text-accent">One shared queue</Eyebrow>
            <h4 className="mt-2 text-base font-medium leading-snug text-fg">
              Operators and agents hit task_runs.
            </h4>
            <p className="mt-2 text-sm leading-relaxed text-muted">
              <code className="font-mono text-fg">agh task create</code> (you) and the coordinator
              agent (them) write to the same SQLite table. Same primitives, same audit trail.
            </p>
          </li>
          <li className="py-6">
            <Eyebrow className="text-accent">Permission narrowing</Eyebrow>
            <h4 className="mt-2 text-base font-medium leading-snug text-fg">
              Children cannot widen parents.
            </h4>
            <p className="mt-2 text-sm leading-relaxed text-muted">
              Lineage, TTLs, and permission scopes are part of the spawn contract; enforced in code,
              not in the prompt.
            </p>
          </li>
        </ul>
      </div>

      <div className="mt-10">
        <Link
          href="/runtime"
          className="inline-flex items-center gap-1.5 text-sm font-medium text-accent transition-colors hover:text-accent-hover"
        >
          Read the autonomy kernel guide
          <ArrowUpRight aria-hidden className="size-4" />
        </Link>
      </div>
    </SectionFrame>
  );
}
