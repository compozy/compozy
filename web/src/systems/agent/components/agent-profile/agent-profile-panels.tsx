/** The focused side panels: each edits one thing with the product's own control. */
import {
  Alert,
  AlertActions,
  AlertDescription,
  Button,
  Pill,
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from "@compozy/ui";

import { useAgentSettingsPage } from "../../hooks/use-agent-settings-page";
import { permissionsBlockedMessage } from "../../lib/agent-permissions";
import type { AgentProfilePanel } from "../../lib/agent-profile";
import { AgentSettingsAccessSection } from "../agent-settings-access-section";
import { AgentSettingsMcpSection } from "../agent-settings-mcp-section";

const TITLES: Record<AgentProfilePanel, string> = {
  tools: "What it can use",
};

function PanelBody({
  page,
  readOnly,
}: {
  page: ReturnType<typeof useAgentSettingsPage>;
  readOnly: boolean;
}) {
  const { agent, draft } = page;
  if (!agent || !draft) return null;
  return (
    <div className="flex flex-col gap-5">
      <AgentSettingsAccessSection
        disabled={page.phase === "saving"}
        draft={draft}
        onPatch={page.patchDraft}
        readOnly={readOnly || page.phase === "denied"}
      />
      <AgentSettingsMcpSection agent={agent} />
    </div>
  );
}

function PanelFoot({
  page,
  onClose,
}: {
  page: ReturnType<typeof useAgentSettingsPage>;
  onClose: () => void;
}) {
  const saving = page.phase === "saving";
  return (
    <SheetFooter className="flex-row items-center justify-end gap-2">
      {page.dirty ? (
        <span className="mr-auto">
          <Pill size="sm" tone="warning">
            Unsaved
          </Pill>
        </span>
      ) : null}
      <Button
        onClick={() => {
          page.onDiscard();
          onClose();
        }}
        size="sm"
        type="button"
        variant="ghost"
      >
        {page.dirty ? "Discard" : "Close"}
      </Button>
      <Button
        aria-busy={saving}
        disabled={!page.dirty || page.saveBlocked || saving}
        onClick={page.onSave}
        size="sm"
        title={page.saveBlockedCaption}
        type="button"
      >
        Save
      </Button>
    </SheetFooter>
  );
}

export function AgentProfilePanels({
  name,
  label,
  owner,
  panel,
  readOnly,
  onClose,
}: {
  name: string;
  label: string;
  owner?: string;
  panel: AgentProfilePanel | null;
  readOnly: boolean;
  onClose: () => void;
}) {
  const page = useAgentSettingsPage({ name, section: "access" });
  return (
    <>
      <Sheet
        onOpenChange={open => {
          if (open) return;
          page.onDiscard();
          onClose();
        }}
        open={panel !== null}
      >
        <SheetContent className="flex flex-col sm:max-w-xl" data-testid="agent-profile-panel">
          <SheetHeader>
            <SheetTitle>{panel ? TITLES[panel] : ""}</SheetTitle>
            <SheetDescription>
              {readOnly ? `${owner} looks after this agent, so this can't be changed here.` : label}
            </SheetDescription>
          </SheetHeader>
          <div className="min-h-0 flex-1 overflow-y-auto px-4">
            {panel && page.permissionsBlocked ? (
              <Alert
                className="mb-4"
                data-testid="agent-profile-panel-permissions-blocked"
                variant="warning"
              >
                <AlertDescription>
                  {permissionsBlockedMessage(page.draft?.legacyPermissions)}
                </AlertDescription>
                <AlertActions>
                  <Button
                    onClick={page.onOpenPermissions}
                    size="sm"
                    type="button"
                    variant="secondary"
                  >
                    Open Permissions
                  </Button>
                </AlertActions>
              </Alert>
            ) : null}
            {panel ? <PanelBody page={page} readOnly={readOnly} /> : null}
          </div>
          {readOnly ? null : <PanelFoot onClose={onClose} page={page} />}
        </SheetContent>
      </Sheet>
      {page.unsavedGuardDialog}
    </>
  );
}
