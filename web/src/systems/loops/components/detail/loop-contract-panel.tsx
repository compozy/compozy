import { Bot, ScrollText, Terminal, UserCheck, type LucideIcon } from "lucide-react";

import { Eyebrow } from "@compozy/ui";

import type { LoopContract, LoopContractVerification } from "../../types";
import { LoopSection } from "../loop-section";

const CRITERION_TYPE_ICONS = {
  "agent-judge": Bot,
  command: Terminal,
  human: UserCheck,
} as const satisfies Record<string, LucideIcon>;

function criterionTypeIcon(type: string): LucideIcon | undefined {
  if (Object.hasOwn(CRITERION_TYPE_ICONS, type)) {
    return CRITERION_TYPE_ICONS[type as keyof typeof CRITERION_TYPE_ICONS];
  }
  return undefined;
}

interface LoopContractPanelProps {
  contract: LoopContract;
  concurrency?: string;
}

/** Human-readable method line for one verification criterion. */
function verificationMethod(criterion: LoopContractVerification): string {
  const parts: string[] = [];
  if (criterion.check) parts.push(criterion.check);
  if (criterion.expect) parts.push(`expect ${criterion.expect}`);
  if (criterion.agent) parts.push(`agent ${criterion.agent}`);
  if (criterion.tool) parts.push(`tool ${criterion.tool}`);
  if (criterion.rubric) parts.push(criterion.rubric);
  if (criterion.prompt) parts.push(criterion.prompt);
  return parts.join(" · ");
}

/**
 * The Loop contract: goal, definition of done, typed verification criteria, the
 * terminal outcomes (color = state), and the concurrency policy (design §4.2).
 */
export function LoopContractPanel({ contract, concurrency }: LoopContractPanelProps) {
  return (
    <LoopSection
      data-testid="loop-contract"
      icon={<ScrollText aria-hidden="true" />}
      title="Contract"
    >
      <div className="flex flex-col rounded-lg border border-line bg-canvas-soft">
        <LoopContractRows concurrency={concurrency} contract={contract} />
      </div>
    </LoopSection>
  );
}

/**
 * The contract rows without a shell, so a caller that already owns the card — the
 * run form's folded rail — composes them instead of nesting a second panel.
 */
export function LoopContractRows({ contract, concurrency }: LoopContractPanelProps) {
  const verification = contract.verification ?? [];
  const terminalStates = contract.terminal_states ?? [];
  return (
    <>
      <ContractRow label="Goal">
        <p className="text-small-body leading-relaxed text-fg">{contract.goal}</p>
      </ContractRow>
      <ContractRow label="Definition of done">
        <p className="text-small-body leading-relaxed text-fg">{contract.definition_of_done}</p>
      </ContractRow>
      {verification.length > 0 ? (
        <ContractRow label="Gate criteria">
          <div className="mt-2 flex flex-col gap-2">
            {verification.map(criterion => {
              const TypeIcon = criterionTypeIcon(criterion.type);
              return (
                <div key={criterion.id} className="flex items-start gap-2.5">
                  <span className="mt-0.5 inline-flex shrink-0 items-center gap-1.5 text-small-body text-muted">
                    {TypeIcon ? <TypeIcon aria-hidden="true" className="size-3.5" /> : null}
                    {criterion.type}
                  </span>
                  <div className="min-w-0">
                    <b className="text-small-body font-medium text-fg-strong">{criterion.id}</b>
                    <div className="mt-0.5 font-mono text-mono-id text-subtle">
                      {verificationMethod(criterion) || "—"}
                    </div>
                  </div>
                </div>
              );
            })}
          </div>
        </ContractRow>
      ) : null}
      {terminalStates.length > 0 ? (
        <ContractRow label="Possible endings">
          {/* A constant enumeration, not live state: plain mono text, never status chips. */}
          <p className="font-mono text-mono-id text-subtle" data-testid="loop-terminal-endings">
            {terminalStates.join(" · ")}
          </p>
        </ContractRow>
      ) : null}
      {concurrency ? (
        <ContractRow label="Concurrency">
          <p className="text-small-body leading-relaxed text-fg">
            <code className="rounded-xs border border-line-soft bg-input-fill px-1.5 font-mono text-xs">
              {concurrency}
            </code>
          </p>
        </ContractRow>
      ) : null}
    </>
  );
}

interface ContractRowProps {
  label: string;
  children: React.ReactNode;
}

function ContractRow({ label, children }: ContractRowProps) {
  return (
    <div className="border-t border-line-soft px-4 py-3.5 first:border-t-0">
      <Eyebrow className="mb-1.5 text-faint">{label}</Eyebrow>
      {children}
    </div>
  );
}
