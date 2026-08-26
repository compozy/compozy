/**
 * The call-detail location — one call's whole record.
 *
 * Composition only. Every decision about which controls may render lives in the
 * view model, so this file cannot drift from the truthful-UI rules by adding a
 * button in the wrong branch.
 */
import { AlertCircle, ArrowLeft, GitBranch } from "lucide-react";

import { Button, Empty, ListingPage, useTopbarSlot } from "@compozy/ui";

import { AgentCallCompose, AgentCallDetail, AgentComposeMessage } from "@/systems/agent-comms";

import { useAgentCall } from "./use-agent-call";

export function AgentCallLocation({ callId, windowId }: { callId: string; windowId: string }) {
  const page = useAgentCall(callId, windowId);

  useTopbarSlot({
    glyph: <GitBranch />,
    onBack: page.openActivity,
    crumbs: [{ id: "activity", label: "Activity", onSelect: page.openActivity }],
    crumb: page.view?.agentName ?? callId,
  });

  if (page.error) {
    return (
      <div
        className="flex min-h-0 flex-1 items-center justify-center py-10"
        data-testid="agent-call-error"
      >
        <Empty
          action={
            <Button onClick={page.openActivity} size="sm" type="button" variant="ghost">
              <ArrowLeft aria-hidden="true" className="size-3" />
              Back to Activity
            </Button>
          }
          description="This call is not in the current profile, or it no longer exists."
          icon={AlertCircle}
          title="Call not found"
        />
      </div>
    );
  }

  if (!page.view) {
    return (
      <div
        className="flex min-h-0 flex-1 items-center justify-center py-10"
        data-testid="agent-call-loading"
      >
        <Empty description="Loading the call record." icon={GitBranch} title="One moment" />
      </div>
    );
  }

  return (
    <ListingPage data-testid="agent-call-page">
      <AgentCallDetail
        data-testid="agent-call-detail"
        view={page.view}
        childUsage={page.childUsage}
        fullPayload={page.fullPayload}
        onFetchFullPayload={page.fetchFullPayload}
        fullPayloadPending={page.fullPayloadPending}
        fullPayloadError={page.fullPayloadError}
        fullPrompt={page.fullPrompt}
        onFetchFullPrompt={page.fetchFullPrompt}
        fullPromptPending={page.fullPromptPending}
        fullPromptError={page.fullPromptError}
        supersededPayload={page.supersededPayload}
        onFetchSuperseded={page.fetchSuperseded}
        supersededPending={page.supersededPending}
        supersededError={page.supersededError}
        onCancel={page.cancel}
        cancelPending={page.cancelPending}
        cancelOutcome={page.cancelOutcome}
        cancelFailure={page.cancelFailure}
        onRetryCancel={page.retryCancel}
        {...(page.view.controls.callAgain ? { onCallAgain: page.callAgain } : {})}
        {...(page.view.controls.messageChild ? { onMessageChild: page.messageChild } : {})}
        onOpenCaller={page.openCaller}
        onOpenChildSession={page.openChildSession}
      />

      {/*
        These are the two compose disclosures on this screen. Their open state
        belongs to the location and is mutually exclusive.
      */}
      {page.composing === "call-again" ? (
        <AgentCallCompose
          data-testid="agent-call-again-compose"
          agentName={page.view.agentName ?? ""}
          target={page.callAgainTarget}
          contractRequired={Boolean(page.view.expectDigest)}
          prompt={page.compose.prompt}
          onPromptChange={page.compose.setPrompt}
          expect={page.compose.expect}
          onExpectChange={page.compose.setExpect}
          onSubmit={page.compose.submit}
          pending={page.compose.pending}
          failure={page.compose.failure}
          accepted={page.compose.accepted}
          onOpenAcceptedCall={page.openCall}
        />
      ) : null}

      {page.composing === "message" ? (
        <AgentComposeMessage
          data-testid="agent-call-message-compose"
          targetLabel={page.view.childSessionId ?? page.view.agentName ?? "the child"}
          value={page.messageText}
          onChange={page.setMessageText}
          onSend={page.sendMessage}
          pending={page.messagePending}
          failureCode={page.messageFailureCode}
          accepted={page.messageAccepted}
        />
      ) : null}
    </ListingPage>
  );
}
