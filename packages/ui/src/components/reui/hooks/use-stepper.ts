"use client";

import { Children, createContext, isValidElement, use, useEffect, useRef, useState } from "react";
import type { KeyboardEvent, ReactElement, ReactNode, RefObject } from "react";

export type StepperOrientation = "horizontal" | "vertical";
export type StepState = "active" | "completed" | "inactive" | "loading";
export type StepIndicators = {
  active?: ReactNode;
  completed?: ReactNode;
  inactive?: ReactNode;
  loading?: ReactNode;
};

export interface StepperContextValue {
  activeStep: number;
  setActiveStep: (step: number) => void;
  stepsCount: number;
  orientation: StepperOrientation;
  triggerNodesRef: RefObject<Map<number, HTMLButtonElement>>;
  focusNext: (currentStep: number) => void;
  focusPrev: (currentStep: number) => void;
  focusFirst: () => void;
  focusLast: () => void;
  indicators: StepIndicators;
}

export interface StepItemContextValue {
  step: number;
  state: StepState;
  isDisabled: boolean;
  isLoading: boolean;
}

interface UseStepperStateOptions {
  defaultValue: number;
  value?: number;
  onValueChange?: (value: number) => void;
  orientation: StepperOrientation;
  indicators: StepIndicators;
  children: ReactNode;
}

export const StepperContext = createContext<StepperContextValue | undefined>(undefined);
export const StepItemContext = createContext<StepItemContextValue | undefined>(undefined);

export function useStepper() {
  const ctx = use(StepperContext);
  if (!ctx) throw new Error("useStepper must be used within a Stepper");
  return ctx;
}

export function useStepItem() {
  const ctx = use(StepItemContext);
  if (!ctx) throw new Error("useStepItem must be used within a StepperItem");
  return ctx;
}

export function useStepperState({
  defaultValue,
  value,
  onValueChange,
  orientation,
  indicators,
  children,
}: UseStepperStateOptions): StepperContextValue {
  const [activeStep, setActiveStep] = useState(defaultValue);
  const triggerNodesRef = useRef(new Map<number, HTMLButtonElement>());

  const handleSetActiveStep = (step: number) => {
    if (value === undefined) {
      setActiveStep(step);
    }
    onValueChange?.(step);
  };

  const currentStep = value ?? activeStep;
  const orderedTriggers = () =>
    [...triggerNodesRef.current.entries()].sort(([left], [right]) => left - right);
  const focusTrigger = (index: number) => {
    orderedTriggers()[index]?.[1].focus();
  };
  const focusNext = (currentStep: number) => {
    const triggers = orderedTriggers();
    const currentIndex = triggers.findIndex(([step]) => step === currentStep);
    if (currentIndex !== -1) triggers[(currentIndex + 1) % triggers.length]?.[1].focus();
  };
  const focusPrev = (currentStep: number) => {
    const triggers = orderedTriggers();
    const currentIndex = triggers.findIndex(([step]) => step === currentStep);
    if (currentIndex !== -1) {
      triggers[(currentIndex - 1 + triggers.length) % triggers.length]?.[1].focus();
    }
  };
  const focusFirst = () => focusTrigger(0);
  const focusLast = () => focusTrigger(triggerNodesRef.current.size - 1);
  const stepsCount = Children.toArray(children).filter(
    (child): child is ReactElement =>
      isValidElement(child) &&
      (child.type as { displayName?: string }).displayName === "StepperItem"
  ).length;

  return {
    activeStep: currentStep,
    setActiveStep: handleSetActiveStep,
    stepsCount,
    orientation,
    triggerNodesRef,
    focusNext,
    focusPrev,
    focusFirst,
    focusLast,
    indicators,
  };
}

export function useStepperTrigger() {
  const { state, isLoading, step, isDisabled } = useStepItem();
  const {
    setActiveStep,
    activeStep,
    triggerNodesRef,
    focusNext,
    focusPrev,
    focusFirst,
    focusLast,
  } = useStepper();
  const buttonRef = useRef<HTMLButtonElement>(null);
  const isSelected = activeStep === step;
  const id = `stepper-tab-${step}`;
  const panelId = `stepper-panel-${step}`;

  useEffect(() => {
    const node = buttonRef.current;
    if (!node) return;
    const triggerNodes = triggerNodesRef.current;
    triggerNodes.set(step, node);
    return () => {
      if (triggerNodes.get(step) === node) {
        triggerNodes.delete(step);
      }
    };
  }, [step, triggerNodesRef]);
  const selectStep = () => setActiveStep(step);
  const handleKeyDown = (event: KeyboardEvent<HTMLButtonElement>) => {
    switch (event.key) {
      case "ArrowRight":
      case "ArrowDown":
        event.preventDefault();
        focusNext(step);
        break;
      case "ArrowLeft":
      case "ArrowUp":
        event.preventDefault();
        focusPrev(step);
        break;
      case "Home":
        event.preventDefault();
        focusFirst();
        break;
      case "End":
        event.preventDefault();
        focusLast();
        break;
      case "Enter":
      case " ":
        event.preventDefault();
        selectStep();
        break;
    }
  };

  return {
    buttonRef,
    handleKeyDown,
    id,
    isDisabled,
    isLoading,
    isSelected,
    panelId,
    selectStep,
    state,
  };
}
