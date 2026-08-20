import { useState } from "react";
import { Search } from "lucide-react";

import {
  CodeBlock,
  Eyebrow,
  LaneTabs,
  MetadataTile,
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
  TabsContent,
} from "@compozy/ui";

import {
  buildTriggerDiagnostics,
  buildTriggerDiagnosticsNote,
  formatTriggerEnvelope,
  triggerInspectDescription,
} from "../../lib/trigger-inspect-model";
import type { AutomationTrigger } from "../../types";

type InspectPane = "diagnostics" | "envelope";

const INSPECT_TABS = [
  { value: "diagnostics", label: "Diagnostics", testId: "trigger-inspect-tab-diagnostics" },
  { value: "envelope", label: "Sample envelope", testId: "trigger-inspect-tab-envelope" },
] as const satisfies ReadonlyArray<{ value: InspectPane; label: string; testId: string }>;

interface TriggerInspectSheetProps {
  trigger: AutomationTrigger;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

/**
 * The raw read, on request.
 *
 * The detail page speaks plain language; everything an operator needs when that
 * is not enough — runtime enums, dispatch internals, and a reconstructed sample
 * envelope — lives here instead of
 * crowding the page that answers "is this on and did it work".
 */
export function TriggerInspectSheet({ trigger, open, onOpenChange }: TriggerInspectSheetProps) {
  const [pane, setPane] = useState<InspectPane>("diagnostics");
  const tiles = buildTriggerDiagnostics(trigger);

  return (
    <Sheet onOpenChange={onOpenChange} open={open}>
      <SheetContent
        className="w-150 max-w-[calc(100%-1.5rem)] gap-0 bg-canvas sm:max-w-[calc(100%-1.5rem)]"
        data-testid="trigger-inspect-sheet"
        side="right"
      >
        <SheetHeader className="flex-row items-start gap-3 border-b border-line p-5">
          <span
            aria-hidden="true"
            className="mt-0.5 flex size-9 shrink-0 items-center justify-center rounded-md border border-line bg-canvas-soft text-subtle"
          >
            <Search className="size-4" strokeWidth={1.75} />
          </span>
          <div className="flex min-w-0 flex-col gap-0.5">
            <Eyebrow className="text-subtle">Operator</Eyebrow>
            <SheetTitle className="text-item-title font-medium tracking-tight text-fg-strong">
              Inspect
            </SheetTitle>
            <SheetDescription className="text-small-body text-muted">
              {triggerInspectDescription(trigger)}
            </SheetDescription>
          </div>
        </SheetHeader>

        <LaneTabs
          ariaLabel="Inspect panes"
          className="min-h-0 flex-1 gap-0"
          items={INSPECT_TABS}
          listClassName="px-5"
          onChange={setPane}
          value={pane}
        >
          <TabsContent
            className="min-h-0 overflow-y-auto px-5 py-4.5"
            data-testid="trigger-inspect-diagnostics"
            value="diagnostics"
          >
            <div className="grid gap-2.5 sm:grid-cols-2">
              {tiles.map(tile => (
                <MetadataTile
                  key={tile.id}
                  label={tile.label}
                  value={tile.value}
                  data-testid={`trigger-inspect-tile-${tile.id}`}
                />
              ))}
            </div>
            <p className="mt-3.5 text-form-label leading-relaxed text-muted">
              {buildTriggerDiagnosticsNote(trigger)}
            </p>
          </TabsContent>
          <TabsContent
            className="min-h-0 overflow-y-auto px-5 py-4.5"
            data-testid="trigger-inspect-envelope"
            value="envelope"
          >
            <CodeBlock
              className="bg-rail"
              code={formatTriggerEnvelope(trigger)}
              copyable
              copyLabel="Copy sample envelope"
              density="compact"
            />
          </TabsContent>
        </LaneTabs>
      </SheetContent>
    </Sheet>
  );
}
