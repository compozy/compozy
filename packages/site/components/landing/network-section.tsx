import Link from "next/link";
import { ArrowUpRight } from "lucide-react";
import { NetworkProtocolVisual } from "./network-protocol-visual";
import { LandingCodeBlock } from "./primitives/code-block";
import { FeatureCard } from "./primitives/feature-card";
import { NETWORK_KIND_COUNT } from "./primitives/network-kinds";
import { SectionFrame } from "./primitives/section-frame";
import { SectionHeader } from "./primitives/section-header";

const NETWORK_CODE = `agh network status
agh network peers builders
agh network directs resolve \\
  --session <session-id> \\
  --channel builders \\
  --peer reviewer.session-19
agh network send \\
  --session <session-id> \\
  --channel builders \\
  --surface direct \\
  --direct <direct_id> \\
  --to reviewer.session-19 \\
  --kind say \\
  --work work_review_pr_482 \\
  --body '{"text":"Review PR #482","intent":"request"}'
agh network inbox --session <session-id>`;

export function NetworkSection() {
  return (
    <SectionFrame background="deep" padY="xl" ariaLabel="agh-network/v0 protocol">
      <SectionHeader
        align="center"
        eyebrow="AGH Network , the differentiator"
        size="lg"
        title={
          <>
            <span className="font-mono text-accent-glyph tracking-tight text-accent">
              agh-network/v0
            </span>{" "}
            , implemented in the alpha runtime.
          </>
        }
        description={
          <>
            {NETWORK_KIND_COUNT} message kinds with commit-first delivery:{" "}
            <code className="font-mono text-accent">greet</code>,{" "}
            <code className="font-mono text-accent">whois</code>,{" "}
            <code className="font-mono text-accent">say</code>,{" "}
            <code className="font-mono text-accent">capability</code>,{" "}
            <code className="font-mono text-accent">receipt</code>,{" "}
            <code className="font-mono text-accent">trace</code>. Conversation lives in two surfaces
            , public <code className="font-mono text-accent">surface:&quot;thread&quot;</code> and
            restricted <code className="font-mono text-accent">surface:&quot;direct&quot;</code>.
            Your agent discovers a peer, opens or joins the right container, and tracks
            lifecycle-bearing work with an explicit{" "}
            <code className="font-mono text-accent">work_id</code>.
          </>
        }
      />

      <div className="mt-12">
        <NetworkProtocolVisual />
      </div>

      <div className="mt-12 grid gap-4 md:grid-cols-3">
        <FeatureCard
          eyebrow="CLI surface"
          title="Implemented commands"
          description={
            <>
              <code className="font-mono text-fg">
                agh network status | peers | channels | threads | directs | work | send | inbox
              </code>{" "}
              are implemented runtime commands, not narrative-only examples.
            </>
          }
          cite={{ href: "/runtime/guides/coordinate-agents-over-network", label: "Network guide" }}
        />
        <FeatureCard
          eyebrow="Delivery"
          title="Commit first, dispatch in-process"
          description="Accepted messages and recipient dispositions become durable together. Eligible local recipients are notified only after commit."
          cite={{ href: "/protocol/overview", label: "Protocol overview" }}
        />
        <FeatureCard
          eyebrow="Auditable"
          title="Receipts are first-class"
          description="Every delegation returns a receipt with status and trace IDs. Every message is persisted to the audit log."
          cite={{ href: "/protocol/delivery", label: "Delivery semantics" }}
        />
      </div>

      <div className="mt-10 grid gap-8 lg:grid-cols-[1fr_minmax(0,480px)] lg:items-center">
        <div className="max-w-[60ch] text-sm leading-relaxed text-muted">
          <p>
            AGH Network separates durable conversation from model activation. Local executions stay
            disconnected by default; explicitly Live executions can coordinate inside finite wake,
            depth, token, and wall-time bounds while tasks remain the authority for executable work.
          </p>
          <Link
            href="/protocol"
            className="mt-5 inline-flex items-center gap-1.5 text-sm font-medium text-accent transition-colors hover:text-accent-hover"
          >
            Read the full agh-network/v0 spec
            <ArrowUpRight aria-hidden className="size-4" />
          </Link>
        </div>
        <LandingCodeBlock code={NETWORK_CODE} caption="agh network" shell />
      </div>
    </SectionFrame>
  );
}
