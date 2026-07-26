import { Plus, Trash2 } from "lucide-react";
import { useState, type ComponentPropsWithoutRef } from "react";

import { Button, cn, ConfirmDialog, MonoId, Time } from "@agh/ui";

import type {
  TaskBridgeNotificationSubscription,
  TaskBridgeNotificationSubscriptionCreateRequest,
} from "../types";
import { TaskBridgeSubscriptionCreateDialog } from "./task-bridge-subscription-create-dialog";
import { TaskRowsLoadingSkeleton } from "./task-loading-skeletons";

export interface TaskBridgeSubscriptionsPaneProps {
  workspaceId: string | null;
  subscriptions: readonly TaskBridgeNotificationSubscription[];
  isLoading?: boolean;
  errorMessage?: string | null;
  isCreatePending?: boolean;
  isDeletePending?: boolean;
  onCreate: (request: TaskBridgeNotificationSubscriptionCreateRequest) => Promise<void> | void;
  onDelete: (subscriptionId: string) => Promise<void> | void;
}

interface SubscriptionRowProps extends ComponentPropsWithoutRef<"div"> {
  subscription: TaskBridgeNotificationSubscription;
  isDeletePending: boolean;
  onDelete: (subscriptionId: string) => Promise<void> | void;
}

function SubscriptionRow({
  subscription,
  isDeletePending,
  onDelete,
  className,
  ...props
}: SubscriptionRowProps) {
  const [confirmOpen, setConfirmOpen] = useState(false);
  const handleDelete = async () => {
    try {
      await onDelete(subscription.subscription_id);
      setConfirmOpen(false);
    } catch {
      // The mutation helper already reports the failure. Keep confirmation open.
    }
  };

  return (
    <>
      <div
        {...props}
        className={cn(
          "grid grid-cols-[minmax(0,1fr)_auto] items-start gap-3 rounded-md border border-line-soft bg-input-fill px-3.5 py-3",
          className
        )}
        data-testid={`tasks-bridges-row-${subscription.subscription_id}`}
      >
        <div className="min-w-0">
          <MonoId value={subscription.subscription_id} />
          <p className="mt-1 text-form-label leading-relaxed text-muted">
            Delivers task events to bridge{" "}
            <span className="font-mono text-eyebrow text-fg">
              {subscription.bridge_instance_id}
            </span>{" "}
            · scope {subscription.scope} · mode {subscription.delivery_mode}
          </p>
          <p className="mt-1 font-mono text-micro text-subtle">
            cursor seq {subscription.cursor.last_sequence}
            {subscription.cursor.last_delivered_at ? (
              <>
                {" "}
                · delivered <Time iso={subscription.cursor.last_delivered_at} mode="relative" />
              </>
            ) : null}
            {subscription.cursor.last_error
              ? ` · last error: ${subscription.cursor.last_error}`
              : null}
          </p>
        </div>
        <Button
          aria-label={`Remove subscription ${subscription.subscription_id}`}
          className="size-6"
          data-testid={`tasks-bridges-delete-${subscription.subscription_id}`}
          disabled={isDeletePending}
          onClick={() => setConfirmOpen(true)}
          size="icon-sm"
          type="button"
          variant="ghost"
        >
          <Trash2 aria-hidden="true" className="size-3.5" />
        </Button>
      </div>
      <ConfirmDialog
        cancelLabel="Keep subscription"
        confirmLabel={isDeletePending ? "Removing…" : "Remove subscription"}
        description={`Task events will stop being delivered to bridge ${subscription.bridge_instance_id}.`}
        isPending={isDeletePending}
        onConfirm={handleDelete}
        onOpenChange={setConfirmOpen}
        open={confirmOpen}
        title="Remove bridge subscription?"
        tone="danger"
      />
    </>
  );
}

/**
 * Inspect drawer › Bridges pane: operator-facing bridge notification
 * subscriptions. All eight create fields stay — this is operator turf.
 *
 * @see docs/design/opendesign/tasks/TASK-DETAILS-REDESIGN-PLAN.md §4.8
 */
export function TaskBridgeSubscriptionsPane({
  workspaceId,
  subscriptions,
  isLoading = false,
  errorMessage = null,
  isCreatePending = false,
  isDeletePending = false,
  onCreate,
  onDelete,
}: TaskBridgeSubscriptionsPaneProps) {
  const [createOpen, setCreateOpen] = useState(false);

  if (isLoading && subscriptions.length === 0) {
    return <TaskRowsLoadingSkeleton label="Loading subscriptions" rows={2} />;
  }

  return (
    <div className="flex flex-col gap-3" data-testid="tasks-bridges-pane">
      {errorMessage ? (
        <p className="text-small-body text-danger" role="alert">
          {errorMessage}
        </p>
      ) : null}
      {subscriptions.length === 0 && !errorMessage ? (
        <p className="text-small-body text-muted">
          No bridge subscriptions. Task events stay inside the runtime until a bridge subscribes.
        </p>
      ) : (
        subscriptions.map(subscription => (
          <SubscriptionRow
            isDeletePending={isDeletePending}
            key={subscription.subscription_id}
            onDelete={onDelete}
            subscription={subscription}
          />
        ))
      )}

      <div>
        <Button
          data-testid="tasks-bridges-add"
          onClick={() => setCreateOpen(true)}
          size="sm"
          type="button"
          variant="neutral"
        >
          <Plus aria-hidden="true" className="size-3" />
          Add subscription
        </Button>
      </div>

      <TaskBridgeSubscriptionCreateDialog
        isPending={isCreatePending}
        onCreate={onCreate}
        onOpenChange={setCreateOpen}
        open={createOpen}
        workspaceId={workspaceId}
      />
    </div>
  );
}
