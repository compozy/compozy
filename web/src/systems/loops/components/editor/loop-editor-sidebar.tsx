import { LaneTabs, TabsContent, type LaneTabsItem } from "@agh/ui";

import type { EditableLoopContractField } from "../../lib/loop-editor-definition";
import type { EditorEdge, EditorNode } from "../../lib/codec";
import type { FieldPath, FieldSpec } from "../../lib/loop-node-schema";
import type { LoopEditorSidebarTab } from "../../hooks/use-loop-editor-state";
import type { LoopContract, LoopDefinition } from "../../types";
import { LoopEditorContract } from "./loop-editor-contract";
import { LoopEditorInspector } from "./loop-editor-inspector";

const SIDEBAR_TABS: ReadonlyArray<LaneTabsItem<LoopEditorSidebarTab>> = [
  { value: "contract", label: "Contract", testId: "loop-editor-tab-contract" },
  { value: "node", label: "Node", testId: "loop-editor-tab-node" },
];

interface LoopEditorSidebarProps {
  contract: LoopContract;
  contractDisabled: boolean;
  onChangeContract: (field: EditableLoopContractField, value: string) => void;
  node: EditorNode | null;
  fields: FieldSpec[];
  nodes: EditorNode[];
  edges: EditorEdge[];
  selectionKey: number;
  definition: Pick<LoopDefinition, "inputs" | "start">;
  inspectorDisabled: boolean;
  onChangeField: (path: FieldPath, value: unknown) => void;
  sidebarTab: LoopEditorSidebarTab;
  onSidebarTabChange: (tab: LoopEditorSidebarTab) => void;
}

/** Right-rail inspector: Contract outcome vs selected-node fields, one lane at a time. */
export function LoopEditorSidebar({
  contract,
  contractDisabled,
  onChangeContract,
  node,
  fields,
  nodes,
  edges,
  selectionKey,
  definition,
  inspectorDisabled,
  onChangeField,
  sidebarTab,
  onSidebarTabChange,
}: LoopEditorSidebarProps) {
  return (
    <aside
      className="flex min-h-0 flex-col border-l border-line bg-canvas"
      data-testid="loop-editor-sidebar"
    >
      <LaneTabs<LoopEditorSidebarTab>
        ariaLabel="Loop editor inspector"
        className="flex min-h-0 flex-1 flex-col gap-0"
        items={SIDEBAR_TABS}
        listClassName="w-full shrink-0 border-b border-line px-4"
        onChange={onSidebarTabChange}
        value={sidebarTab}
      >
        <div className="min-h-0 flex-1 overflow-y-auto">
          <TabsContent value="contract" className="mt-0">
            <LoopEditorContract
              contract={contract}
              disabled={contractDisabled}
              onChange={onChangeContract}
            />
          </TabsContent>
          <TabsContent value="node" className="mt-0 h-full">
            <LoopEditorInspector
              node={node}
              fields={fields}
              nodes={nodes}
              edges={edges}
              selectionKey={selectionKey}
              definition={definition}
              disabled={inspectorDisabled}
              onChange={onChangeField}
            />
          </TabsContent>
        </div>
      </LaneTabs>
    </aside>
  );
}
