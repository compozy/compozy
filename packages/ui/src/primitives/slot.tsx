"use client";

import * as React from "react";

import { isMotionComponent, m, type HTMLMotionProps } from "motion/react";

import { cn } from "../lib/utils";

type AnyProps = Record<string, unknown>;

type DOMMotionProps<T extends HTMLElement = HTMLElement> = Omit<
  HTMLMotionProps<keyof HTMLElementTagNameMap>,
  "ref"
> & {
  ref?: React.Ref<T>;
};

type WithAsChild<Base extends object> =
  | (Base & { asChild: true; children: React.ReactElement })
  | (Base & { asChild?: false | undefined });

type SlotProps<T extends HTMLElement = HTMLElement> = {
  children?: React.ReactNode;
} & DOMMotionProps<T>;

function mergeRefs<T>(...refs: (React.Ref<T> | undefined)[]): React.RefCallback<T> {
  return node => {
    refs.forEach(ref => {
      if (!ref) return;
      if (typeof ref === "function") {
        ref(node);
      } else {
        (ref as React.RefObject<T | null>).current = node;
      }
    });
  };
}

function mergeProps<T extends HTMLElement>(
  childProps: AnyProps,
  slotProps: DOMMotionProps<T>
): AnyProps {
  const merged: AnyProps = { ...childProps, ...slotProps };

  if (childProps.className || slotProps.className) {
    merged.className = cn(childProps.className as string, slotProps.className as string);
  }

  if (childProps.style || slotProps.style) {
    merged.style = {
      ...(childProps.style as React.CSSProperties),
      ...(slotProps.style as React.CSSProperties),
    };
  }

  return merged;
}

function SlotClone<T extends HTMLElement = HTMLElement>({ children, ref, ...props }: SlotProps<T>) {
  if (!React.isValidElement(children)) return null;
  const child = children as React.ReactElement<AnyProps>;
  const { ref: childRef, ...childProps } = child.props;
  const mergedProps = mergeProps(childProps, props);

  return React.cloneElement(child, {
    ...mergedProps,
    ref: mergeRefs(childRef as React.Ref<T>, ref),
  });
}

const MotionSlot = m.create(SlotClone);

function Slot<T extends HTMLElement = HTMLElement>({ children, ...props }: SlotProps<T>) {
  const isAlreadyMotion =
    React.isValidElement(children) &&
    typeof children.type === "object" &&
    children.type !== null &&
    isMotionComponent(children.type);

  if (isAlreadyMotion) return <SlotClone {...props}>{children}</SlotClone>;
  return <MotionSlot {...props}>{children}</MotionSlot>;
}

export { Slot, type AnyProps, type DOMMotionProps, type SlotProps, type WithAsChild };
