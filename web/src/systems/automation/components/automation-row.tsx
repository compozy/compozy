import type { ComponentProps } from "react";
import { Link } from "@tanstack/react-router";

import { ListingRow, cn } from "@compozy/ui";

import {
  automationLocationLabel,
  automationTimeStat,
  type AutomationView,
} from "../lib/automation-view";
import { AutomationRowSwitch } from "./automation-enable-switch";
import {
  AutomationBadges,
  AutomationLastRunText,
  AutomationOverflowMenu,
  AutomationPublicLinkText,
  AutomationSentenceText,
  AutomationStartGlyph,
  type AutomationOverflowHandlers,
} from "./automation-row-parts";
import { ProfileOwnerTag } from "@/systems/profiles";

export interface AutomationItemControls extends AutomationOverflowHandlers {
  /** Runtime unavailable: the switch and Run now are disabled. */
  unavailable: boolean;
  isRunPending: (view: AutomationView) => boolean;
  isTogglePending: (view: AutomationView) => boolean;
  onToggleEnabled: (view: AutomationView, enabled: boolean) => void;
}

interface AutomationRowProps {
  view: AutomationView;
  controls: AutomationItemControls;
}

/** Detail link for a view; routes keep the daemon entity. */
export function AutomationDetailLink({
  view,
  ...props
}: { view: AutomationView } & Omit<ComponentProps<"a">, "href">) {
  return view.kind === "job" ? (
    <Link params={{ jobId: view.id }} to="/automations/jobs/$jobId" {...props} />
  ) : (
    <Link params={{ triggerId: view.id }} to="/automations/triggers/$triggerId" {...props} />
  );
}

/** One automation as a row: kind glyph, name, sentence, meta, time stat, switch, overflow. */
export function AutomationRow({ view, controls }: AutomationRowProps) {
  const stat = automationTimeStat(view);
  const failed = view.lastRun?.status === "failed";
  return (
    <ListingRow
      data-automation-id={view.id}
      data-automation-kind={view.kind}
      data-testid={`automation-row-${view.kind}-${view.id}`}
    >
      <ListingRow.Link
        render={<AutomationDetailLink aria-label={`Open ${view.name}`} view={view} />}
      >
        <ListingRow.Icon>
          <AutomationStartGlyph start={view.start} />
        </ListingRow.Icon>
        <ListingRow.Main>
          <ListingRow.Name>
            <ListingRow.Title className={cn(!view.enabled && "text-muted")}>
              {view.name}
            </ListingRow.Title>
            <AutomationBadges view={view} />
          </ListingRow.Name>
          <ListingRow.Description data-testid={`automation-sentence-${view.id}`}>
            <AutomationSentenceText sentence={view.sentence} />
          </ListingRow.Description>
          <ListingRow.Meta>
            <span>{automationLocationLabel(view)}</span>
            {view.start === "webhook" && !failed ? (
              <>
                <ListingRow.MetaDot />
                <AutomationPublicLinkText view={view} />
              </>
            ) : view.lastRun ? (
              <>
                <ListingRow.MetaDot />
                <AutomationLastRunText view={view} />
              </>
            ) : null}
            {view.owner ? (
              <>
                <ListingRow.MetaDot />
                <ProfileOwnerTag data-testid={`automation-profile-${view.id}`} owner={view.owner} />
              </>
            ) : null}
          </ListingRow.Meta>
        </ListingRow.Main>
      </ListingRow.Link>
      <ListingRow.Trail>
        <ListingRow.Stat data-testid={`automation-stat-${view.id}`}>
          <ListingRow.Stat.Value className={cn(stat.value === null && "text-faint")}>
            {stat.value ?? "—"}
          </ListingRow.Stat.Value>
          <ListingRow.Stat.Label>{stat.label}</ListingRow.Stat.Label>
        </ListingRow.Stat>
        <AutomationRowSwitch
          data-testid={`automation-switch-${view.id}`}
          disabled={controls.unavailable}
          enabled={view.enabled}
          name={view.name}
          onEnabledChange={enabled => controls.onToggleEnabled(view, enabled)}
          pending={controls.isTogglePending(view)}
          size="sm"
        />
        <AutomationOverflowMenu
          onCopyLink={controls.onCopyLink}
          onDelete={controls.onDelete}
          onEdit={controls.onEdit}
          onRunNow={controls.onRunNow}
          runDisabled={controls.unavailable}
          runPending={controls.isRunPending(view)}
          view={view}
        />
      </ListingRow.Trail>
    </ListingRow>
  );
}
