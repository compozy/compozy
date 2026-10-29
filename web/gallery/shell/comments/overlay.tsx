/**
 * Layer over the canvas iframe: pins of the current state's comments and, in comment mode, the
 * picker (hover highlight, click an element or drag a region) and the composer. It lives outside
 * the scaled frame so pins and the composer keep their size. It measures itself: the canvas size
 * is the frame's viewport and the scale is the layer's width over it.
 */
import type { CSSProperties } from "react";

import { cn } from "@compozy/ui";

import { useElementSize } from "#gallery/shell/use-element-size";
import { CommentComposer } from "#gallery/shell/comments/composer";
import type { Located } from "#gallery/shell/comments/dom";
import {
  commentNumber,
  type ElementTarget,
  type Rect,
  type ReviewComment,
} from "#gallery/shell/comments/model";
import { useCommentPicker } from "#gallery/shell/comments/use-comment-picker";
import { useLocated } from "#gallery/shell/comments/use-located";

type OverlayProps = {
  slug: string;
  doc: Document | null;
  picking: boolean;
  /** Every comment of the prototype; pins show the open ones of `state` and the selected one. */
  comments: readonly ReviewComment[];
  state: string;
  selectedId: string | null;
  onSelect: (id: string | null) => void;
  onCreate: (
    target: ElementTarget,
    body: string,
    viewport: { width: number; height: number },
    attachments: string[]
  ) => Promise<void>;
  onExit: () => void;
};

const pinSize = 24;

function box(rect: Rect, scale: number): CSSProperties {
  return {
    left: rect.x * scale,
    top: rect.y * scale,
    width: rect.width * scale,
    height: rect.height * scale,
  };
}

function Highlight({ rect, scale, selected }: { rect: Rect; scale: number; selected?: boolean }) {
  return (
    <div
      className={cn(
        "pointer-events-none absolute rounded-xs ring-2 ring-accent",
        selected ? "ring-offset-2 ring-offset-transparent" : "bg-accent-tint"
      )}
      style={box(rect, scale)}
    />
  );
}

type PinsProps = {
  comments: readonly ReviewComment[];
  located: ReadonlyMap<string, Located>;
  scale: number;
  bounds: { width: number; height: number };
  selectedId: string | null;
  onSelect: (id: string | null) => void;
};

/** Numbered pins at the top-right corner of each target, clamped inside the canvas. */
function CommentPins({ comments, located, scale, bounds, selectedId, onSelect }: PinsProps) {
  return comments.map(comment => {
    const spot = located.get(comment.id);
    if (!spot) return null;
    const { rect } = spot;
    const left = Math.min(
      Math.max((rect.x + rect.width) * scale - pinSize / 2, 4),
      bounds.width - pinSize - 4
    );
    const top = Math.min(Math.max(rect.y * scale - pinSize / 2, 4), bounds.height - pinSize - 4);
    const selected = comment.id === selectedId;
    const number = commentNumber(comment);
    return (
      <button
        aria-label={`Comment ${number}${spot.anchored ? "" : " (anchor not found)"}`}
        aria-pressed={selected}
        className={cn(
          "pointer-events-auto absolute grid size-6 place-items-center rounded-pill font-mono text-mono-id tabular-nums shadow-pop transition-transform outline-none hover:scale-110 focus-visible:shadow-focus-ring",
          comment.status === "open" ? "bg-accent text-accent-ink" : "bg-surface-2 text-muted",
          selected && "scale-110 ring-2 ring-accent-ink",
          !spot.anchored && "opacity-70"
        )}
        data-testid="comment-pin"
        key={comment.id}
        onClick={() => onSelect(selected ? null : comment.id)}
        onPointerDown={event => event.stopPropagation()}
        style={{ left, top }}
        title={comment.thread[0]?.body}
        type="button"
      >
        {number}
      </button>
    );
  });
}

/** Pins on screen: open comments of `state` inside the canvas, plus the selected one. */
function visibleComments(
  comments: readonly ReviewComment[],
  located: ReadonlyMap<string, Located>,
  view: { state: string; selectedId: string | null; height: number }
) {
  return comments.filter(comment => {
    if (comment.id === view.selectedId) return located.has(comment.id);
    if (comment.context.state !== view.state || comment.status === "resolved") return false;
    const rect = located.get(comment.id)?.rect;
    return rect !== undefined && rect.y + rect.height >= 0 && rect.y <= view.height;
  });
}

type HighlightsProps = {
  picking: boolean;
  hover: Rect | null;
  drag: Rect | null;
  draft: ElementTarget | null;
  selectedRect: Rect | undefined;
  scale: number;
};

/** The comment-mode hint and the hover, drag, selected and draft outlines. */
function PickerHighlights({ picking, hover, drag, draft, selectedRect, scale }: HighlightsProps) {
  if (draft) return <Highlight rect={draft.rect} scale={scale} />;
  return (
    <>
      {picking ? (
        <p
          className="pointer-events-none absolute top-3 left-1/2 z-10 -translate-x-1/2 rounded-pill bg-fg px-3 py-1 text-meta whitespace-nowrap text-canvas"
          role="status"
        >
          Click an element or drag an area · Esc to exit
        </p>
      ) : null}
      {picking && hover && !drag ? <Highlight rect={hover} scale={scale} /> : null}
      {drag ? <Highlight rect={drag} scale={scale} /> : null}
      {selectedRect ? <Highlight rect={selectedRect} scale={scale} selected /> : null}
    </>
  );
}

export function CommentOverlay(props: OverlayProps) {
  const { doc, picking, selectedId } = props;
  const [ref, size] = useElementSize<HTMLDivElement>();
  const view = doc?.defaultView;
  const canvas = { width: view?.innerWidth ?? 0, height: view?.innerHeight ?? 0 };
  const scale = canvas.width > 0 && size.width > 0 ? size.width / canvas.width : 1;
  const located = useLocated(doc, props.comments, canvas);
  const picker = useCommentPicker({ doc, canvas, scale, picking, onExit: props.onExit });
  const { draft } = picker;
  const visible = visibleComments(props.comments, located, {
    state: props.state,
    selectedId,
    height: canvas.height,
  });

  return (
    <div
      className={cn(
        "absolute inset-0 overflow-hidden",
        picking ? "cursor-crosshair" : "pointer-events-none"
      )}
      data-testid="comment-overlay"
      ref={ref}
      {...(picking ? picker.handlers : {})}
    >
      <PickerHighlights
        drag={picker.drag}
        draft={draft}
        hover={picker.hover}
        picking={picking}
        scale={scale}
        selectedRect={selectedId ? located.get(selectedId)?.rect : undefined}
      />
      <CommentPins
        bounds={size}
        comments={visible}
        located={located}
        onSelect={props.onSelect}
        scale={scale}
        selectedId={selectedId}
      />
      {draft ? (
        <CommentComposer
          bounds={size}
          onCancel={picker.clearDraft}
          onSubmit={async (body, attachments) => {
            await props.onCreate(draft, body, canvas, attachments);
            picker.clearDraft();
          }}
          scale={scale}
          slug={props.slug}
          target={draft}
        />
      ) : null}
    </div>
  );
}
