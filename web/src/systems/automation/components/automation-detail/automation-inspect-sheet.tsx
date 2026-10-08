import { useState } from "react";
import { Search } from "lucide-react";

import {
  CodeBlock,
  Eyebrow,
  KindIcon,
  LaneTabs,
  MetadataTile,
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
  StatusDot,
  TabsContent,
} from "@compozy/ui";

import { isAutomationTrigger, type AutomationEntity } from "../../lib/automation-detail";
import {
  automationInspectDescription,
  buildAutomationDiagnostics,
  buildAutomationDiagnosticsNote,
  formatSchedulerState,
  formatTriggerEnvelope,
} from "../../lib/automation-inspect-model";

type InspectPane = "diagnostics" | "raw";

interface AutomationInspectSheetProps {
  entity: AutomationEntity;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

/**
 * The machine truth, on request.
 *
 * The detail page speaks plain language; everything an operator needs when
 * that is not enough — the daemon's kind and enums, scheduler bookkeeping, and
 * the raw scheduler state or a sample event — lives here instead.
 */
export function AutomationInspectSheet({
  entity,
  open,
  onOpenChange,
}: AutomationInspectSheetProps) {
  const [pane, setPane] = useState<InspectPane>("diagnostics");
  const tiles = buildAutomationDiagnostics(entity);
  const isTrigger = isAutomationTrigger(entity);
  const tabs = [
    { value: "diagnostics", label: "Diagnostics", testId: "automation-inspect-tab-diagnostics" },
    {
      value: "raw",
      label: isTrigger ? "Sample event" : "Scheduler state",
      testId: "automation-inspect-tab-raw",
    },
  ] as const satisfies ReadonlyArray<{ value: InspectPane; label: string; testId: string }>;

  return (
    <Sheet onOpenChange={onOpenChange} open={open}>
      <SheetContent
        className="w-150 max-w-[calc(100%-1.5rem)] gap-0 sm:max-w-[calc(100%-1.5rem)]"
        data-testid="automation-inspect-sheet"
        side="right"
      >
        <SheetHeader className="flex-row items-start gap-3 border-b border-line p-5">
          <KindIcon aria-hidden="true" className="mt-0.5" icon={Search} tone="well" />
          <div className="flex min-w-0 flex-col gap-0.5">
            <Eyebrow className="text-subtle">Operator</Eyebrow>
            <SheetTitle className="text-item-title font-medium tracking-tight text-fg-strong">
              Inspect
            </SheetTitle>
            <SheetDescription className="text-small-body text-muted">
              {automationInspectDescription(entity)}
            </SheetDescription>
          </div>
        </SheetHeader>

        <LaneTabs
          ariaLabel="Inspect panes"
          className="min-h-0 flex-1 gap-0"
          items={tabs}
          listClassName="px-5"
          onChange={setPane}
          value={pane}
        >
          <TabsContent
            className="min-h-0 overflow-y-auto px-5 py-4.5"
            data-testid="automation-inspect-diagnostics"
            value="diagnostics"
          >
            <div className="grid gap-2.5 sm:grid-cols-2">
              {tiles.map(tile => (
                <MetadataTile
                  data-testid={`automation-inspect-tile-${tile.id}`}
                  key={tile.id}
                  label={tile.label}
                  value={
                    tile.tone ? (
                      <span className="inline-flex items-center gap-1.5">
                        <StatusDot size="sm" tone={tile.tone} />
                        {tile.value}
                      </span>
                    ) : (
                      tile.value
                    )
                  }
                />
              ))}
            </div>
            <p className="mt-3.5 text-form-label leading-relaxed text-muted">
              {buildAutomationDiagnosticsNote(entity)}
            </p>
          </TabsContent>
          <TabsContent
            className="min-h-0 overflow-y-auto px-5 py-4.5"
            data-testid="automation-inspect-raw"
            value="raw"
          >
            <CodeBlock
              className="bg-rail"
              code={isTrigger ? formatTriggerEnvelope(entity) : formatSchedulerState(entity)}
              copyable
              copyLabel={isTrigger ? "Copy sample event" : "Copy scheduler state"}
              density="compact"
            />
          </TabsContent>
        </LaneTabs>
      </SheetContent>
    </Sheet>
  );
}
