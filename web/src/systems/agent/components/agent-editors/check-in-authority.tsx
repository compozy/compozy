/**
 * U7: standing consent for unattended check-ins. A block that waits says so with Approve; a save
 * that changes the block shows it and approves only on confirm.
 */
import { ShieldAlert } from "@compozy/ui/icons";

import {
  Alert,
  AlertActions,
  AlertDescription,
  AlertTitle,
  Button,
  ConfirmDialog,
} from "@compozy/ui";

import type { CheckInsEditorModel } from "../../hooks/use-check-ins-editor";
import type { HeartbeatAuthority } from "../../hooks/use-check-in-authority";

function caps(checkIns: HeartbeatAuthority["check_ins"]) {
  const parts = [
    typeof checkIns.max_tool_calls === "number" ? `${checkIns.max_tool_calls} tool calls` : "",
    typeof checkIns.max_write_calls === "number" ? `${checkIns.max_write_calls} changes` : "",
  ].filter(Boolean);
  return parts.length > 0 ? `Up to ${parts.join(" and ")} per check-in` : null;
}

/** The authority block as the operator approves it: writes, caps and auto-accepted actions. */
export function AuthorityBlock({ authority }: { authority: HeartbeatAuthority }) {
  const writes = authority.check_ins.allow_writes ?? [];
  const limit = caps(authority.check_ins);
  const auto = authority.actions.filter(action => action.auto_accept);
  return (
    <dl className="flex flex-col gap-2 text-small-body" data-testid="check-in-authority-block">
      <div className="flex flex-col gap-0.5">
        <dt className="font-medium text-fg">What it can change</dt>
        <dd className="text-muted-foreground">
          {writes.length > 0 ? writes.join(", ") : "Nothing, reads only"}
        </dd>
        {limit ? <dd className="text-muted-foreground">{limit}</dd> : null}
      </div>
      {auto.length > 0 ? (
        <div className="flex flex-col gap-0.5">
          <dt className="font-medium text-fg">Done without asking you</dt>
          {auto.map(action => (
            <dd className="text-muted-foreground" key={action.id}>
              <span className="text-fg">{action.title}</span>: {action.instructions}
            </dd>
          ))}
        </div>
      ) : null}
    </dl>
  );
}

/** "Check-in changes need your approval", with the block and Approve; nothing once approved. */
export function CheckInAuthorityNotice({ model }: { model: CheckInsEditorModel }) {
  const { current, approve, approving } = model.authority;
  if (current?.status !== "unapproved") return null;
  return (
    <Alert aria-label="Check-in changes need your approval" role="region" variant="warning">
      <ShieldAlert aria-hidden="true" />
      <AlertTitle>Check-in changes need your approval</AlertTitle>
      <AlertDescription className="flex flex-col gap-3">
        <p>Until you approve them, check-ins only read and every proposal waits for you.</p>
        <AuthorityBlock authority={current} />
      </AlertDescription>
      <AlertActions>
        <Button
          aria-busy={approving || undefined}
          disabled={approving}
          onClick={approve}
          size="sm"
          type="button"
        >
          Approve
        </Button>
      </AlertActions>
    </Alert>
  );
}

/** Asked by a save that changes the block: approve it with the save, or save it unapproved. */
export function AuthoritySaveConfirm({ model }: { model: CheckInsEditorModel }) {
  const { pending } = model.authority;
  return (
    <ConfirmDialog
      body={pending ? <AuthorityBlock authority={pending.authority} /> : null}
      cancelLabel="Save without approving"
      confirmLabel="Approve and save"
      description="Check-ins run unattended. Approving lets them do this without asking you."
      onConfirm={() => pending?.settle(true)}
      onOpenChange={open => {
        if (!open) pending?.settle(false);
      }}
      open={pending !== null}
      title="Approve these check-in changes?"
      tone="neutral"
    />
  );
}
