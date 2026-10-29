/**
 * The prototype level of the gallery: toolbar, then the canvas with the prototype in the selected
 * state, beside an optional side panel. Review comments plug into the three slots: toolbar
 * actions, the overlay over the reviewed frame and the side panel.
 */
import type { ReactNode } from "react";

import { Canvas } from "#gallery/shell/canvas";
import type { FrameInfo } from "#gallery/shell/use-frame";
import type { Entry, Group } from "#gallery/shell/registry-model";
import { WithSidePanel } from "#gallery/shell/side-panel";
import { GalleryToolbar } from "#gallery/shell/toolbar";
import type { GalleryView } from "#gallery/shell/view";

export type ReviewSlots = {
  /** Controls at the end of the toolbar's second row. */
  actions?: ReactNode;
  /** Layer over the reviewed frame (pins, picking). */
  overlay?: ReactNode;
  /** Resizable panel beside the canvas. */
  panel?: ReactNode;
};

type PrototypeViewProps = {
  entry: Entry;
  group: Group | undefined;
  view: GalleryView;
  state: string;
  dark: boolean;
  helpOpen: boolean;
  onHelp: (open: boolean) => void;
  onStep: (direction: 1 | -1) => void;
  onInfo: (info: FrameInfo | null) => void;
  review?: ReviewSlots;
};

export function PrototypeView(props: PrototypeViewProps) {
  const { entry, view, state, review } = props;
  return (
    <>
      <GalleryToolbar
        dark={props.dark}
        entry={entry}
        group={props.group}
        helpOpen={props.helpOpen}
        onHelp={props.onHelp}
        onStep={props.onStep}
        state={state}
        view={view}
      >
        {review?.actions}
      </GalleryToolbar>
      <WithSidePanel id="review" label="Review panel width" side={review?.panel}>
        <Canvas
          compare={view.compare}
          onInfo={props.onInfo}
          slug={entry.slug}
          state={state}
          theme={view.theme}
          title={entry.meta.title}
          width={view.width}
        >
          {review?.overlay}
        </Canvas>
      </WithSidePanel>
    </>
  );
}
