import { Check, Network, Users } from "lucide-react";
import type { FormEvent } from "react";

import {
  Dialog,
  DialogContent,
  dialogShellClass,
  Empty,
  EntityDialogBody,
  EntityDialogFooter,
  EntityDialogHeader,
  Eyebrow,
  Field,
  FieldContent,
  FieldDescription,
  FieldLabel,
  FormSection,
  Input,
  Pill,
  Textarea,
} from "@agh/ui";
import { AgentCommandMultiSelect, type AgentPayload } from "@/systems/agent";

import type { NetworkCreateChannelDraft, NetworkFanoutPolicy } from "../types";
import { ChannelFanoutCards } from "./channel-fanout-cards";

/**
 * `coordinator` needs a `coordinator_peer_id`, and channel peers are only minted
 * when the member sessions are provisioned by the create call itself. The card
 * stays visible so the policy set reads completely, but it cannot be written here.
 */
const CREATE_UNAVAILABLE_POLICIES: ReadonlyArray<NetworkFanoutPolicy> = ["coordinator"];

interface NetworkCreateChannelDialogProps {
  agents: AgentPayload[];
  canSubmit: boolean;
  draft: NetworkCreateChannelDraft;
  isSubmitting: boolean;
  onChannelNameChange: (value: string) => void;
  onOpenChange: (open: boolean) => void;
  onPurposeChange: (value: string) => void;
  onAgentSelectionChange: (agentNames: string[]) => void;
  onFanoutPolicyChange: (policy: NetworkFanoutPolicy) => void;
  onSubmit: () => void;
  open: boolean;
  workspaceName?: string | null;
}

export function NetworkCreateChannelDialog({
  agents,
  canSubmit,
  draft,
  isSubmitting,
  onChannelNameChange,
  onOpenChange,
  onPurposeChange,
  onAgentSelectionChange,
  onFanoutPolicyChange,
  onSubmit,
  open,
  workspaceName,
}: NetworkCreateChannelDialogProps) {
  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!canSubmit || isSubmitting) return;
    onSubmit();
  };

  return (
    <Dialog onOpenChange={onOpenChange} open={open}>
      <DialogContent
        className={`grid-rows-[auto_minmax(0,1fr)] ${dialogShellClass("sm")}`}
        data-testid="network-create-channel-dialog"
        showCloseButton={false}
        unframed
      >
        <EntityDialogHeader
          description={
            workspaceName
              ? `Spawn one new session per selected agent inside ${workspaceName}.`
              : "Create a materialized network channel by spawning one new session per selected agent."
          }
          eyebrow="Network · Channel"
          icon={Network}
          onClose={() => onOpenChange(false)}
          title="Create channel"
        />

        <form className="grid min-h-0 grid-rows-[minmax(0,1fr)_auto]" onSubmit={handleSubmit}>
          <EntityDialogBody className="flex flex-col">
            <FormSection
              description="Name and membership are fixed after creation."
              title="The channel"
            >
              <div className="flex flex-col gap-4">
                <Field>
                  <FieldContent>
                    <FieldLabel htmlFor="network-channel-name">
                      Channel name
                      <Eyebrow className="ml-1.5 text-accent-strong">required</Eyebrow>
                    </FieldLabel>
                    <FieldDescription>
                      Use lowercase letters, numbers, underscores, or hyphens; e.g. coord_core.
                    </FieldDescription>
                  </FieldContent>
                  <Input
                    className="font-mono"
                    data-testid="network-channel-name-input"
                    id="network-channel-name"
                    onChange={event => onChannelNameChange(event.target.value)}
                    placeholder="e.g. website_copy"
                    value={draft.channelName}
                  />
                </Field>

                <Field>
                  <FieldContent>
                    <FieldLabel htmlFor="network-channel-purpose">
                      Purpose
                      <Eyebrow className="ml-1.5 text-accent-strong">required</Eyebrow>
                    </FieldLabel>
                    <FieldDescription>
                      Peers use the purpose to decide whether to delegate here.
                    </FieldDescription>
                  </FieldContent>
                  <Textarea
                    aria-required="true"
                    className="min-h-24 border-line bg-canvas-soft p-3 text-small-body leading-6"
                    data-testid="network-channel-purpose-input"
                    id="network-channel-purpose"
                    onChange={event => onPurposeChange(event.target.value)}
                    placeholder="e.g. Coordinate release handoffs across coder and reviewer agents."
                    required
                    value={draft.purpose}
                  />
                </Field>
              </div>
            </FormSection>

            <FormSection
              description="Agents from the workspace catalog."
              rightLabel={
                <Pill mono data-testid="network-selected-agents-count">
                  {draft.selectedAgentNames.length} selected
                </Pill>
              }
              title="Members"
            >
              {agents.length === 0 ? (
                <Empty
                  description="This workspace has no local agents to invite."
                  icon={Users}
                  title="No agents available"
                />
              ) : (
                <AgentCommandMultiSelect
                  agents={agents}
                  itemTestId={agent => `network-agent-option-${agent.name}`}
                  onToggle={onAgentSelectionChange}
                  placeholder="Select agents"
                  triggerTestId="network-create-channel-agent-trigger"
                  value={draft.selectedAgentNames}
                />
              )}

              {!workspaceName ? (
                <p className="mt-2 text-xs leading-relaxed text-warning">
                  Select an active workspace before creating a channel.
                </p>
              ) : null}
            </FormSection>

            <FormSection
              description="The fanout policy routes incoming work."
              title="Who receives a message?"
            >
              <ChannelFanoutCards
                disabled={isSubmitting}
                onChange={onFanoutPolicyChange}
                testIdPrefix="network-create-channel"
                unavailable={CREATE_UNAVAILABLE_POLICIES}
                unavailableHint="Coordinator needs a live member peer, which exists only once the channel sessions start. Pick it in Delivery policy after creating the channel."
                value={draft.fanoutPolicy}
              />
            </FormSection>
          </EntityDialogBody>

          <EntityDialogFooter
            cancelTestId="network-create-channel-cancel"
            hint="Name and membership are fixed after creation — purpose and fanout stay editable."
            isSaving={isSubmitting}
            onCancel={() => onOpenChange(false)}
            primaryDisabled={!canSubmit}
            primaryIcon={Check}
            primaryLabel="Create channel"
            primaryTestId="network-create-channel-submit"
            primaryType="submit"
          />
        </form>
      </DialogContent>
    </Dialog>
  );
}
