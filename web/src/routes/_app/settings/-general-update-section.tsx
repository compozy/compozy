import { ExternalLink, RefreshCw } from "@compozy/ui/icons";
import type { ReactNode } from "react";

import { Button, Pill, SkeletonRegion, SkeletonText, Spinner } from "@compozy/ui";
import {
  SettingRow,
  SettingValue,
  SettingsGroup,
  settingsProductUpdate,
  settingsUpdateHolderLabel,
  settingsUpdateView,
} from "@/systems/settings";
import type {
  SettingsProductUpdateView,
  SettingsUpdateApplyResult,
  SettingsUpdateCancelResult,
  SettingsUpdateHolder,
  SettingsUpdateStatus,
} from "@/systems/settings";

export interface GeneralUpdateActions {
  check: () => void;
  isChecking: boolean;
  checkError: string | null;
  /** Updates every part of CompozyOS this surface may update, in one operation. */
  apply: () => void;
  cancel: () => void;
  isApplying: boolean;
  isCanceling: boolean;
  /** The daemon's own answer to the last apply — `accepted` or `blocked`. */
  result: SettingsUpdateApplyResult | null;
  /** The daemon's own answer to the last cancel — `canceled` or declined. */
  cancelResult: SettingsUpdateCancelResult | null;
  error: string | null;
}

interface GeneralUpdateSectionProps {
  data?: SettingsUpdateStatus;
  error: unknown;
  isError: boolean;
  isLoading: boolean;
  actions: GeneralUpdateActions;
}

function RetryButton({ isFetching, onRetry }: { isFetching: boolean; onRetry: () => void }) {
  return (
    <Button
      data-testid="settings-page-general-update-retry"
      disabled={isFetching}
      onClick={onRetry}
      size="sm"
      type="button"
      variant="ghost"
    >
      {isFetching ? (
        <Spinner aria-hidden="true" className="size-3.5" />
      ) : (
        <RefreshCw aria-hidden="true" />
      )}
      Retry
    </Button>
  );
}

/** Who holds the update channel, in plain words; the process id stays in the tooltip. */
function UpdateHolderValue({ holder }: { holder: SettingsUpdateHolder }) {
  return (
    <span title={`${holder.surface} · PID ${holder.pid}`}>
      <SettingValue>{`In use by ${settingsUpdateHolderLabel(holder)}`}</SettingValue>
    </span>
  );
}

function UpdatesGroup({
  description,
  action,
  children,
}: {
  description?: ReactNode;
  action?: ReactNode;
  children: ReactNode;
}) {
  return (
    <SettingsGroup
      className="@container/updates"
      action={action}
      data-testid="settings-page-general-updates"
      description={description}
      title="Updates"
    >
      {children}
    </SettingsGroup>
  );
}

/** The first read of the update status: the group's own title and row label, a placeholder for the verdict. */
function UpdateCheckingGroup() {
  return (
    <SkeletonRegion
      data-testid="settings-page-general-update-checking"
      label="Checking for a new version"
    >
      <UpdatesGroup>
        <SettingRow
          data-testid="settings-page-general-update-status"
          label="CompozyOS"
          control={<SkeletonText className="w-32 text-small-body" />}
        />
      </UpdatesGroup>
    </SkeletonRegion>
  );
}

function UpdateFailureGroup({
  failed,
  message,
  isFetching,
  onRetry,
}: {
  failed: boolean;
  message: string;
  isFetching: boolean;
  onRetry: () => void;
}) {
  return (
    <UpdatesGroup>
      <SettingRow
        data-testid="settings-page-general-update-status"
        description={message}
        label="CompozyOS"
        control={
          <>
            <Pill form="plain" tone={failed ? "destructive" : "warning"}>
              <Pill.Dot />
              {failed ? "Check failed" : "Unavailable"}
            </Pill>
            <RetryButton isFetching={isFetching} onRetry={onRetry} />
          </>
        }
      />
    </UpdatesGroup>
  );
}

/** `current → latest` while a newer build is pending, else the bare version. */
function ProductVersion({ product }: { product: SettingsProductUpdateView }) {
  return (
    <SettingValue mono>
      {product.currentVersion}
      {product.latestVersion ? (
        <>
          <span aria-hidden="true" className="text-faint">
            {" → "}
          </span>
          <span className="sr-only"> to </span>
          <span className="text-fg">{product.latestVersion}</span>
        </>
      ) : null}
    </SettingValue>
  );
}

/** Live work reads as one word; percent qualifies it only when something measures it. */
function ProductProgress({ product }: { product: SettingsProductUpdateView }) {
  return (
    <span
      aria-atomic="true"
      aria-live="polite"
      className="flex items-center gap-1.5"
      data-phase={product.phase ?? undefined}
      data-testid="settings-page-general-update-progress"
      role="status"
    >
      <Spinner aria-hidden="true" className="size-3.5 text-info" role="presentation" />
      <SettingValue>
        <span className="text-fg">Updating</span>
        {product.percent === null ? null : (
          <>
            <span aria-hidden="true" className="text-faint">
              {" · "}
            </span>
            <span className="tabular-nums">{product.percent}%</span>
          </>
        )}
      </SettingValue>
    </span>
  );
}

function productDescription(product: SettingsProductUpdateView): ReactNode {
  if (!product.description && !product.recommendation) return undefined;
  return (
    <>
      {product.description}
      {product.recommendation ? (
        <code
          className="mt-1 block w-fit font-mono whitespace-nowrap text-fg"
          data-testid="settings-page-general-update-recommendation"
        >
          {product.recommendation}
        </code>
      ) : null}
    </>
  );
}

function ProductUpdateRow({
  product,
  actions,
  canApply,
}: {
  product: SettingsProductUpdateView;
  actions: GeneralUpdateActions;
  canApply: boolean;
}) {
  const updating = product.updating || actions.isApplying;
  return (
    <SettingRow
      className="@max-xl/updates:flex-col @max-xl/updates:items-start [&>div:last-child]:max-w-full"
      data-testid="settings-page-general-update-status"
      description={productDescription(product)}
      label="CompozyOS"
      control={
        <div className="flex min-w-0 flex-wrap items-center gap-2">
          <ProductVersion product={product} />
          {updating ? (
            <ProductProgress product={product} />
          ) : (
            <Pill form="plain" tone={product.tone}>
              <Pill.Dot />
              {product.statusLabel}
            </Pill>
          )}
          {canApply && !updating ? (
            <Button
              data-testid="settings-page-general-update-apply"
              onClick={actions.apply}
              size="sm"
              type="button"
              variant="primary"
            >
              {product.status === "failed" ? "Retry update" : "Update"}
            </Button>
          ) : null}
          {product.canCancel ? (
            <Button
              data-testid="settings-page-general-update-cancel"
              disabled={actions.isCanceling}
              onClick={actions.cancel}
              size="sm"
              type="button"
              variant="ghost"
            >
              {actions.isCanceling ? <Spinner className="size-3.5" /> : null}
              Cancel update
            </Button>
          ) : null}
          {product.releaseUrl && product.latestVersion ? (
            <Button
              nativeButton={false}
              render={
                <a
                  aria-label="Open release notes"
                  data-testid="settings-page-general-update-release"
                  href={product.releaseUrl}
                  rel="noreferrer"
                  target="_blank"
                />
              }
              size="sm"
              variant="ghost"
            >
              <ExternalLink aria-hidden="true" className="text-subtle" />
            </Button>
          ) : null}
        </div>
      }
    />
  );
}

function UpdateBlockedRow({ result }: { result: SettingsUpdateApplyResult | null }) {
  // A blocked apply is a 200 whose body refuses; it never reads as success.
  if (result?.status !== "blocked") return null;
  return (
    <SettingRow
      data-testid="settings-page-general-update-blocked"
      description={result.message}
      label="Update channel busy"
      control={result.holder ? <UpdateHolderValue holder={result.holder} /> : undefined}
    />
  );
}

function UpdateCancelResultRow({ result }: { result: SettingsUpdateCancelResult | null }) {
  if (!result) return null;
  const canceled = result.status === "canceled";
  return (
    <SettingRow
      data-testid="settings-page-general-update-cancel-result"
      description={result.message}
      label={canceled ? "Update canceled" : "Cancel declined"}
      control={
        result.holder ? (
          <UpdateHolderValue holder={result.holder} />
        ) : (
          <Pill form="plain" tone={canceled ? "neutral" : "warning"}>
            <Pill.Dot />
            {canceled ? "Canceled" : "Declined"}
          </Pill>
        )
      }
    />
  );
}

/**
 * Settings → General → Updates: CompozyOS is one product with one version and one
 * Update button. The daemon still applies the runtime and the desktop app as one
 * durable operation (ADR-006); that split never surfaces here. The section holds
 * no shell awareness — it reads the daemon like any browser client, so it renders
 * identically in the desktop app and in a plain browser.
 */
export function GeneralUpdateSection(props: GeneralUpdateSectionProps) {
  const { actions } = props;
  const view = settingsUpdateView(props);
  if (view.kind === "checking") return <UpdateCheckingGroup />;
  if (view.kind !== "snapshot") {
    return (
      <UpdateFailureGroup
        failed={view.kind === "error"}
        isFetching={actions.isChecking}
        message={view.message}
        onRetry={actions.check}
      />
    );
  }

  const refreshError = actions.checkError ?? view.refreshError;
  const checkedAt = view.snapshot.runtime.checked_at;
  const product = settingsProductUpdate(view.snapshot);

  return (
    <UpdatesGroup
      // A failed refresh is one event for the whole section, so it lives once in
      // the header while the row keeps its last known truth.
      description={
        <>
          {checkedAt ? (
            <span>
              Last checked <time dateTime={checkedAt}>{new Date(checkedAt).toLocaleString()}</time>.
            </span>
          ) : (
            <span>No successful update check yet.</span>
          )}
          {refreshError ? (
            <span> Showing the last known status. Check failed: {refreshError}</span>
          ) : null}
        </>
      }
      action={
        <>
          {refreshError ? (
            <Pill form="plain" tone="destructive">
              <Pill.Dot />
              Refresh failed
            </Pill>
          ) : null}
          <Button
            disabled={actions.isChecking}
            onClick={actions.check}
            size="sm"
            type="button"
            variant="ghost"
          >
            {actions.isChecking ? (
              <Spinner aria-hidden="true" className="size-3.5" />
            ) : (
              <RefreshCw aria-hidden="true" />
            )}
            {actions.isChecking ? "Checking…" : "Check for updates"}
          </Button>
        </>
      }
    >
      <ProductUpdateRow
        actions={actions}
        canApply={!refreshError && product.targets !== null}
        product={
          !checkedAt && product.status === "up-to-date"
            ? { ...product, statusLabel: "Not checked", tone: "neutral" }
            : product
        }
      />
      <UpdateBlockedRow result={actions.result} />
      <UpdateCancelResultRow result={actions.cancelResult} />
      {actions.error ? (
        <SettingRow
          data-testid="settings-page-general-update-action-error"
          description={<span className="text-destructive">{actions.error}</span>}
          label="The update couldn't start"
        />
      ) : null}
      {product.lastError ? (
        <SettingRow
          data-testid="settings-page-general-update-last-error"
          description={<span className="text-destructive">{product.lastError}</span>}
          label="What went wrong"
        />
      ) : null}
    </UpdatesGroup>
  );
}
