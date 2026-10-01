"use client";

import { ChevronLeftIcon } from "lucide-react";
import { AnimatePresence, m, useReducedMotionConfig } from "motion/react";
import * as React from "react";

import { MOTION_DURATION_BASE, MOTION_EASE_OUT } from "../../lib/motion";
import { cn } from "../../lib/utils";
import { useNarrowViewport } from "./hooks/use-narrow-viewport";

const SPLIT_LIST_WIDTH_DEFAULT = 340;
const SPLIT_NARROW_BREAKPOINT_DEFAULT = 768;

export interface SplitPaneProps extends Omit<React.ComponentProps<"div">, "onChange"> {
  list: React.ReactNode;
  detail?: React.ReactNode;
  listWidth?: number;
  detailEmpty?: React.ReactNode;
  onDetailClose?: () => void;
  narrowBreakpoint?: number;
  backLabel?: string;
}

function isDetailPresent(detail: React.ReactNode): boolean {
  return detail !== null && detail !== undefined && detail !== false;
}

interface SplitPaneLayout {
  /** Narrow with detail but no close handler: list and detail stack in a column. */
  stack: boolean;
  /** Narrow with a closable detail: the detail overlays the list. */
  overlay: boolean;
  showList: boolean;
  showDetail: boolean;
}

function splitPaneLayout(narrow: boolean, hasDetail: boolean, closable: boolean): SplitPaneLayout {
  if (!narrow) return { stack: false, overlay: false, showList: true, showDetail: true };
  if (!hasDetail) return { stack: false, overlay: true, showList: true, showDetail: false };
  if (!closable) return { stack: true, overlay: false, showList: true, showDetail: true };
  return { stack: false, overlay: true, showList: false, showDetail: true };
}

function SplitPaneBackBar({ label, onBack }: { label: string; onBack?: () => void }) {
  return (
    <div
      data-slot="split-pane-detail-bar"
      className="flex shrink-0 items-center gap-2 border-b border-line px-3 py-2"
    >
      <button
        type="button"
        data-slot="split-pane-back"
        onClick={onBack}
        className="inline-flex items-center gap-1.5 rounded-md px-2 py-1 text-eyebrow font-medium text-muted transition-colors hover:bg-hover hover:text-fg focus-visible:outline-none focus-visible:shadow-focus-ring"
      >
        <ChevronLeftIcon aria-hidden="true" className="size-3" />
        <span>{label}</span>
      </button>
    </div>
  );
}

interface SplitPaneDetailProps {
  detail: React.ReactNode;
  detailEmpty: React.ReactNode;
  hasDetail: boolean;
  overlay: boolean;
  backLabel: string;
  onDetailClose?: () => void;
  duration: number;
}

/** The detail column: a back bar when it overlays a narrow list, then a cross-faded body. */
function SplitPaneDetail({
  detail,
  detailEmpty,
  hasDetail,
  overlay,
  backLabel,
  onDetailClose,
  duration,
}: SplitPaneDetailProps) {
  const transition = { duration, ease: MOTION_EASE_OUT };
  return (
    <m.div
      data-slot="split-pane-detail"
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
      exit={{ opacity: 0 }}
      transition={transition}
      className={cn(
        "flex min-h-0 min-w-0 flex-1 flex-col bg-canvas",
        overlay && "absolute inset-0"
      )}
    >
      {overlay && hasDetail ? <SplitPaneBackBar label={backLabel} onBack={onDetailClose} /> : null}
      <AnimatePresence initial={false} mode="wait">
        <m.div
          key={hasDetail ? "detail" : "empty"}
          data-slot={hasDetail ? "split-pane-detail-body" : "split-pane-detail-empty"}
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          exit={{ opacity: 0 }}
          transition={transition}
          className="flex min-h-0 min-w-0 flex-1 flex-col"
        >
          {hasDetail ? detail : detailEmpty}
        </m.div>
      </AnimatePresence>
    </m.div>
  );
}

function SplitPane({
  list,
  detail,
  listWidth = SPLIT_LIST_WIDTH_DEFAULT,
  detailEmpty,
  onDetailClose,
  narrowBreakpoint = SPLIT_NARROW_BREAKPOINT_DEFAULT,
  backLabel = "Back",
  className,
  ...props
}: SplitPaneProps) {
  const narrow = useNarrowViewport(narrowBreakpoint);
  const hasDetail = isDetailPresent(detail);
  const layout = splitPaneLayout(narrow, hasDetail, onDetailClose !== undefined);
  const reducedMotion = useReducedMotionConfig();

  return (
    <div
      data-slot="split-pane"
      data-narrow={narrow ? "true" : "false"}
      className={cn(
        "flex min-h-0 min-w-0 flex-1",
        layout.stack && "flex-col",
        layout.overlay && "relative",
        className
      )}
      {...props}
    >
      {layout.showList ? (
        <div
          data-slot="split-pane-list"
          className={cn(
            "flex min-h-0 shrink-0 flex-col bg-canvas",
            layout.stack ? "border-b border-line" : "border-r border-line"
          )}
          style={{ width: narrow ? "100%" : listWidth }}
        >
          {list}
        </div>
      ) : null}
      <AnimatePresence initial={false}>
        {layout.showDetail ? (
          <SplitPaneDetail
            key="split-pane-detail"
            backLabel={backLabel}
            detail={detail}
            detailEmpty={detailEmpty}
            duration={reducedMotion ? 0 : MOTION_DURATION_BASE}
            hasDetail={hasDetail}
            onDetailClose={onDetailClose}
            overlay={layout.overlay}
          />
        ) : null}
      </AnimatePresence>
    </div>
  );
}

export { SPLIT_LIST_WIDTH_DEFAULT, SplitPane };
