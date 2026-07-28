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
  const stackNarrowDetail = narrow && hasDetail && onDetailClose === undefined;
  const showList = stackNarrowDetail || !narrow || !hasDetail;
  const showDetail = stackNarrowDetail || !narrow || hasDetail;

  const reducedMotion = useReducedMotionConfig();
  const duration = reducedMotion ? 0 : MOTION_DURATION_BASE;

  return (
    <div
      data-slot="split-pane"
      data-narrow={narrow ? "true" : "false"}
      className={cn(
        "flex min-h-0 min-w-0 flex-1",
        stackNarrowDetail && "flex-col",
        narrow && !stackNarrowDetail && "relative",
        className
      )}
      {...props}
    >
      {showList ? (
        <div
          data-slot="split-pane-list"
          className={cn(
            "flex min-h-0 shrink-0 flex-col bg-canvas",
            stackNarrowDetail ? "border-b border-line" : "border-r border-line"
          )}
          style={{ width: narrow ? "100%" : listWidth }}
        >
          {list}
        </div>
      ) : null}
      <AnimatePresence initial={false}>
        {showDetail ? (
          <m.div
            key="split-pane-detail"
            data-slot="split-pane-detail"
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            transition={{ duration, ease: MOTION_EASE_OUT }}
            className={cn(
              "flex min-h-0 min-w-0 flex-1 flex-col bg-canvas",
              narrow && !stackNarrowDetail && "absolute inset-0"
            )}
          >
            {narrow && hasDetail && !stackNarrowDetail ? (
              <div
                data-slot="split-pane-detail-bar"
                className="flex shrink-0 items-center gap-2 border-b border-line px-3 py-2"
              >
                <button
                  type="button"
                  data-slot="split-pane-back"
                  onClick={onDetailClose}
                  className="inline-flex items-center gap-1.5 rounded-md px-2 py-1 text-xs font-medium text-muted transition-colors hover:bg-hover hover:text-fg focus-visible:outline-none focus-visible:shadow-focus-ring"
                >
                  <ChevronLeftIcon aria-hidden="true" className="size-3" />
                  <span>{backLabel}</span>
                </button>
              </div>
            ) : null}
            <AnimatePresence initial={false} mode="wait">
              <m.div
                key={hasDetail ? "detail" : "empty"}
                data-slot={hasDetail ? "split-pane-detail-body" : "split-pane-detail-empty"}
                initial={{ opacity: 0 }}
                animate={{ opacity: 1 }}
                exit={{ opacity: 0 }}
                transition={{ duration, ease: MOTION_EASE_OUT }}
                className="flex min-h-0 min-w-0 flex-1 flex-col"
              >
                {hasDetail ? detail : detailEmpty}
              </m.div>
            </AnimatePresence>
          </m.div>
        ) : null}
      </AnimatePresence>
    </div>
  );
}

export { SPLIT_LIST_WIDTH_DEFAULT, SplitPane };
