"use client";

import * as React from "react";

import { cn } from "../../lib/utils";
import { Surface } from "../surface";
import type { PillTone } from "./pill";
import { StatusCardContext } from "./hooks/use-status-card-tone";
import {
  StatusCardAction,
  StatusCardBody,
  StatusCardFooter,
  StatusCardHeader,
} from "./status-card-parts";

type StatusCardTone = PillTone;

interface StatusCardProps extends React.ComponentProps<"div"> {
  tone?: StatusCardTone;
}

function StatusCard({ tone = "neutral", className, children, ...props }: StatusCardProps) {
  const contextValue = { tone };
  return (
    <StatusCardContext.Provider value={contextValue}>
      <Surface
        className={cn("flex min-w-0 flex-col gap-3", className)}
        data-slot="status-card"
        data-tone={tone}
        {...props}
      >
        {children}
      </Surface>
    </StatusCardContext.Provider>
  );
}

const StatusCardCompound = Object.assign(StatusCard, {
  Header: StatusCardHeader,
  Body: StatusCardBody,
  Footer: StatusCardFooter,
  Action: StatusCardAction,
});

export { StatusCardCompound as StatusCard };
export type {
  StatusCardActionProps,
  StatusCardBodyProps,
  StatusCardFooterProps,
  StatusCardHeaderProps,
} from "./status-card-parts";
export type { StatusCardProps, StatusCardTone };
