import type { ReactNode } from "react";

import {
  Alert,
  AlertDescription,
  Button,
  Empty,
  SkeletonText,
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@compozy/ui";
import {
  AlertCircle,
  ChevronLeft,
  ChevronRight,
  FileDiff,
  FolderX,
  GitBranch,
  Layers,
} from "@compozy/ui/icons";

import { formatChangeCount } from "../../lib/session-changes";

/** A toolbar or header icon button with its label as tooltip and accessible name. */
export function ChangesIconAction({
  label,
  onClick,
  pressed,
  disabled,
  testId,
  children,
}: {
  label: string;
  onClick: () => void;
  pressed?: boolean;
  disabled?: boolean;
  testId: string;
  children: ReactNode;
}) {
  return (
    <Tooltip>
      <TooltipTrigger
        render={
          <Button
            type="button"
            variant="quiet"
            size="icon-xs"
            aria-label={label}
            aria-pressed={pressed}
            disabled={disabled}
            data-testid={testId}
            onClick={onClick}
          />
        }
      >
        {children}
      </TooltipTrigger>
      <TooltipContent>{label}</TooltipContent>
    </Tooltip>
  );
}

/** Loading summary: a skeleton header and three file skeletons (`_uiux.md` S2). */
export function ChangesLoadingState() {
  return (
    <div className="flex flex-col gap-4 p-3" aria-busy data-testid="changes-loading">
      <SkeletonText lines={1} />
      {[0, 1, 2].map(index => (
        <div key={index} className="flex flex-col gap-2 rounded-md border border-line-soft p-3">
          <SkeletonText lines={4} />
        </div>
      ))}
    </div>
  );
}

export function ChangesMessageState({
  icon,
  title,
  description,
  action,
  testId,
}: {
  icon: typeof FileDiff;
  title: string;
  description?: ReactNode;
  action?: ReactNode;
  testId: string;
}) {
  return (
    <Empty
      size="compact"
      icon={icon}
      title={title}
      description={description}
      action={action}
      data-testid={testId}
    />
  );
}

/** The summary read failed with nothing to keep: one truthful state per daemon code. */
export function ChangesErrorState({
  code,
  message,
  base,
  onPickBase,
  onRetry,
}: {
  code: string | null;
  message: string | null;
  base: string | undefined;
  onPickBase: () => void;
  onRetry: () => void;
}) {
  if (code === "not_a_git_repository") {
    return (
      <ChangesMessageState
        icon={FolderX}
        title="This folder isn't a git repository"
        description="Changes need git. Files still works."
        testId="changes-not-a-repo"
      />
    );
  }
  if (code === "base_not_found") {
    return (
      <ChangesMessageState
        icon={GitBranch}
        title={base ? `Base ${base} not found` : "Base not found"}
        description="Pick another base to compare against."
        action={
          <Button type="button" variant="outline" size="sm" onClick={onPickBase}>
            Choose base
          </Button>
        }
        testId="changes-base-not-found"
      />
    );
  }
  return (
    <ChangesMessageState
      icon={AlertCircle}
      title="Couldn't read changes"
      description={message ?? undefined}
      action={
        <Button type="button" variant="outline" size="sm" onClick={onRetry}>
          Retry
        </Button>
      }
      testId="changes-error"
    />
  );
}

/** A refresh failed: the last good summary stays below (US-010.EC-2). */
export function ChangesRefreshFailed({ onRetry }: { onRetry: () => void }) {
  return (
    <Alert variant="warning" className="rounded-none border-x-0 border-t-0 py-1.5" role="status">
      <AlertCircle aria-hidden />
      <AlertDescription className="flex items-center gap-2" data-testid="changes-refresh-failed">
        Couldn't refresh
        <span aria-hidden className="text-faint">
          ·
        </span>
        <Button type="button" variant="link" size="xs" className="h-auto px-0" onClick={onRetry}>
          Retry
        </Button>
      </AlertDescription>
    </Alert>
  );
}

/** Large mode: one file at a time with steppers (US-009.AC-1, EC-2). */
export function ChangesLargeBanner({
  index,
  count,
  onStep,
}: {
  index: number;
  count: number;
  onStep: (delta: -1 | 1) => void;
}) {
  return (
    <Alert variant="info" className="rounded-none border-x-0 border-t-0 py-1.5" role="status">
      <Layers aria-hidden />
      <AlertDescription className="flex items-center gap-2" data-testid="changes-large-banner">
        <span className="min-w-0 flex-1 truncate text-fg">
          This diff is large, showing one file at a time
        </span>
        <span className="shrink-0 tabular-nums" data-testid="changes-large-position">
          {formatChangeCount(index + 1)} of {formatChangeCount(count)}
        </span>
        <ChangesIconAction
          label="Previous file"
          testId="changes-previous-file"
          disabled={index <= 0}
          onClick={() => onStep(-1)}
        >
          <ChevronLeft aria-hidden className="size-3.5" />
        </ChangesIconAction>
        <ChangesIconAction
          label="Next file"
          testId="changes-next-file"
          disabled={index >= count - 1}
          onClick={() => onStep(1)}
        >
          <ChevronRight aria-hidden className="size-3.5" />
        </ChangesIconAction>
      </AlertDescription>
    </Alert>
  );
}

/** More than 3,000 changed files: the list stops, the totals don't (US-009.EC-1). */
export function ChangesTruncatedNote({ shown, total }: { shown: number; total: number }) {
  return (
    <p
      className="shrink-0 border-b border-line-soft px-3 py-1.5 text-xs text-muted-foreground"
      data-testid="changes-truncated"
    >
      Showing {formatChangeCount(shown)} of {formatChangeCount(total)} files
    </p>
  );
}
