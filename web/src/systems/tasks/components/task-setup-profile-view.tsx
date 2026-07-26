import type { ReactNode } from "react";

import { Pill, PillGroup, PropertyRow, Section, type PillGroupItem } from "@agh/ui";

import type { TaskExecutionProfile } from "../types";

function SetupGroup({ label, children }: { label: string; children: ReactNode }) {
  return (
    <Section className="mb-5" label={label}>
      {children}
    </Section>
  );
}

function SetupRow({ label, children }: { label: string; children: ReactNode }) {
  return <PropertyRow label={label}>{children}</PropertyRow>;
}

function Chips({ values }: { values: readonly string[] }) {
  return values.map(value => (
    <Pill key={value} tone="neutral">
      {value}
    </Pill>
  ));
}

type RoutingPolicy = Partial<
  Record<
    | "allowed_agent_names"
    | "preferred_agent_names"
    | "allowed_channel_ids"
    | "preferred_channel_ids"
    | "allowed_peer_ids"
    | "preferred_peer_ids"
    | "required_capabilities"
    | "preferred_capabilities",
    string[]
  >
>;

const ROUTING_ROWS: ReadonlyArray<{ key: keyof RoutingPolicy; label: string }> = [
  { key: "allowed_agent_names", label: "Allowed agents" },
  { key: "preferred_agent_names", label: "Preferred agents" },
  { key: "allowed_channel_ids", label: "Allowed channels" },
  { key: "preferred_channel_ids", label: "Preferred channels" },
  { key: "allowed_peer_ids", label: "Allowed peers" },
  { key: "preferred_peer_ids", label: "Preferred peers" },
  { key: "required_capabilities", label: "Required capabilities" },
  { key: "preferred_capabilities", label: "Preferred capabilities" },
];

function hasRoutingSelectors(policy: RoutingPolicy): boolean {
  return ROUTING_ROWS.some(({ key }) => (policy[key]?.length ?? 0) > 0);
}

function RoutingRows({ policy }: { policy: RoutingPolicy }) {
  return ROUTING_ROWS.map(({ key, label }) => {
    const values = policy[key];
    return values && values.length > 0 ? (
      <SetupRow key={key} label={label}>
        <Chips values={values} />
      </SetupRow>
    ) : null;
  });
}

function runtimeLabel(provider?: string, model?: string): string | null {
  if (provider && model) return `${provider} · ${model}`;
  return provider ?? model ?? null;
}

const WORKER_MODE_ITEMS: ReadonlyArray<PillGroupItem<"inherit" | "select">> = [
  { value: "inherit", label: "Inherit", disabled: true },
  { value: "select", label: "Select", disabled: true },
];

/** Read-only, lossless projection of the daemon execution-profile contract. */
export function TaskSetupProfileView({ profile }: { profile: TaskExecutionProfile }) {
  const worker = profile.worker;
  const coordinator = profile.coordinator;
  const sandbox = profile.sandbox;
  const review = profile.review;
  const participants = profile.participants;
  const workerRuntime = runtimeLabel(worker.provider, worker.model);
  const coordinatorRuntime = runtimeLabel(coordinator.provider, coordinator.model);
  const reviewRuntime = runtimeLabel(review.provider, review.model);
  const reviewConfigured = Boolean(
    review.agent_name || reviewRuntime || hasRoutingSelectors(review)
  );
  const participantsConfigured = hasRoutingSelectors(participants);
  const network = profile.network_participation;

  return (
    <>
      <SetupGroup label="Worker">
        <SetupRow label="Mode">
          <PillGroup
            aria-label="Worker mode"
            items={WORKER_MODE_ITEMS}
            onChange={() => undefined}
            size="sm"
            value={worker.mode}
          />
        </SetupRow>
        {worker.agent_name ? (
          <SetupRow label="Agent">
            <Pill tone="neutral">{worker.agent_name}</Pill>
          </SetupRow>
        ) : null}
        {workerRuntime ? (
          <SetupRow label="Runtime">
            <span className="font-mono text-eyebrow text-fg">{workerRuntime}</span>
          </SetupRow>
        ) : null}
        <RoutingRows policy={worker} />
      </SetupGroup>

      <SetupGroup label="Coordinator">
        <SetupRow label="Mode">
          <span className="capitalize">{coordinator.mode}</span>
        </SetupRow>
        {coordinator.agent_name ? (
          <SetupRow label="Agent">
            <Pill tone="neutral">{coordinator.agent_name}</Pill>
          </SetupRow>
        ) : null}
        {coordinatorRuntime ? (
          <SetupRow label="Runtime">
            <span className="font-mono text-eyebrow text-fg">{coordinatorRuntime}</span>
          </SetupRow>
        ) : null}
        {coordinator.guidance ? (
          <SetupRow label="Guidance">
            <span className="text-small-body leading-relaxed text-muted">
              {coordinator.guidance}
            </span>
          </SetupRow>
        ) : null}
      </SetupGroup>

      <SetupGroup label="Review">
        {reviewConfigured ? (
          <>
            {review.agent_name ? (
              <SetupRow label="Agent">
                <Pill tone="neutral">{review.agent_name}</Pill>
              </SetupRow>
            ) : null}
            {reviewRuntime ? (
              <SetupRow label="Runtime">
                <span className="font-mono text-eyebrow text-muted">{reviewRuntime}</span>
              </SetupRow>
            ) : null}
            <RoutingRows policy={review} />
          </>
        ) : (
          <SetupRow label="Policy">
            <span className="text-muted">Off</span>
          </SetupRow>
        )}
      </SetupGroup>

      <SetupGroup label="Sandbox & participants">
        <SetupRow label="Sandbox">
          {sandbox.mode === "ref" && sandbox.sandbox_ref ? (
            <Pill tone="neutral">{sandbox.sandbox_ref}</Pill>
          ) : (
            <span className="capitalize text-muted">{sandbox.mode}</span>
          )}
        </SetupRow>
        <SetupRow label="Runtime evidence">
          <span className="capitalize text-muted">{profile.runtime.mode}</span>
        </SetupRow>
        <SetupRow label="Network">
          <span className="text-muted">
            {network
              ? `${network.mode}${"channel_strategy" in network ? ` · ${network.channel_strategy}` : ""}`
              : "Off"}
          </span>
        </SetupRow>
        {participantsConfigured ? (
          <RoutingRows policy={participants} />
        ) : (
          <SetupRow label="Participants">
            <span className="text-muted">No custom routing policy</span>
          </SetupRow>
        )}
      </SetupGroup>
    </>
  );
}
