import { ExternalLink, FileJson2, RefreshCw } from "lucide-react";

import {
  ActionResultBanner,
  Button,
  CodeBlock,
  FormSection,
  OperationalLinksRow,
  Spinner,
} from "@agh/ui";

export interface BridgeManifestCommittedState {
  bridgeId: string;
  error?: Error | string | null;
  isLoading: boolean;
  manifestJSON?: string;
  onOpenBridge: () => void;
  onRetry: () => void;
}

interface BridgeManifestHandoffProps {
  state: BridgeManifestCommittedState;
}

function manifestErrorMessage(error: BridgeManifestCommittedState["error"]): string {
  if (error instanceof Error) return error.message;
  if (typeof error === "string" && error.trim()) return error;
  return "The bridge is saved, but its Slack manifest could not be loaded.";
}

export function BridgeManifestHandoff({ state }: BridgeManifestHandoffProps) {
  return (
    <div className="flex flex-col gap-4" data-testid="bridge-manifest-handoff">
      <ActionResultBanner
        description={`Bridge ${state.bridgeId} is persisted. Closing this dialog will not remove it.`}
        title="Bridge created"
        tone="success"
      />

      <FormSection
        description="Import the daemon-generated JSON in Slack, then return to the bridge to bind credentials and verify it."
        icon={FileJson2}
        title="Slack app manifest"
      >
        {state.isLoading ? (
          <div
            aria-live="polite"
            className="flex min-h-32 items-center justify-center gap-2 text-small-body text-muted"
            data-testid="bridge-manifest-loading"
          >
            <Spinner aria-label="Loading Slack app manifest" className="size-4" />
            Loading manifest…
          </div>
        ) : state.error || !state.manifestJSON ? (
          <ActionResultBanner
            actions={
              <Button onClick={state.onRetry} size="sm" type="button" variant="outline">
                <RefreshCw aria-hidden="true" className="size-3" />
                Retry
              </Button>
            }
            data-testid="bridge-manifest-error"
            description={manifestErrorMessage(state.error)}
            role="alert"
            title="Manifest unavailable"
            tone="danger"
          />
        ) : (
          <>
            <CodeBlock
              caption="Slack app manifest"
              code={state.manifestJSON}
              copyLabel="Copy Slack app manifest"
              data-testid="bridge-manifest-json"
              language="json"
              showPrompt={false}
            />
            <ol className="list-decimal space-y-1 pl-5 text-small-body leading-relaxed text-muted">
              <li>Open the Slack app dashboard and choose Create New App.</li>
              <li>Select From an app manifest, then paste the copied JSON.</li>
              <li>Install the app, return to AGH, bind its credentials, and run verification.</li>
            </ol>
            <OperationalLinksRow
              ariaLabel="Slack setup links"
              items={[
                {
                  href: "https://api.slack.com/apps",
                  icon: ExternalLink,
                  label: "Open Slack app dashboard",
                  target: "_blank",
                },
              ]}
            />
          </>
        )}
      </FormSection>

      <div className="flex justify-end">
        <Button
          data-testid="bridge-manifest-open-bridge"
          onClick={state.onOpenBridge}
          size="sm"
          type="button"
        >
          Open bridge
        </Button>
      </div>
    </div>
  );
}
