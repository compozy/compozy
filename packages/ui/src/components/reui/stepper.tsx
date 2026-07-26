"use client";

import type { ButtonHTMLAttributes, ComponentProps, HTMLAttributes } from "react";

import { cn } from "@agh/ui/lib/utils";
import {
  StepItemContext,
  StepperContext,
  useStepItem,
  useStepper,
  useStepperState,
  useStepperTrigger,
  type StepIndicators,
  type StepperOrientation,
  type StepState,
} from "./hooks/use-stepper";

const EMPTY_STEP_INDICATORS: StepIndicators = {};

interface StepperProps extends HTMLAttributes<HTMLDivElement> {
  defaultValue?: number;
  value?: number;
  onValueChange?: (value: number) => void;
  orientation?: StepperOrientation;
  indicators?: StepIndicators;
}

function Stepper({
  defaultValue = 1,
  value,
  onValueChange,
  orientation = "horizontal",
  className,
  children,
  indicators = EMPTY_STEP_INDICATORS,
  ...props
}: StepperProps) {
  const contextValue = useStepperState({
    defaultValue,
    value,
    onValueChange,
    orientation,
    indicators,
    children,
  });

  return (
    <StepperContext.Provider value={contextValue}>
      <div
        role="tablist"
        aria-orientation={orientation}
        data-slot="stepper"
        className={cn("w-full", className)}
        data-orientation={orientation}
        {...props}
      >
        {children}
      </div>
    </StepperContext.Provider>
  );
}

interface StepperItemProps extends HTMLAttributes<HTMLDivElement> {
  step: number;
  completed?: boolean;
  disabled?: boolean;
  loading?: boolean;
}

function StepperItem({
  step,
  completed = false,
  disabled = false,
  loading = false,
  className,
  children,
  ...props
}: StepperItemProps) {
  const { activeStep } = useStepper();

  const state: StepState =
    completed || step < activeStep ? "completed" : activeStep === step ? "active" : "inactive";

  const isLoading = loading && step === activeStep;
  const contextValue = { step, state, isDisabled: disabled, isLoading };

  return (
    <StepItemContext.Provider value={contextValue}>
      <div
        data-slot="stepper-item"
        className={cn(
          "group/step flex items-center justify-center not-last:flex-1 group-data-[orientation=horizontal]/stepper-nav:flex-row group-data-[orientation=vertical]/stepper-nav:block group-data-[orientation=vertical]/stepper-nav:w-full",
          className
        )}
        data-state={state}
        {...(isLoading ? { "data-loading": true } : {})}
        {...props}
      >
        {children}
      </div>
    </StepItemContext.Provider>
  );
}

interface StepperTriggerProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  asChild?: boolean;
}

function StepperTrigger({
  asChild = false,
  className,
  children,
  tabIndex,
  ...props
}: StepperTriggerProps) {
  const {
    buttonRef,
    handleKeyDown,
    id,
    isDisabled,
    isLoading,
    isSelected,
    panelId,
    selectStep,
    state,
  } = useStepperTrigger();

  if (asChild) {
    return (
      <span data-slot="stepper-trigger" data-state={state} className={className}>
        {children}
      </span>
    );
  }

  return (
    <button
      ref={buttonRef}
      role="tab"
      id={id}
      aria-selected={isSelected}
      aria-controls={panelId}
      tabIndex={typeof tabIndex === "number" ? tabIndex : isSelected ? 0 : -1}
      data-slot="stepper-trigger"
      data-state={state}
      data-loading={isLoading}
      className={cn(
        "cursor-pointer outline-none focus-visible:z-10 focus-visible:shadow-focus-ring disabled:pointer-events-none disabled:cursor-not-allowed disabled:opacity-60",
        "group-data-[orientation=horizontal]/stepper-nav:inline-flex group-data-[orientation=horizontal]/stepper-nav:items-center group-data-[orientation=horizontal]/stepper-nav:gap-2.5 group-data-[orientation=horizontal]/stepper-nav:rounded-full",
        "group-data-[orientation=vertical]/stepper-nav:flex group-data-[orientation=vertical]/stepper-nav:w-full group-data-[orientation=vertical]/stepper-nav:items-start group-data-[orientation=vertical]/stepper-nav:gap-3 group-data-[orientation=vertical]/stepper-nav:rounded-md group-data-[orientation=vertical]/stepper-nav:text-left group-data-[orientation=vertical]/stepper-nav:enabled:hover:**:data-[slot=stepper-title]:text-fg",
        className
      )}
      onClick={selectStep}
      onKeyDown={handleKeyDown}
      disabled={isDisabled}
      {...props}
    >
      {children}
    </button>
  );
}

function StepperRail({ children, className, ...props }: ComponentProps<"div">) {
  return (
    <div
      data-slot="stepper-rail"
      className={cn(
        "hidden group-data-[orientation=vertical]/stepper-nav:flex group-data-[orientation=vertical]/stepper-nav:flex-col group-data-[orientation=vertical]/stepper-nav:items-center group-data-[orientation=vertical]/stepper-nav:self-stretch",
        className
      )}
      {...props}
    >
      {children}
    </div>
  );
}

function StepperBody({ children, className, ...props }: ComponentProps<"div">) {
  return (
    <div
      data-slot="stepper-body"
      className={cn(
        "group-data-[orientation=vertical]/stepper-nav:pt-0.5 group-data-[orientation=vertical]/stepper-nav:pb-6",
        className
      )}
      {...props}
    >
      {children}
    </div>
  );
}

function StepperIndicator({ children, className }: ComponentProps<"div">) {
  const { state, isLoading } = useStepItem();
  const { indicators } = useStepper();

  return (
    <div
      data-slot="stepper-indicator"
      data-state={state}
      className={cn(
        "relative flex size-button-icon-default shrink-0 items-center justify-center overflow-hidden rounded-full text-xs font-semibold tabular-nums transition-[background-color,color,box-shadow] duration-base ease-in-out",
        "data-[state=inactive]:bg-elevated data-[state=inactive]:text-subtle data-[state=inactive]:shadow-inset-strong",
        "data-[state=active]:bg-accent data-[state=active]:text-accent-ink data-[state=active]:shadow-[var(--shadow-highlight),0_0_0_4px_var(--color-accent-tint)]",
        "data-[state=completed]:bg-accent data-[state=completed]:text-accent-ink data-[state=completed]:shadow-highlight",
        className
      )}
    >
      <div className="absolute">
        {indicators &&
        ((isLoading && indicators.loading) ||
          (state === "completed" && indicators.completed) ||
          (state === "active" && indicators.active) ||
          (state === "inactive" && indicators.inactive))
          ? (isLoading && indicators.loading) ||
            (state === "completed" && indicators.completed) ||
            (state === "active" && indicators.active) ||
            (state === "inactive" && indicators.inactive)
          : children}
      </div>
    </div>
  );
}

function StepperSeparator({ className }: ComponentProps<"div">) {
  const { state } = useStepItem();

  return (
    <div
      data-slot="stepper-separator"
      data-state={state}
      className={cn(
        "rounded-sm bg-line transition-colors duration-base",
        "group-data-[orientation=horizontal]/stepper-nav:m-0.5 group-data-[orientation=horizontal]/stepper-nav:h-0.5 group-data-[orientation=horizontal]/stepper-nav:flex-1",
        "group-data-[orientation=vertical]/stepper-nav:my-1.25 group-data-[orientation=vertical]/stepper-nav:w-px group-data-[orientation=vertical]/stepper-nav:min-h-button-default group-data-[orientation=vertical]/stepper-nav:flex-1 group-data-[orientation=vertical]/stepper-nav:data-[state=completed]:bg-accent-dim",
        className
      )}
    />
  );
}

function StepperTitle({ children, className }: ComponentProps<"h3">) {
  const { state } = useStepItem();

  return (
    <h3
      data-slot="stepper-title"
      data-state={state}
      className={cn(
        "text-card-title leading-none font-medium tracking-normal transition-colors duration-base",
        "data-[state=inactive]:text-muted",
        "data-[state=active]:text-fg-strong",
        "data-[state=completed]:text-fg",
        className
      )}
    >
      {children}
    </h3>
  );
}

function StepperDescription({ children, className }: ComponentProps<"div">) {
  const { state } = useStepItem();

  return (
    <div
      data-slot="stepper-description"
      data-state={state}
      className={cn(
        "mt-1 text-form-hint leading-normal transition-colors duration-base",
        "data-[state=inactive]:text-faint data-[state=completed]:text-faint",
        "data-[state=active]:text-muted",
        className
      )}
    >
      {children}
    </div>
  );
}

function StepperNav({ children, className }: ComponentProps<"nav">) {
  const { activeStep, orientation } = useStepper();

  return (
    <nav
      data-slot="stepper-nav"
      data-state={activeStep}
      data-orientation={orientation}
      className={cn(
        "group/stepper-nav inline-flex data-[orientation=horizontal]:w-full data-[orientation=horizontal]:flex-row data-[orientation=vertical]:flex-col",
        className
      )}
    >
      {children}
    </nav>
  );
}

function StepperPanel({ children, className }: ComponentProps<"div">) {
  const { activeStep } = useStepper();

  return (
    <div data-slot="stepper-panel" data-state={activeStep} className={cn("w-full", className)}>
      {children}
    </div>
  );
}

interface StepperContentProps extends ComponentProps<"div"> {
  value: number;
  forceMount?: boolean;
}

function StepperContent({ value, forceMount, children, className }: StepperContentProps) {
  const { activeStep } = useStepper();
  const isActive = value === activeStep;

  if (!forceMount && !isActive) {
    return null;
  }

  return (
    <div
      data-slot="stepper-content"
      data-state={activeStep}
      className={cn("w-full", className, !isActive && forceMount && "hidden")}
      hidden={!isActive && forceMount}
    >
      {children}
    </div>
  );
}

export {
  useStepper,
  useStepItem,
  Stepper,
  StepperItem,
  StepperTrigger,
  StepperRail,
  StepperBody,
  StepperIndicator,
  StepperSeparator,
  StepperTitle,
  StepperDescription,
  StepperPanel,
  StepperContent,
  StepperNav,
  type StepperProps,
  type StepperItemProps,
  type StepperTriggerProps,
  type StepperContentProps,
};
