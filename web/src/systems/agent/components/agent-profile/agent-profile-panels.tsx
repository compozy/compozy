/** The focused side panels: each edits one thing with the product's own control. */
import {
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
import type { AgentProfilePanel } from "../../lib/agent-profile";
import { AgentSettingsAccessSection } from "../agent-settings-access-section";
import { AgentSettingsMcpSection } from "../agent-settings-mcp-section";
import { AgentSettingsRuntimeSection } from "../agent-settings-runtime-section";
import { AgentPlanCard } from "./agent-profile-activity";

const TITLES: Record<AgentProfilePanel, string> = {
  model: "Model",
  tools: "What it can use",
};

function PanelBody({
  panel,
  page,
  provider,
  readOnly,
}: {
  panel: AgentProfilePanel;
  page: ReturnType<typeof useAgentSettingsPage>;
  provider: string | null;
  readOnly: boolean;
}) {
  const { agent, draft } = page;
  if (!agent || !draft) return null;
  const disabled = page.phase === "saving";
  const locked = readOnly || page.phase === "denied";
  if (panel === "tools") {
    return (
      <div className="flex flex-col gap-5">
        <AgentSettingsAccessSection
          disabled={disabled}
          draft={draft}
          onPatch={page.patchDraft}
          readOnly={locked}
        />
        <AgentSettingsMcpSection agent={agent} />
      </div>
    );
  }
  return (
    <AgentSettingsRuntimeSection
      agent={agent}
      disabled={disabled}
      draft={draft}
      errors={page.validation?.fields ?? {}}
      modelCatalogError={page.modelCatalogError}
      modelCatalogLoading={page.modelCatalogLoading}
      modelCatalogRefreshing={page.modelCatalogRefreshing}
      onOpenProviderSettings={page.onOpenProviderSettings}
      onPatch={page.patchDraft}
      onRefreshCatalog={page.onRefreshCatalog}
      providerOptions={page.providerOptions}
      providersLoading={page.providersLoading}
      readOnly={locked}
      runtimeModels={page.runtimeModels}
    >
      <AgentPlanCard provider={provider} />
    </AgentSettingsRuntimeSection>
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
  provider,
  panel,
  readOnly,
  onClose,
}: {
  name: string;
  label: string;
  owner?: string;
  provider: string | null;
  panel: AgentProfilePanel | null;
  readOnly: boolean;
  onClose: () => void;
}) {
  const page = useAgentSettingsPage({ name, section: panel === "tools" ? "access" : "runtime" });
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
            {panel ? (
              <PanelBody page={page} panel={panel} provider={provider} readOnly={readOnly} />
            ) : null}
          </div>
          {readOnly ? null : <PanelFoot onClose={onClose} page={page} />}
        </SheetContent>
      </Sheet>
      {page.unsavedGuardDialog}
    </>
  );
}
