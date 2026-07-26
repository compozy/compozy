import { Check, Network } from "lucide-react";
import { useState, type FormEvent } from "react";

import {
  CommandEmpty,
  CommandItem,
  CommandList,
  CommandSelect,
  CommandSelectGroup,
  CommandSelectShell,
  CommandSelectTrigger,
  Dialog,
  DialogContent,
  dialogShellClass,
  EntityDialogBody,
  EntityDialogFooter,
  EntityDialogHeader,
  Field,
  FieldContent,
  FieldDescription,
  FieldLabel,
  FormSection,
  ImmutableIdentity,
  Input,
} from "@agh/ui";

import { ChannelFanoutCards } from "../channel-fanout-cards";
import type { ChannelMember } from "../../hooks/use-channel-members";
import type {
  NetworkChannel,
  NetworkChannelSummary,
  NetworkChannelUpdateRequest,
  NetworkFanoutPolicy,
} from "../../types";

interface ChannelPolicyDialogProps {
  channel: NetworkChannelSummary;
  detail: NetworkChannel | null;
  members: ReadonlyArray<ChannelMember>;
  isSubmitting: boolean;
  onOpenChange: (open: boolean) => void;
  onSubmit: (data: NetworkChannelUpdateRequest) => Promise<void>;
  open: boolean;
}

interface ChannelPolicyDialogFormState {
  coordinatorPeerId: string;
  policy: NetworkFanoutPolicy;
  purpose: string;
  submitError: string | null;
}

function normalizeFanoutPolicy(value?: string | null): NetworkFanoutPolicy {
  if (value === "coordinator" || value === "all_members") {
    return value;
  }
  return "capability_match";
}

function currentCoordinator(channel: NetworkChannelSummary, detail: NetworkChannel | null): string {
  return detail?.coordinator_peer_id ?? channel.coordinator_peer_id ?? "";
}

function currentPurpose(channel: NetworkChannelSummary, detail: NetworkChannel | null): string {
  return detail?.purpose ?? channel.purpose ?? "";
}

function initialFormState(
  channel: NetworkChannelSummary,
  detail: NetworkChannel | null
): ChannelPolicyDialogFormState {
  return {
    coordinatorPeerId: currentCoordinator(channel, detail),
    policy: normalizeFanoutPolicy(detail?.fanout_policy ?? channel.fanout_policy),
    purpose: currentPurpose(channel, detail),
    submitError: null,
  };
}

function channelPolicyFormKey(
  channel: NetworkChannelSummary,
  detail: NetworkChannel | null,
  open: boolean
): string {
  return JSON.stringify([
    open ? "open" : "closed",
    channel.channel,
    currentPurpose(channel, detail),
    normalizeFanoutPolicy(detail?.fanout_policy ?? channel.fanout_policy),
    currentCoordinator(channel, detail),
  ]);
}

function memberLabel(members: ReadonlyArray<ChannelMember>, peerId: string): string {
  const match = members.find(member => member.peerId === peerId);
  return match?.displayName || match?.peerId || peerId;
}

function ChannelPolicyDialogForm({
  channel,
  detail,
  members,
  isSubmitting,
  onOpenChange,
  onSubmit,
}: Omit<ChannelPolicyDialogProps, "open">) {
  const [formState, setFormState] = useState(() => initialFormState(channel, detail));
  const [coordinatorOpen, setCoordinatorOpen] = useState(false);

  const setPurpose = (purpose: string) => {
    setFormState(current => ({ ...current, purpose }));
  };
  // Leaving `coordinator` clears the peer: the daemon rejects a coordinator id
  // under any other fanout policy (`ValidateNetworkChannelFanoutConfiguration`).
  const setPolicy = (policy: NetworkFanoutPolicy) => {
    setFormState(current => ({
      ...current,
      coordinatorPeerId: policy === "coordinator" ? current.coordinatorPeerId : "",
      policy,
    }));
  };

  const requiresCoordinator = formState.policy === "coordinator";
  const missingCoordinator = requiresCoordinator && formState.coordinatorPeerId.trim() === "";

  const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setFormState(current => ({ ...current, submitError: null }));
    const payload: NetworkChannelUpdateRequest = {
      purpose: formState.purpose.trim(),
      fanout_policy: formState.policy,
      coordinator_peer_id: formState.coordinatorPeerId.trim(),
    };
    try {
      await onSubmit(payload);
      onOpenChange(false);
    } catch (error) {
      const submitError = error instanceof Error ? error.message : "Couldn't save delivery policy.";
      setFormState(current => ({ ...current, submitError }));
    }
  };

  const memberNames = members.map(member => member.displayName || member.peerId);

  return (
    <form className="grid min-h-0 grid-rows-[minmax(0,1fr)_auto]" onSubmit={handleSubmit}>
      <EntityDialogBody className="flex flex-col">
        <ImmutableIdentity
          data-testid="channel-policy-identity"
          hint="UpdateNetworkChannelRequest cannot change the name or membership — recreate the channel to change them."
          rows={[
            { label: "Channel", value: channel.channel, mono: true },
            {
              label: "Members",
              value: memberNames.length > 0 ? memberNames.join(", ") : "No members yet",
            },
          ]}
        />

        <FormSection description="The mutable half of the channel." title="Purpose & fanout">
          <div className="flex flex-col gap-4">
            <Field>
              <FieldContent>
                <FieldLabel htmlFor="network-channel-policy-purpose">Purpose</FieldLabel>
                <FieldDescription>
                  Used by agents to decide whether this room is relevant.
                </FieldDescription>
              </FieldContent>
              <Input
                className="font-mono"
                data-testid="channel-policy-purpose"
                id="network-channel-policy-purpose"
                onChange={event => setPurpose(event.target.value)}
                value={formState.purpose}
              />
            </Field>

            <Field>
              <FieldContent>
                <FieldLabel>Who receives a message?</FieldLabel>
                <FieldDescription>
                  The fanout policy routes unaddressed thread traffic.
                </FieldDescription>
              </FieldContent>
              <ChannelFanoutCards
                disabled={isSubmitting}
                onChange={setPolicy}
                testIdPrefix="channel-policy"
                value={formState.policy}
              />
            </Field>

            {requiresCoordinator ? (
              <Field>
                <FieldContent>
                  <FieldLabel id="channel-policy-coordinator-label">Coordinator peer</FieldLabel>
                  <FieldDescription>Derived from the channel's live members.</FieldDescription>
                </FieldContent>
                <CommandSelect onOpenChange={setCoordinatorOpen} open={coordinatorOpen}>
                  <CommandSelectTrigger
                    aria-labelledby="channel-policy-coordinator-label"
                    data-testid="channel-policy-coordinator"
                    placeholder="Select coordinator"
                    selected={formState.coordinatorPeerId !== ""}
                  >
                    {formState.coordinatorPeerId
                      ? memberLabel(members, formState.coordinatorPeerId)
                      : null}
                  </CommandSelectTrigger>
                  <CommandSelectShell inputPlaceholder="Search members…">
                    <CommandList>
                      <CommandEmpty>No members match.</CommandEmpty>
                      <CommandSelectGroup>
                        {members.map(member => (
                          <CommandItem
                            data-testid={`channel-policy-coordinator-${member.peerId}`}
                            key={member.peerId}
                            onSelect={() => {
                              setFormState(current => ({
                                ...current,
                                coordinatorPeerId: member.peerId,
                              }));
                              setCoordinatorOpen(false);
                            }}
                            value={`${member.displayName ?? ""} ${member.peerId}`}
                          >
                            {member.displayName || member.peerId}
                          </CommandItem>
                        ))}
                      </CommandSelectGroup>
                    </CommandList>
                  </CommandSelectShell>
                </CommandSelect>
              </Field>
            ) : null}

            {formState.submitError ? (
              <p className="text-xs text-danger" role="alert">
                {formState.submitError}
              </p>
            ) : null}
          </div>
        </FormSection>
      </EntityDialogBody>

      <EntityDialogFooter
        cancelTestId="channel-policy-cancel"
        hint="Delivery changes apply to new unaddressed traffic."
        isSaving={isSubmitting}
        onCancel={() => onOpenChange(false)}
        primaryDisabled={missingCoordinator}
        primaryIcon={Check}
        primaryLabel="Save changes"
        primaryTestId="channel-policy-submit"
        primaryType="submit"
      />
    </form>
  );
}

export function ChannelPolicyDialog({
  channel,
  detail,
  members,
  isSubmitting,
  onOpenChange,
  onSubmit,
  open,
}: ChannelPolicyDialogProps) {
  const formKey = channelPolicyFormKey(channel, detail, open);

  return (
    <Dialog onOpenChange={onOpenChange} open={open}>
      <DialogContent
        className={`grid-rows-[auto_minmax(0,1fr)] ${dialogShellClass("sm")}`}
        data-testid="channel-policy-dialog"
        showCloseButton={false}
        unframed
      >
        <EntityDialogHeader
          description={`Update the purpose and fanout policy for #${channel.channel}. Identity and membership stay fixed by the update contract.`}
          eyebrow="Network · Channel"
          icon={Network}
          onClose={() => onOpenChange(false)}
          title="Edit channel"
        />

        <ChannelPolicyDialogForm
          channel={channel}
          detail={detail}
          isSubmitting={isSubmitting}
          key={formKey}
          members={members}
          onOpenChange={onOpenChange}
          onSubmit={onSubmit}
        />
      </DialogContent>
    </Dialog>
  );
}
