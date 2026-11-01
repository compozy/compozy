import { INTERNAL, useAui } from "@assistant-ui/react";
import { DirectiveNode } from "@assistant-ui/react-lexical";
import { useLexicalComposerContext } from "@lexical/react/LexicalComposerContext";
import {
  $createParagraphNode,
  $createTextNode,
  $getRoot,
  $getSelection,
  $isRangeSelection,
  $isTextNode,
  COMMAND_PRIORITY_CRITICAL,
  COMMAND_PRIORITY_HIGH,
  KEY_ARROW_DOWN_COMMAND,
  KEY_ARROW_UP_COMMAND,
  KEY_ENTER_COMMAND,
  KEY_ESCAPE_COMMAND,
  KEY_TAB_COMMAND,
  PASTE_COMMAND,
  type BaseSelection,
  type EditorState,
  type UpdateListenerPayload,
} from "lexical";
import { useEffect, useEffectEvent } from "react";

import { readCursorSnapshot, type EditorCursorSnapshot } from "./session-composer-cursor-snapshot";
import { detectMentionTrigger, type SessionMentionTrigger } from "./session-mention-model";

export interface SessionComposerInputHandle {
  clear: () => void;
  focus: () => void;
}

/**
 * Exposes an imperative focus handle for prefill flows outside the editor.
 */
export function SessionComposerHandleBridge({
  onHandle,
}: {
  onHandle: (handle: SessionComposerInputHandle | null) => void;
}) {
  const [editor] = useLexicalComposerContext();
  const publishHandle = useEffectEvent(onHandle);

  useEffect(() => {
    publishHandle({
      clear: () => {
        // Let assistant-ui's pending runtime sync finish before the empty editor state wins.
        queueMicrotask(() => {
          editor.update(
            () => {
              const root = $getRoot();
              root.clear();
              root.append($createParagraphNode());
              root.selectEnd();
            },
            { discrete: true }
          );
        });
      },
      focus: () => editor.focus(),
    });
    return () => publishHandle(null);
  }, [editor]);

  return null;
}

/**
 * Busy-turn keyboard contract, registered above the library's submit handler
 * (which hard-blocks Enter while running and drops every Shift+Enter combo):
 * a plain Enter performs the operator's follow-up default and Cmd/Ctrl+Enter
 * performs the opposite for that one send (ADR-002). Shift+Enter still breaks
 * the line. The command popover wins when open — its key plugin consumes
 * Enter first via the delegation pass.
 *
 * A send blocker (a file still saving, a rejected or unsupported file) holds
 * Enter exactly like the disabled Send control, idle or busy, and answers with
 * its reason instead of leaving silently (issue #815).
 */
export function SessionBusyEnterPlugin({
  active,
  blocker = null,
  onBlocked,
  onEnter,
}: {
  active: boolean;
  blocker?: string | null;
  onBlocked?: (reason: string) => void;
  onEnter: (variant: "default" | "opposite") => void;
}) {
  const [editor] = useLexicalComposerContext();
  const pluginRegistry = INTERNAL.useComposerInputPluginRegistryOptional();
  const enter = useEffectEvent(onEnter);
  const blocked = useEffectEvent((reason: string) => onBlocked?.(reason));
  const blocking = blocker !== null;

  useEffect(() => {
    if (!active && !blocking) return;
    return editor.registerCommand(
      KEY_ENTER_COMMAND,
      event => {
        if (!event || event.isComposing) return false;
        if (event.shiftKey) return false;
        const opposite = event.ctrlKey || event.metaKey;
        if (!opposite && pluginRegistry) {
          for (const plugin of pluginRegistry.getPlugins()) {
            if (plugin.handleKeyDown(event)) return true;
          }
        }
        event.preventDefault();
        if (blocker !== null) {
          blocked(blocker);
          return true;
        }
        enter(opposite ? "opposite" : "default");
        return true;
      },
      COMMAND_PRIORITY_CRITICAL
    );
  }, [editor, pluginRegistry, active, blocking, blocker]);

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

type ComposerCursorRead =
  /** Neither the text nor the selection moved since the previous update. */
  | { changed: false }
  /** `snapshot` is null without a collapsed, readable caret. */
  | { changed: true; snapshot: EditorCursorSnapshot | null };

const cursorReads = new WeakMap<EditorState, ComposerCursorRead>();

function sameSelection(a: BaseSelection | null, b: BaseSelection | null): boolean {
  return a === b || (a !== null && b !== null && a.is(b));
}

/**
 * One text/caret read per committed editor state, shared by the `/` scope and
 * `@` trigger detectors: the first listener walks the document, the next one
 * reuses the result. An update that dirtied no node and kept the selection
 * (Lexical marks the root dirty for `setEditorState`) reports `changed: false`
 * so both detectors skip it — their previous answer still holds.
 */
function readComposerCursor({
  dirtyElements,
  dirtyLeaves,
  editorState,
  prevEditorState,
}: UpdateListenerPayload): ComposerCursorRead {
  const cached = cursorReads.get(editorState);
  if (cached) return cached;
  const selection = editorState.read(() => $getSelection());
  let read: ComposerCursorRead;
  if (
    dirtyElements.size === 0 &&
    dirtyLeaves.size === 0 &&
    sameSelection(
      prevEditorState.read(() => $getSelection()),
      selection
    )
  ) {
    read = { changed: false };
  } else if (!$isRangeSelection(selection) || !selection.isCollapsed()) {
    read = { changed: true, snapshot: null };
  } else {
    read = { changed: true, snapshot: editorState.read(() => readCursorSnapshot(selection)) };
  }
  cursorReads.set(editorState, read);
  return read;
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
    return editor.registerUpdateListener(update => {
      const read = readComposerCursor(update);
      if (!read.changed || !read.snapshot) return;
      const { text, cursor } = read.snapshot;
      const triggerOffset = detectSlashTriggerOffset(text, cursor);
      if (triggerOffset === null) {
        updateScope("inline");
        return;
      }
      const prefix = text.slice(0, triggerOffset);
      updateScope(prefix.trim().length === 0 ? "standalone" : "inline");
    });
  }, [editor]);

  return null;
}

/**
 * Publishes the live `@` occurrence before the caret (or `null`) so the
 * mention picker searches what the operator is typing, and lets Esc dismiss
 * that occurrence — above the library's handlers, so the `@` stays closed
 * while typing continues after it and when the caret returns to it
 * (US-004.AC-1). The full text travels with each report so a dismissal
 * follows its `@` across edits elsewhere. The library forwards arrows and Tab
 * to the picker even during IME composition, so they stop here and composition
 * always wins (US-004.EC-1).
 */
export function SessionMentionTriggerPlugin({
  onTrigger,
  onEscape,
}: {
  /** `text` is the full composer text when the editor selection could be read. */
  onTrigger: (trigger: SessionMentionTrigger | null, text?: string) => void;
  /** Returns true when an open picker consumed the key. */
  onEscape: () => boolean;
}) {
  const [editor] = useLexicalComposerContext();
  const publish = useEffectEvent(onTrigger);
  const escape = useEffectEvent(onEscape);

  useEffect(() => {
    const unregisterUpdate = editor.registerUpdateListener(update => {
      const read = readComposerCursor(update);
      if (!read.changed) return;
      if (!read.snapshot) {
        publish(null);
        return;
      }
      publish(detectMentionTrigger(read.snapshot.text, read.snapshot.cursor), read.snapshot.text);
    });
    const unregisterEscape = editor.registerCommand(
      KEY_ESCAPE_COMMAND,
      event => {
        if (!escape()) return false;
        event?.preventDefault();
        return true;
      },
      COMMAND_PRIORITY_CRITICAL
    );
    const yieldToComposition = (event: KeyboardEvent | null) =>
      event !== null && (event.isComposing || event.keyCode === 229);
    const unregisterCompositionKeys = [
      KEY_ARROW_DOWN_COMMAND,
      KEY_ARROW_UP_COMMAND,
      KEY_TAB_COMMAND,
    ].map(command =>
      editor.registerCommand(command, yieldToComposition, COMMAND_PRIORITY_CRITICAL)
    );
    return () => {
      for (const unregister of unregisterCompositionKeys) unregister();
      unregisterUpdate();
      unregisterEscape();
    };
  }, [editor]);

  return null;
}

/**
 * Clipboard files become composer attachments. Binary never enters the editor;
 * text/markdown clipboard content keeps the default paste path.
 */
export function SessionComposerPastePlugin() {
  const [editor] = useLexicalComposerContext();
  const aui = useAui();

  const addFiles = useEffectEvent((files: File[]) => {
    for (const file of files) {
      void aui.composer.addAttachment(file);
    }
  });

  useEffect(() => {
    return editor.registerCommand(
      PASTE_COMMAND,
      event => {
        if (!isClipboardEvent(event)) return false;
        const files = Array.from(event.clipboardData?.files ?? []);
        if (files.length === 0) return false;
        event.preventDefault();
        addFiles(files);
        return true;
      },
      COMMAND_PRIORITY_HIGH
    );
  }, [editor]);

  return null;
}

function isClipboardEvent(event: unknown): event is ClipboardEvent {
  return typeof event === "object" && event !== null && "clipboardData" in event;
}
