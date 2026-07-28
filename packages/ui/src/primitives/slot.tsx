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

const motionComponentCache = new Map<React.ElementType, React.ElementType>();

function getOrCreateMotionComponent(type: React.ElementType): React.ElementType {
  const key = type;
  if (!motionComponentCache.has(key)) {
    motionComponentCache.set(key, m.create(type));
  }
  return motionComponentCache.get(key)!;
}

function Slot<T extends HTMLElement = HTMLElement>({ children, ref, ...props }: SlotProps<T>) {
  if (!React.isValidElement(children)) return null;

  const isAlreadyMotion =
    typeof children.type === "object" && children.type !== null && isMotionComponent(children.type);

  const Base = isAlreadyMotion
    ? (children.type as React.ElementType)
    : getOrCreateMotionComponent(children.type as React.ElementType);

  const { ref: childRef, ...childProps } = children.props as AnyProps;

  const mergedProps = mergeProps(childProps, props);

  return <Base {...mergedProps} ref={mergeRefs(childRef as React.Ref<T>, ref)} />;
}

export { Slot, type AnyProps, type DOMMotionProps, type SlotProps, type WithAsChild };
