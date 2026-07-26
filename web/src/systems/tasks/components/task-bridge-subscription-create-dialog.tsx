import { useState } from "react";

import {
  Button,
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  Field,
  FieldGroup,
  FieldLabel,
  Input,
  NativeSelect,
  NativeSelectOption,
} from "@agh/ui";

import type { TaskBridgeNotificationSubscriptionCreateRequest } from "../types";

interface FormState {
  bridge_instance_id: string;
  delivery_mode: TaskBridgeNotificationSubscriptionCreateRequest["delivery_mode"];
  scope: TaskBridgeNotificationSubscriptionCreateRequest["scope"];
  peer_id: string;
  group_id: string;
  thread_id: string;
  subscription_id: string;
}

function createInitialForm(workspaceId: string | null): FormState {
  return {
    bridge_instance_id: "",
    delivery_mode: "direct-send",
    scope: workspaceId ? "workspace" : "global",
    peer_id: "",
    group_id: "",
    thread_id: "",
    subscription_id: "",
  };
}

function toRequest(
  form: FormState,
  workspaceId: string | null
): TaskBridgeNotificationSubscriptionCreateRequest {
  return {
    bridge_instance_id: form.bridge_instance_id.trim(),
    delivery_mode: form.delivery_mode,
    scope: form.scope,
    workspace_id: form.scope === "workspace" ? (workspaceId ?? undefined) : undefined,
    peer_id: form.peer_id.trim() || undefined,
    group_id: form.group_id.trim() || undefined,
    thread_id: form.thread_id.trim() || undefined,
    subscription_id: form.subscription_id.trim() || undefined,
  };
}

export interface TaskBridgeSubscriptionCreateDialogProps {
  workspaceId: string | null;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  isPending: boolean;
  onCreate: (request: TaskBridgeNotificationSubscriptionCreateRequest) => Promise<void> | void;
}

export function TaskBridgeSubscriptionCreateDialog({
  workspaceId,
  open,
  onOpenChange,
  isPending,
  onCreate,
}: TaskBridgeSubscriptionCreateDialogProps) {
  const [form, setForm] = useState<FormState>(() => createInitialForm(workspaceId));
  const hasRequiredScopeId = form.scope !== "workspace" || workspaceId !== null;
  const canSubmit = form.bridge_instance_id.trim() !== "" && hasRequiredScopeId && !isPending;

  const submit = async () => {
    if (!canSubmit) return;
    try {
      await onCreate(toRequest(form, workspaceId));
      setForm(createInitialForm(workspaceId));
      onOpenChange(false);
    } catch {
      // The mutation helper reports the failure; retain the entered request for correction.
    }
  };

  return (
    <Dialog onOpenChange={onOpenChange} open={open}>
      <DialogContent className="max-w-md" data-testid="tasks-bridges-create-dialog">
        <DialogHeader>
          <DialogTitle>Add bridge subscription</DialogTitle>
        </DialogHeader>
        <FieldGroup>
          <Field>
            <FieldLabel htmlFor="tasks-bridges-instance">Bridge instance id</FieldLabel>
            <Input
              data-testid="tasks-bridges-instance"
              id="tasks-bridges-instance"
              onChange={event =>
                setForm(previous => ({ ...previous, bridge_instance_id: event.target.value }))
              }
              placeholder="bridge_9f2c1e"
              value={form.bridge_instance_id}
            />
          </Field>
          <div className="grid grid-cols-2 gap-3">
            <Field>
              <FieldLabel htmlFor="tasks-bridges-mode">Delivery mode</FieldLabel>
              <NativeSelect
                id="tasks-bridges-mode"
                onChange={event =>
                  setForm(previous => ({
                    ...previous,
                    delivery_mode: event.target.value as FormState["delivery_mode"],
                  }))
                }
                value={form.delivery_mode}
              >
                <NativeSelectOption value="direct-send">direct-send</NativeSelectOption>
                <NativeSelectOption value="reply">reply</NativeSelectOption>
              </NativeSelect>
            </Field>
            <Field>
              <FieldLabel htmlFor="tasks-bridges-scope">Scope</FieldLabel>
              <NativeSelect
                id="tasks-bridges-scope"
                onChange={event =>
                  setForm(previous => ({
                    ...previous,
                    scope: event.target.value as FormState["scope"],
                  }))
                }
                value={form.scope}
              >
                {workspaceId ? (
                  <NativeSelectOption value="workspace">workspace</NativeSelectOption>
                ) : null}
                <NativeSelectOption value="global">global</NativeSelectOption>
              </NativeSelect>
            </Field>
          </div>
          <div className="grid grid-cols-2 gap-3">
            <Field>
              <FieldLabel htmlFor="tasks-bridges-workspace">
                Workspace id{form.scope === "workspace" ? "" : " (optional)"}
              </FieldLabel>
              <Input
                id="tasks-bridges-workspace"
                readOnly
                value={form.scope === "workspace" ? (workspaceId ?? "") : ""}
              />
            </Field>
            <Field>
              <FieldLabel htmlFor="tasks-bridges-peer">Peer id (optional)</FieldLabel>
              <Input
                id="tasks-bridges-peer"
                onChange={event =>
                  setForm(previous => ({ ...previous, peer_id: event.target.value }))
                }
                value={form.peer_id}
              />
            </Field>
          </div>
          <div className="grid grid-cols-2 gap-3">
            <Field>
              <FieldLabel htmlFor="tasks-bridges-group">Group id (optional)</FieldLabel>
              <Input
                id="tasks-bridges-group"
                onChange={event =>
                  setForm(previous => ({ ...previous, group_id: event.target.value }))
                }
                value={form.group_id}
              />
            </Field>
            <Field>
              <FieldLabel htmlFor="tasks-bridges-thread">Thread id (optional)</FieldLabel>
              <Input
                id="tasks-bridges-thread"
                onChange={event =>
                  setForm(previous => ({ ...previous, thread_id: event.target.value }))
                }
                value={form.thread_id}
              />
            </Field>
          </div>
          <Field>
            <FieldLabel htmlFor="tasks-bridges-subscription">Subscription id (optional)</FieldLabel>
            <Input
              id="tasks-bridges-subscription"
              onChange={event =>
                setForm(previous => ({ ...previous, subscription_id: event.target.value }))
              }
              placeholder="Generated when empty"
              value={form.subscription_id}
            />
          </Field>
        </FieldGroup>
        <DialogFooter className="gap-2">
          <Button
            disabled={isPending}
            onClick={() => onOpenChange(false)}
            size="sm"
            type="button"
            variant="neutral"
          >
            Cancel
          </Button>
          <Button
            data-testid="tasks-bridges-create-submit"
            disabled={!canSubmit}
            onClick={submit}
            size="sm"
            type="button"
          >
            {isPending ? "Adding…" : "Add subscription"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
