import { useEffect, useRef } from "react";
import { Bot } from "lucide-react";

import {
  Button,
  Dialog,
  DialogContent,
  DialogFooter,
  EntityDialogHeader,
  Pill,
  Spinner,
  cn,
  dialogShellClass,
} from "@compozy/ui";

import {
  AGENT_SETTINGS_SECTIONS,
  AgentSettingsPanels,
  type AgentSettingsSearch,
  type AgentSettingsSection,
  resolveAgentSettingsSearch,
  useAgentSettingsPage,
} from "@/systems/agent";

const SECTION_LABELS: Record<AgentSettingsSection, string> = {
  basics: "Basics",
  runtime: "Model",
  instructions: "Instructions",
  access: "Access",
  mcp: "MCP servers",
};

const SETTINGS_MODAL_CLASS = `text-fg flex flex-col overflow-hidden ${dialogShellClass("lg", { fill: true })}`;

export function AgentSettingsLocation({
  name,
  rawSearch,
}: {
  name: string;
  rawSearch: AgentSettingsSearch;
}) {
  const search = resolveAgentSettingsSearch(rawSearch);
  const page = useAgentSettingsPage({ name, section: search.section });
  const returnFocusRef = useRef<HTMLElement | null>(null);

  useEffect(() => {
    const previous = document.activeElement;
    if (previous instanceof HTMLElement) {
      returnFocusRef.current = previous;
    }
    return () => {
      returnFocusRef.current?.focus?.();
    };
  }, []);

  // Missing agent: keep detail's not-found; do not paint an empty modal over it.
  if (!page.agentLoading && (page.agentError || !page.agent || !page.draft)) {
    return null;
  }

  const handleOpenChange = (open: boolean) => {
    if (!open) {
      page.onBackToDetail();
    }
  };
  const saving = page.phase === "saving";

  return (
    <Dialog open onOpenChange={handleOpenChange}>
      <DialogContent
        unframed
        showCloseButton={false}
        className={SETTINGS_MODAL_CLASS}
        data-testid="agent-settings-dialog"
      >
        {page.unsavedGuardDialog}
        {page.deleteFlow.confirmDialog}

        <EntityDialogHeader
          className="shrink-0"
          closeLabel="Close settings"
          eyebrow="Agents"
          icon={Bot}
          onClose={() => page.onBackToDetail()}
          title={`Edit ${name}`}
        />

        {page.agentLoading || !page.agent || !page.draft ? (
          <div
            className="flex flex-1 items-center justify-center gap-2 px-6 py-10 text-muted"
            data-testid="agent-settings-loading"
          >
            <Spinner aria-hidden="true" />
            Loading settings…
          </div>
        ) : (
          <div
            className="flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden md:flex-row"
            data-testid="agent-settings-page"
          >
            <nav
              aria-label="Agent settings sections"
              className="flex w-full shrink-0 flex-wrap gap-1 border-b border-line px-3 py-2 md:w-settings-nav md:flex-col md:flex-nowrap md:border-r md:border-b-0 md:py-3"
              data-testid="agent-settings-section-nav"
            >
              {AGENT_SETTINGS_SECTIONS.map(section => {
                const isActive = search.section === section;
                return (
                  <button
                    key={section}
                    type="button"
                    data-testid={`agent-settings-nav-${section}`}
                    data-active={isActive ? "true" : "false"}
                    aria-current={isActive ? "page" : undefined}
                    onClick={() => page.setSection(section)}
                    className={cn(
                      "rounded-md px-3 py-2 text-left text-small-body font-medium tracking-tight transition-colors duration-base ease-out",
                      "w-auto shrink-0 md:w-full",
                      isActive
                        ? "bg-row-selected text-fg-strong"
                        : "text-muted hover:bg-hover hover:text-fg"
                    )}
                  >
                    <span className="whitespace-nowrap md:truncate">{SECTION_LABELS[section]}</span>
                  </button>
                );
              })}
            </nav>
            <div className="relative flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden">
              <AgentSettingsPanels
                section={search.section}
                agent={page.agent}
                draft={page.draft}
                validation={page.validation}
                phase={page.phase}
                error={page.error}
                onReloadAndRetry={() => {
                  void page.onReloadAndRetry();
                }}
                onPatch={page.patchDraft}
                onDelete={page.deleteFlow.openDialog}
                isDeleting={page.deleteFlow.isDeleting}
                providerOptions={page.providerOptions}
                providersLoading={page.providersLoading}
                runtimeModels={page.runtimeModels}
                modelCatalogLoading={page.modelCatalogLoading}
                modelCatalogRefreshing={page.modelCatalogRefreshing}
                modelCatalogError={page.modelCatalogError}
                onRefreshCatalog={page.onRefreshCatalog}
                onOpenProviderSettings={page.onOpenProviderSettings}
              />
            </div>
          </div>
        )}

        <DialogFooter variant="ruled" className="shrink-0 sm:justify-between">
          <div className="flex min-w-0 items-center gap-2">
            <p className="text-small-body text-muted" data-testid="agent-settings-footer-note">
              Changes apply to new sessions only.
            </p>
            {page.dirty ? (
              <Pill tone="warning" size="sm" data-testid="agent-settings-unsaved">
                Unsaved
              </Pill>
            ) : null}
          </div>
          <div className="flex items-center gap-2">
            <Button
              type="button"
              variant="ghost"
              size="sm"
              onClick={() => page.onBackToDetail()}
              data-testid="agent-settings-cancel"
            >
              Cancel
            </Button>
            <Button
              type="button"
              variant="default"
              size="sm"
              disabled={!page.dirty || page.saveBlocked || saving || page.agentLoading}
              aria-busy={saving}
              title={page.saveBlockedCaption}
              onClick={page.onSave}
              data-testid="agent-settings-save"
            >
              {saving ? <Spinner aria-hidden="true" /> : null}
              Save changes
            </Button>
          </div>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
