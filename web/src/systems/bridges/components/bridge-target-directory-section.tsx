import { AlertCircle, CheckCircle2, ListFilter, Search, Waypoints } from "lucide-react";
import type { FormEvent } from "react";

import {
  Button,
  DataSurface,
  Empty,
  Eyebrow,
  Field,
  FieldContent,
  FieldDescription,
  FieldTitle,
  Input,
  Pill,
  SearchInput,
  Section,
  Spinner,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@agh/ui";

import {
  bridgeTargetTypeLabel,
  describeBridgeTargetCapabilities,
  describeBridgeTargetQualifier,
  formatBridgeRelativeTime,
} from "../lib/bridge-formatters";
import type { BridgeResolveTargetResponse, BridgeTarget, BridgeTargetsResponse } from "../types";

export interface BridgeTargetDirectoryState {
  error: Error | null;
  isLoading: boolean;
  isResolving: boolean;
  onQueryChange: (query: string) => void;
  onResolveInputChange: (value: string) => void;
  onResolveSubmit: () => void;
  query: string;
  resolveInput: string;
  resolveResult: BridgeResolveTargetResponse | null;
  response?: BridgeTargetsResponse;
}

function TargetIdentity({ target }: { target: BridgeTarget }) {
  return (
    <div className="min-w-0">
      <div className="truncate text-small-body font-medium text-fg">{target.display_name}</div>
      <div className="mt-1 truncate font-mono text-eyebrow text-subtle">
        {describeBridgeTargetCapabilities(target)}
      </div>
    </div>
  );
}

function BridgeTargetRow({ target }: { target: BridgeTarget }) {
  return (
    <TableRow data-testid={`bridge-target-${target.canonical_route}`}>
      <TableCell>
        <TargetIdentity target={target} />
      </TableCell>
      <TableCell>
        <Pill mono tone="neutral">
          {bridgeTargetTypeLabel(target)}
        </Pill>
      </TableCell>
      <TableCell className="font-mono text-xs text-muted">
        {describeBridgeTargetQualifier(target)}
      </TableCell>
      <TableCell className="max-w-[18rem] truncate font-mono text-xs text-muted">
        {target.canonical_route}
      </TableCell>
      <TableCell className="font-mono text-xs text-subtle">
        {formatBridgeRelativeTime(target.last_seen_at ?? target.updated_at)}
      </TableCell>
    </TableRow>
  );
}

function BridgeTargetCard({ target }: { target: BridgeTarget }) {
  return (
    <article
      className="min-w-0 rounded-md border border-line bg-canvas px-3 py-3"
      data-testid={`bridge-target-card-${target.canonical_route}`}
    >
      <div className="flex min-w-0 flex-wrap items-center gap-2">
        <div className="min-w-0 flex-1">
          <TargetIdentity target={target} />
        </div>
        <Pill mono tone="neutral">
          {bridgeTargetTypeLabel(target)}
        </Pill>
      </div>
      <dl className="mt-3 grid gap-2 text-xs">
        <div className="min-w-0">
          <dt className="text-muted">Qualifier</dt>
          <dd className="mt-1 break-all font-mono text-subtle">
            {describeBridgeTargetQualifier(target)}
          </dd>
        </div>
        <div className="min-w-0">
          <dt className="text-muted">Route</dt>
          <dd className="mt-1 break-all font-mono text-subtle">{target.canonical_route}</dd>
        </div>
        <div className="min-w-0">
          <dt className="text-muted">Seen</dt>
          <dd className="mt-1 font-mono text-subtle">
            {formatBridgeRelativeTime(target.last_seen_at ?? target.updated_at)}
          </dd>
        </div>
      </dl>
    </article>
  );
}

function BridgeResolveResult({ result }: { result: BridgeResolveTargetResponse }) {
  const diagnostic = result.diagnostic;
  if (result.result.match) {
    const target = result.result.match;
    return (
      <div
        className="rounded-md border border-success/40 bg-success-tint px-4 py-3"
        data-testid="bridge-target-resolve-result"
      >
        <div className="flex flex-wrap items-center gap-2">
          <CheckCircle2 className="size-4 text-success" />
          <span className="text-small-body font-medium text-fg">{target.display_name}</span>
          <Pill mono tone="success">
            STEP {result.result.step}
          </Pill>
        </div>
        <p className="mt-2 break-all font-mono text-xs text-muted">{target.canonical_route}</p>
      </div>
    );
  }

  if (result.result.ambiguous) {
    return (
      <div
        className="rounded-md border border-warning/40 bg-warning-tint px-4 py-3"
        data-testid="bridge-target-resolve-ambiguous"
      >
        <div className="flex flex-wrap items-center gap-2">
          <AlertCircle className="size-4 text-warning" />
          <span className="text-small-body font-medium text-fg">
            {diagnostic?.title ?? "Bridge target is ambiguous"}
          </span>
          <Pill mono tone="warning">
            {result.result.candidates?.length ?? 0} candidates
          </Pill>
        </div>
        {diagnostic?.message ? (
          <p className="mt-2 text-xs text-muted">{diagnostic.message}</p>
        ) : null}
        <div className="mt-3 grid gap-2 md:grid-cols-2">
          {(result.result.candidates ?? []).map(candidate => (
            <div
              className="min-w-0 rounded-md border border-line bg-canvas-soft px-3 py-2"
              data-testid={`bridge-target-resolve-candidate-${candidate.canonical_route}`}
              key={candidate.canonical_route}
            >
              <TargetIdentity target={candidate} />
            </div>
          ))}
        </div>
      </div>
    );
  }

  return (
    <div
      className="rounded-md border border-danger/40 bg-danger-tint px-4 py-3"
      data-testid="bridge-target-resolve-unknown"
    >
      <div className="flex flex-wrap items-center gap-2">
        <AlertCircle className="size-4 text-danger" />
        <span className="text-small-body font-medium text-fg">
          {diagnostic?.title ?? "Bridge target not found"}
        </span>
      </div>
      {diagnostic?.message ? <p className="mt-2 text-xs text-muted">{diagnostic.message}</p> : null}
    </div>
  );
}

export function BridgeTargetDirectorySection({ state }: { state?: BridgeTargetDirectoryState }) {
  const targets = state?.response?.targets ?? [];
  const total = state?.response?.total ?? targets.length;
  const handleResolveSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    state?.onResolveSubmit();
  };

  return (
    <Section
      label="Target directory"
      right={
        <div className="flex flex-wrap items-center gap-2">
          {state?.response?.cache_stale ? (
            <Pill mono tone="warning">
              STALE
            </Pill>
          ) : null}
          <Pill mono>{total}</Pill>
        </div>
      }
    >
      <div className="grid gap-3 xl:grid-cols-[minmax(0,1.45fr)_minmax(18rem,0.55fr)]">
        <div
          className="min-w-0 overflow-hidden rounded-md border border-line bg-canvas-soft"
          data-testid="bridge-target-directory"
        >
          <div className="flex flex-wrap items-center justify-between gap-3 border-b border-line px-4 py-3">
            <SearchInput
              containerClassName="min-w-0 flex-1"
              data-testid="bridge-target-search"
              onChange={value => state?.onQueryChange(value)}
              placeholder="Filter targets"
              value={state?.query ?? ""}
            />
            <div className="flex items-center gap-2 text-xs text-subtle">
              <ListFilter className="size-3" />
              <span className="font-mono">
                {formatBridgeRelativeTime(state?.response?.last_successful_refresh_at)}
              </span>
            </div>
          </div>
          {state?.isLoading ? (
            <DataSurface.Loading data-testid="bridge-targets-loading" label="Loading targets" />
          ) : state?.error ? (
            <div className="p-4" data-testid="bridge-targets-error">
              <Empty
                description={state.error.message}
                icon={AlertCircle}
                title="Unable to load targets"
              />
            </div>
          ) : targets.length === 0 ? (
            <div className="p-4" data-testid="bridge-targets-empty">
              <Empty
                description="No provider targets have been discovered for this bridge."
                icon={Waypoints}
                title="No targets"
              />
            </div>
          ) : (
            <>
              <div className="grid gap-3 p-3 md:hidden">
                {targets.map(target => (
                  <BridgeTargetCard key={target.canonical_route} target={target} />
                ))}
              </div>
              <div className="hidden overflow-x-auto md:block" data-testid="bridge-targets-table">
                <Table className="min-w-[46rem]">
                  <TableHeader>
                    <TableRow>
                      {["Target", "Type", "Qualifier", "Route", "Seen"].map(label => (
                        <TableHead key={label}>
                          <Eyebrow>{label}</Eyebrow>
                        </TableHead>
                      ))}
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {targets.map(target => (
                      <BridgeTargetRow key={target.canonical_route} target={target} />
                    ))}
                  </TableBody>
                </Table>
              </div>
            </>
          )}
        </div>

        <form
          className="min-w-0 rounded-md border border-line bg-canvas-soft px-4 py-3"
          data-testid="bridge-target-resolver"
          onSubmit={handleResolveSubmit}
        >
          <Field>
            <FieldContent>
              <FieldTitle>Resolve target</FieldTitle>
              <FieldDescription>
                Returns the canonical provider route without sending.
              </FieldDescription>
            </FieldContent>
            <div className="flex gap-2">
              <Input
                aria-label="Bridge target name"
                data-testid="bridge-target-resolve-input"
                onChange={event => state?.onResolveInputChange(event.target.value)}
                placeholder="launch room"
                value={state?.resolveInput ?? ""}
              />
              <Button
                data-testid="bridge-target-resolve-submit"
                disabled={!state || !state.resolveInput.trim() || state.isResolving}
                size="sm"
                type="submit"
              >
                {state?.isResolving ? (
                  <Spinner aria-hidden="true" className="size-3" />
                ) : (
                  <Search className="size-3" />
                )}
                Resolve
              </Button>
            </div>
          </Field>
          {state?.resolveResult ? (
            <div className="mt-3">
              <BridgeResolveResult result={state.resolveResult} />
            </div>
          ) : null}
        </form>
      </div>
    </Section>
  );
}
