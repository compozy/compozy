import {
  Alert,
  AlertDescription,
  AlertTitle,
  Button,
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  Eyebrow,
  Pill,
  Spinner,
  Textarea,
} from "@agh/ui";
import { Check, CircleAlert, CircleCheck, Copy, ExternalLink, Plug } from "lucide-react";
import { type ReactNode, useState } from "react";
import { toast } from "sonner";

import type { UseMCPAuthorizeReturn } from "../hooks/use-mcp-authorize";

import { authTone, formatStatusLabel } from "../lib/mcp-status-view-model";
import type { SettingsMCPServerEntry } from "../types";

export interface MCPAuthorizeDialogProps {
  authorize: UseMCPAuthorizeReturn;
  scope: "workspace" | "global";
  /** The live entry for the authorizing server; drives the status snapshot. */
  server: SettingsMCPServerEntry | null;
}

export function MCPAuthorizeDialog({ authorize, scope, server }: MCPAuthorizeDialogProps) {
  return (
    <Dialog
      open={authorize.isOpen}
      onOpenChange={next => {
        if (!next) authorize.cancel();
      }}
    >
      <DialogContent
        className="grid w-[calc(100%-2rem)] max-w-xl gap-4"
        unframed
        data-testid="settings-page-mcp-authorize"
      >
        {authorize.isOpen ? (
          <AuthorizeContent
            key={authorize.server ?? "none"}
            authorize={authorize}
            scope={scope}
            server={server}
          />
        ) : null}
      </DialogContent>
    </Dialog>
  );
}

function AuthorizeContent({ authorize, scope, server }: MCPAuthorizeDialogProps) {
  const [manualValue, setManualValue] = useState("");
  const name = authorize.server ?? server?.name ?? "";
  const phase = authorize.phase;
  const url = authorize.begin?.authorization_url ?? "";

  const isFailure = phase === "failed";
  const isConfirmed = phase === "confirmed";
  const isManual = phase === "manual";
  const hasActiveBegin = authorize.begin !== null;
  const showUrl = Boolean(url) && !isConfirmed;
  const showManual = isManual || (isFailure && hasActiveBegin);

  return (
    <>
      <DialogHeader variant="ruled">
        <Eyebrow className="flex items-center gap-2 text-muted">
          <Plug className="size-3.5" />
          {name} · {scope} · oauth2_pkce
        </Eyebrow>
        <DialogTitle>Authorize {name}</DialogTitle>
        <DialogDescription>
          Complete provider authorization without exposing codes or tokens in server configuration.
        </DialogDescription>
      </DialogHeader>

      <div
        className="flex max-h-[60vh] flex-col gap-0 overflow-y-auto px-5 py-4"
        data-testid="settings-page-mcp-authorize-body"
      >
        {phase === "beginning" ? (
          <Alert variant="warning" data-testid="settings-page-mcp-authorize-beginning">
            <Spinner className="size-3.5" />
            <AlertTitle>Starting authorization</AlertTitle>
            <AlertDescription>
              Requesting a live authorization URL from the daemon.
            </AlertDescription>
          </Alert>
        ) : null}

        {isConfirmed ? (
          <Alert
            variant="success"
            role="status"
            data-testid="settings-page-mcp-authorize-confirmed"
          >
            <CircleCheck />
            <AlertTitle>Authorization confirmed</AlertTitle>
            <AlertDescription>
              The scoped status now reports authenticated with a present token. Runtime readiness
              remains an independent daemon result.
            </AlertDescription>
          </Alert>
        ) : null}

        {isFailure ? (
          <Alert variant="danger" role="alert" data-testid="settings-page-mcp-authorize-failure">
            <CircleAlert />
            <AlertTitle>
              {hasActiveBegin
                ? "Authorization could not be completed"
                : "Authorization could not be started"}
            </AlertTitle>
            <AlertDescription>
              {authorize.error ??
                `${name} keeps its prior status; no existing credential was changed.`}
            </AlertDescription>
          </Alert>
        ) : null}

        {phase === "waiting" ? (
          <Alert variant="warning" role="status" data-testid="settings-page-mcp-authorize-waiting">
            <CircleAlert />
            <AlertTitle>Waiting for confirmed token</AlertTitle>
            <AlertDescription>
              The browser step may finish automatically. This dialog stays pending until status
              reports authenticated and token_present.
            </AlertDescription>
          </Alert>
        ) : null}

        {isConfirmed || isFailure ? <AuthSnapshot server={server} authorize={authorize} /> : null}

        {showUrl ? <AuthorizationUrlBlock url={url} /> : null}

        {showManual ? (
          <div
            className="mt-3.5 border-t border-line-soft pt-3.5"
            data-testid="settings-page-mcp-authorize-manual"
          >
            <label
              className="mb-1.5 flex items-center gap-2 text-form-label font-medium text-fg"
              htmlFor="mcp-authorize-manual-value"
            >
              Code or full redirect URL
              <Eyebrow className="ml-auto text-muted">required</Eyebrow>
            </label>
            <Textarea
              id="mcp-authorize-manual-value"
              className="font-mono"
              value={manualValue}
              onChange={event => setManualValue(event.target.value)}
              placeholder="Paste the authorization code or the full redirected URL"
              aria-describedby="mcp-authorize-manual-help"
              data-testid="settings-page-mcp-authorize-manual-input"
            />
            <p className="mt-1.5 text-caption text-muted" id="mcp-authorize-manual-help">
              Use this when the browser cannot reach the daemon host. AGH sends exactly one value to
              auth/exchange.
            </p>
          </div>
        ) : null}
      </div>

      <DialogFooter variant="ruled" className="grid items-center">
        <span className="text-caption text-muted">
          Existing credentials stay unchanged until a confirmed exchange.
        </span>
        <div className="flex flex-wrap items-center justify-end gap-2">
          <Button
            type="button"
            variant="ghost"
            size="sm"
            onClick={authorize.cancel}
            data-testid="settings-page-mcp-authorize-cancel"
          >
            {isConfirmed ? "Close" : "Cancel"}
          </Button>
          <AuthorizePrimaryAction
            authorize={authorize}
            manualValue={manualValue}
            showManual={showManual}
          />
        </div>
      </DialogFooter>
    </>
  );
}

function AuthorizePrimaryAction({
  authorize,
  manualValue,
  showManual,
}: {
  authorize: UseMCPAuthorizeReturn;
  manualValue: string;
  showManual: boolean;
}) {
  if (authorize.phase === "confirmed") {
    return (
      <Button
        type="button"
        size="sm"
        onClick={authorize.cancel}
        data-testid="settings-page-mcp-authorize-done"
      >
        Done
      </Button>
    );
  }
  if (showManual) {
    return (
      <Button
        type="button"
        size="sm"
        disabled={manualValue.trim() === "" || authorize.isExchanging}
        onClick={() => authorize.submitManual(manualValue)}
        data-testid="settings-page-mcp-authorize-exchange"
      >
        {authorize.isExchanging ? <Spinner className="size-3" /> : null}
        Complete authorization
      </Button>
    );
  }
  if (authorize.phase === "failed" && authorize.begin === null) {
    return (
      <div className="flex items-center gap-2">
        {authorize.mode !== "manual" ? (
          <Button
            type="button"
            variant="neutral"
            size="sm"
            onClick={() => void authorize.enterManual()}
            data-testid="settings-page-mcp-authorize-manual-fallback"
          >
            Try manual callback
          </Button>
        ) : null}
        <Button
          type="button"
          size="sm"
          onClick={() => void authorize.retryBegin()}
          data-testid="settings-page-mcp-authorize-retry"
        >
          Retry authorization
        </Button>
      </div>
    );
  }
  return (
    <Button
      type="button"
      variant="neutral"
      size="sm"
      disabled={authorize.phase !== "waiting"}
      onClick={() => void authorize.enterManual()}
      data-testid="settings-page-mcp-authorize-manual-trigger"
    >
      Enter code or redirect
    </Button>
  );
}

function AuthorizationUrlBlock({ url }: { url: string }) {
  const [copied, setCopied] = useState(false);
  const handleCopy = async () => {
    if (!navigator.clipboard) {
      toast.error("Clipboard is unavailable in this browser. Copy the URL manually.");
      return;
    }
    try {
      await navigator.clipboard.writeText(url);
      setCopied(true);
    } catch (error) {
      toast.error(
        error instanceof Error ? `Could not copy URL: ${error.message}` : "Could not copy the URL."
      );
    }
  };
  return (
    <div className="my-3.5 rounded-md bg-canvas p-3" data-testid="settings-page-mcp-authorize-url">
      <div className="mb-2 flex flex-wrap items-center gap-2">
        <Eyebrow className="flex-1 text-muted">Live URL from auth/begin</Eyebrow>
        <Button
          type="button"
          variant="ghost"
          size="sm"
          onClick={() => void handleCopy()}
          data-testid="settings-page-mcp-authorize-copy-url"
        >
          {copied ? <Check /> : <Copy />}
          {copied ? "Copied" : "Copy URL"}
        </Button>
        <Button
          type="button"
          variant="neutral"
          size="sm"
          onClick={() => window.open(url, "_blank", "noopener,noreferrer")}
          data-testid="settings-page-mcp-authorize-open-url"
        >
          <ExternalLink />
          Open browser
        </Button>
      </div>
      <code className="block break-words font-mono text-mono-id leading-relaxed text-fg">
        {url}
      </code>
    </div>
  );
}

function AuthSnapshot({
  server,
  authorize,
}: {
  server: SettingsMCPServerEntry | null;
  authorize: UseMCPAuthorizeReturn;
}) {
  const status = server?.auth_status?.status ?? "unknown";
  const tokenPresent = Boolean(server?.auth_status?.token_present);
  const runtime = server?.runtime_status?.state ?? "unknown";
  const prior = authorize.prior;
  return (
    <dl
      className="mt-3 grid grid-cols-2 gap-px overflow-hidden rounded-md border border-line bg-line"
      data-testid="settings-page-mcp-authorize-snapshot"
    >
      <SnapshotCell term="Authorization">
        <Pill tone={authTone(status, tokenPresent)}>{formatStatusLabel(status)}</Pill>
      </SnapshotCell>
      <SnapshotCell term="Token present">
        <Pill tone={tokenPresent ? "success" : "neutral"}>{tokenPresent ? "Yes" : "No"}</Pill>
      </SnapshotCell>
      <SnapshotCell term="Prior status" mono>
        {prior ? `${prior.status} · ${prior.tokenPresent ? "token present" : "token absent"}` : "-"}
      </SnapshotCell>
      <SnapshotCell term="Runtime" mono>
        {runtime}
      </SnapshotCell>
    </dl>
  );
}

function SnapshotCell({
  term,
  mono,
  children,
}: {
  term: string;
  mono?: boolean;
  children: ReactNode;
}) {
  return (
    <div className="min-w-0 bg-canvas p-2.5">
      <dt className="eyebrow text-subtle">{term}</dt>
      <dd className={mono ? "mt-1 font-mono text-mono-id text-fg" : "mt-1 text-form-label text-fg"}>
        {children}
      </dd>
    </div>
  );
}
