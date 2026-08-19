export interface LoopEditorNodeActions {
  readOnly: boolean;
  canPaste: boolean;
  onDuplicate: (nodeId: string) => void;
  onCopy: (nodeId: string) => void;
  onPaste: () => void;
  onRename: (nodeId: string) => void;
  onDelete: (nodeId: string) => void;
}

export interface LoopEditorConnectionDrop {
  source: string;
  point: { x: number; y: number };
  position: { x: number; y: number };
}

export type LoopEditorPaletteMode = "expanded" | "collapsed" | "menu";
