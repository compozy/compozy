/**
 * Review comments of the prototype on screen, as the three `PrototypeView` slots: toolbar actions,
 * the overlay over the reviewed frame (pins, picker, composer) and the comments panel. The panel
 * and the selected comment live in the URL (`comments=1`, `comment=<id>`); comment mode does not.
 */
import { type ElementTarget, type ReviewComment } from "#gallery/shell/comments/model";
import { commentsApi, galleryCommit } from "#gallery/shell/comments/api";
import { CommentOverlay } from "#gallery/shell/comments/overlay";
import { CommentsPanel } from "#gallery/shell/comments/panel";
import { ReviewActions } from "#gallery/shell/comments/review-actions";
import { useComments } from "#gallery/shell/comments/use-comments";
import type { ReviewSlots } from "#gallery/shell/prototype-view";
import type { Entry } from "#gallery/shell/registry-model";
import type { FrameInfo } from "#gallery/shell/use-frame";
import { maxWidth, minWidth, updateView, type GalleryView } from "#gallery/shell/view";

type ReviewOptions = {
  entry: Entry;
  view: GalleryView;
  state: string;
  dark: boolean;
  frame: FrameInfo | null;
  picking: boolean;
  onPicking: (picking: boolean) => void;
};

/** Reopens the view a comment was written on: its state, theme and width. */
function selectComment(comment: ReviewComment | null) {
  if (!comment) {
    updateView({ comment: null });
    return;
  }
  const { context } = comment;
  const width = Math.round(context.viewport.width);
  updateView({
    comment: comment.id,
    comments: true,
    state: context.state,
    theme: context.theme === "light" || context.theme === "dark" ? context.theme : null,
    ...(width >= minWidth && width <= maxWidth ? { width } : {}),
  });
}

export function useCommentReview(options: ReviewOptions): ReviewSlots {
  const { entry, view, state, frame, picking, onPicking } = options;
  const { comments, error } = useComments(entry.slug);
  // The frame reports the document it loaded; only review the one on screen, once it renders the
  // selected state (a state change rewrites the URL in place and reports it again).
  const doc = frame && frame.slug === entry.slug && frame.state === state ? frame.doc : null;
  const canPick = !view.compare && doc !== null;
  const theme = view.theme ?? (options.dark ? "dark" : "light");
  // A draft belongs to the view it was picked on; changing state, theme or width discards it.
  const draftView = `${state}|${theme}|${view.width}`;

  const create = async (
    target: ElementTarget,
    body: string,
    viewport: { width: number; height: number },
    attachments: string[]
  ) => {
    const created = await commentsApi.create(entry.slug, {
      body,
      attachments,
      target,
      context: {
        commit: galleryCommit,
        state,
        theme,
        viewport: { width: Math.round(viewport.width), height: Math.round(viewport.height) },
      },
    });
    onPicking(false);
    updateView({ comment: created.id, comments: true });
  };

  return {
    actions: (
      <ReviewActions
        canPick={canPick}
        onPanel={open => updateView(open ? { comments: true } : { comments: false, comment: null })}
        onPicking={onPicking}
        open={comments.filter(comment => comment.status === "open").length}
        panel={view.comments}
        picking={picking && canPick}
      />
    ),
    overlay: view.compare ? null : (
      <CommentOverlay
        comments={comments}
        key={draftView}
        doc={doc}
        onCreate={create}
        onExit={() => onPicking(false)}
        onSelect={id => selectComment(comments.find(comment => comment.id === id) ?? null)}
        picking={picking && canPick}
        selectedId={view.comment}
        slug={entry.slug}
        state={state}
      />
    ),
    panel: view.comments ? (
      <CommentsPanel
        comments={comments}
        error={error}
        onClose={() => updateView({ comments: false, comment: null })}
        onSelect={selectComment}
        place={state}
        places={entry.meta.states}
        selectedId={view.comment}
      />
    ) : null,
  };
}
