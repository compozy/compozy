import { INTERNAL } from "@assistant-ui/react";
import { DirectiveNode } from "@assistant-ui/react-lexical";
import { useLexicalComposerContext } from "@lexical/react/LexicalComposerContext";
import {
  $createTextNode,
  $getRoot,
  $getSelection,
  $isElementNode,
  $isRangeSelection,
  $isTextNode,
  COMMAND_PRIORITY_CRITICAL,
  KEY_ENTER_COMMAND,
  type RangeSelection,
} from "lexical";
import { useEffect, useEffectEvent } from "react";

export interface SessionComposerInputHandle {
  focus: () => void;
}

/**
 * Exposes an imperative focus handle for prefill flows outside the editor and
 * labels the contenteditable itself — the library only spreads ARIA props on
 * its wrapper div, leaving the actual textbox unnamed.
 */
export function SessionComposerHandleBridge({
  onHandle,
  editableAriaLabel,
}: {
  onHandle: (handle: SessionComposerInputHandle | null) => void;
  editableAriaLabel: string;
}) {
  const [editor] = useLexicalComposerContext();
  const publishHandle = useEffectEvent(onHandle);

  useEffect(() => {
    publishHandle({ focus: () => editor.focus() });
    return () => publishHandle(null);
  }, [editor]);

  useEffect(() => {
    return editor.registerRootListener(rootElement => {
      rootElement?.setAttribute("aria-label", editableAriaLabel);
    });
  }, [editor, editableAriaLabel]);

  return null;
}

/**
 * Busy-turn keyboard contract, registered above the library's submit handler
 * (which hard-blocks Enter while running and drops every Shift+Enter combo):
 * a plain Enter queues the draft, and Cmd/Ctrl+Shift+Enter steers the active
 * turn. The command popover still wins when open — its key plugin consumes
 * Enter first via the delegation pass.
 */
export function SessionBusyEnterPlugin({
  queueActive,
  steerActive,
  onQueue,
  onSteer,
}: {
  queueActive: boolean;
  steerActive: boolean;
  onQueue: () => void;
  onSteer: () => void;
}) {
  const [editor] = useLexicalComposerContext();
  const pluginRegistry = INTERNAL.useComposerInputPluginRegistryOptional();
  const queue = useEffectEvent(onQueue);
  const steer = useEffectEvent(onSteer);

  useEffect(() => {
    if (!queueActive && !steerActive) return;
    return editor.registerCommand(
      KEY_ENTER_COMMAND,
      event => {
        if (!event || event.isComposing) return false;
        if (event.shiftKey && (event.ctrlKey || event.metaKey)) {
          if (!steerActive) return false;
          event.preventDefault();
          steer();
          return true;
        }
        if (event.shiftKey || event.ctrlKey || event.metaKey) return false;
        if (!queueActive) return false;
        if (pluginRegistry) {
          for (const plugin of pluginRegistry.getPlugins()) {
            if (plugin.handleKeyDown(event)) return true;
          }
        }
        event.preventDefault();
        queue();
        return true;
      },
      COMMAND_PRIORITY_CRITICAL
    );
  }, [editor, pluginRegistry, queueActive, steerActive]);

  return null;
}

/**
 * The daemon only parses a command token at a whitespace boundary, and an
 * unspaced chip keeps the slash trigger active (the popover would reopen while
 * typing). The library's DirectivePlugin inserts the chip without the trailing
 * space the core textarea flow adds, so this transform restores the boundary —
 * once per materialized node, so deleting the space (and then the chip) stays
 * possible.
 */
export function SessionDirectiveBoundaryPlugin() {
  const [editor] = useLexicalComposerContext();

  useEffect(() => {
    const boundedKeys = new Set<string>();
    return editor.registerNodeTransform(DirectiveNode, node => {
      if (boundedKeys.has(node.getKey())) return;
      boundedKeys.add(node.getKey());
      const next = node.getNextSibling();
      if (next === null) {
        const selection = $getSelection();
        const space = $createTextNode(" ");
        node.insertAfter(space);
        if ($isRangeSelection(selection) && selection.isCollapsed()) {
          const anchor = selection.anchor;
          const anchorNode = anchor.getNode();
          const caretSitsAfterChip =
            (anchor.type === "element" &&
              anchorNode === node.getParent() &&
              anchor.offset === node.getIndexWithinParent() + 1) ||
            (anchor.type === "text" && anchorNode === space && anchor.offset === 0);
          if (caretSitsAfterChip) space.select(1, 1);
        }
        return;
      }
      if ($isTextNode(next) && !next.getTextContent().startsWith(" ")) {
        node.insertAfter($createTextNode(" "));
      }
    });
  }, [editor]);

  return null;
}

const WHITESPACE_RE = /\s/u;

/** Offset of the active `/` trigger before the cursor, or null when none. */
function detectSlashTriggerOffset(text: string, cursor: number): number | null {
  const upToCursor = text.slice(0, cursor);
  for (let i = upToCursor.length - 1; i >= 0; i--) {
    const char = upToCursor[i]!;
    if (WHITESPACE_RE.test(char)) return null;
    if (char === "/") {
      if (i > 0 && !WHITESPACE_RE.test(upToCursor[i - 1]!)) continue;
      return i;
    }
  }
  return null;
}

interface EditorCursorSnapshot {
  text: string;
  cursor: number;
}

function readCursorSnapshot(selection: RangeSelection): EditorCursorSnapshot | null {
  const anchor = selection.anchor;
  const anchorNode = anchor.getNode();
  const root = $getRoot();
  let text = "";
  let cursor = -1;
  let paragraphIndex = 0;

  for (const paragraph of root.getChildren()) {
    if (paragraphIndex > 0) text += "\n";
    paragraphIndex += 1;
    if (!$isElementNode(paragraph)) continue;

    if (anchor.type === "element" && anchorNode === paragraph) {
      // Element anchor: offset counts children before the caret.
      const paragraphStart = text.length;
      let cursorInParagraph = 0;
      let childIndex = 0;
      for (const child of paragraph.getChildren()) {
        if (childIndex < anchor.offset) cursorInParagraph += child.getTextContent().length;
        text += child.getTextContent();
        childIndex += 1;
      }
      cursor = paragraphStart + cursorInParagraph;
      continue;
    }

    for (const child of paragraph.getChildren()) {
      if (anchor.type === "text" && child === anchorNode) {
        cursor = text.length + anchor.offset;
      }
      text += child.getTextContent();
    }
  }
  if (cursor < 0) return null;
  return { text, cursor };
}

/**
 * Derives the command-menu scope from the live editor selection: a `/`
 * trigger with only whitespace before it offers the full standalone catalog;
 * mid-text triggers offer inline skills only.
 */
export function SessionCommandScopePlugin({
  setScope,
}: {
  setScope: (scope: "inline" | "standalone") => void;
}) {
  const [editor] = useLexicalComposerContext();
  const updateScope = useEffectEvent(setScope);

  useEffect(() => {
    return editor.registerUpdateListener(({ editorState }) => {
      editorState.read(() => {
        const selection = $getSelection();
        if (!$isRangeSelection(selection) || !selection.isCollapsed()) return;
        const snapshot = readCursorSnapshot(selection);
        if (!snapshot) return;
        const triggerOffset = detectSlashTriggerOffset(snapshot.text, snapshot.cursor);
        if (triggerOffset === null) {
          updateScope("inline");
          return;
        }
        const prefix = snapshot.text.slice(0, triggerOffset);
        updateScope(prefix.trim().length === 0 ? "standalone" : "inline");
      });
    });
  }, [editor]);

  return null;
}
