"use client";

import * as React from "react";

import { useMotionValue, useMotionValueEvent, useSpring } from "motion/react";

import { useIsInView, type UseIsInViewOptions } from "./use-is-in-view";

export interface UseSlidingNumberParams {
  ref?: React.Ref<HTMLElement>;
  number: number;
  fromNumber?: number;
  onNumberChange?: (number: number) => void;
  inView: boolean;
  inViewMargin: UseIsInViewOptions["inViewMargin"];
  inViewOnce: boolean;
  padStart: boolean;
  decimalPlaces: number;
  initiallyStable: boolean;
  delay: number;
}

function formatSlidingNumber(num: number, decimalPlaces: number): string {
  return decimalPlaces != null ? num.toFixed(decimalPlaces) : num.toString();
}

export function useSlidingNumber({
  ref,
  number,
  fromNumber,
  onNumberChange,
  inView,
  inViewMargin,
  inViewOnce,
  padStart,
  decimalPlaces,
  initiallyStable,
  delay,
}: UseSlidingNumberParams) {
  const { ref: localRef, isInView } = useIsInView(ref as React.Ref<HTMLElement>, {
    inView,
    inViewOnce,
    inViewMargin,
  });

  const initialNumeric = Math.abs(Number(number));
  const hasAnimated = fromNumber !== undefined;

  const motionVal = useMotionValue(initiallyStable ? initialNumeric : (fromNumber ?? 0));
  const springVal = useSpring(motionVal, { stiffness: 90, damping: 50 });

  const skippedInitialWhenStable = React.useRef(false);

  React.useLayoutEffect(() => {
    if (!hasAnimated) return;
    if (initiallyStable && !skippedInitialWhenStable.current) {
      skippedInitialWhenStable.current = true;
      return;
    }
    const timeoutId = setTimeout(() => {
      if (isInView) motionVal.set(number);
    }, delay);
    return () => clearTimeout(timeoutId);
  }, [hasAnimated, initiallyStable, isInView, number, motionVal, delay]);

  const [animatedNumber, setAnimatedNumber] = React.useState<number>(
    initiallyStable ? initialNumeric : 0
  );
  const inferredDecimals =
    typeof decimalPlaces === "number" && decimalPlaces >= 0
      ? decimalPlaces
      : (() => {
          const value = String(number);
          const decimalIndex = value.indexOf(".");
          return decimalIndex >= 0 ? value.length - decimalIndex - 1 : 0;
        })();
  const factor = Math.pow(10, inferredDecimals);
  useMotionValueEvent(springVal, "change", latest => {
    if (!hasAnimated) return;
    const next = inferredDecimals > 0 ? Math.round(latest * factor) / factor : Math.round(latest);
    setAnimatedNumber(current => (current === next ? current : next));
    onNumberChange?.(next);
  });
  const effectiveNumber = hasAnimated
    ? animatedNumber
    : initiallyStable || isInView
      ? initialNumeric
      : 0;
  const [numberFrame, setNumberFrame] = React.useState(() => ({
    current: effectiveNumber,
    previous: effectiveNumber,
  }));
  const nextNumberFrame =
    numberFrame.current === effectiveNumber
      ? numberFrame
      : { current: effectiveNumber, previous: numberFrame.current };
  if (nextNumberFrame !== numberFrame) setNumberFrame(nextNumberFrame);
  const prevNumber = nextNumberFrame.previous;

  const numberStr = formatSlidingNumber(effectiveNumber, decimalPlaces);
  const [newIntStrRaw = "0", newDecStrRaw = ""] = numberStr.split(".");

  const finalIntLength = padStart
    ? Math.max(Math.floor(Math.abs(number)).toString().length, newIntStrRaw.length)
    : newIntStrRaw.length;

  const newIntStr = padStart ? newIntStrRaw.padStart(finalIntLength, "0") : newIntStrRaw;

  const prevFormatted = formatSlidingNumber(prevNumber, decimalPlaces);
  const [prevIntStrRaw = "", prevDecStrRaw = ""] = prevFormatted.split(".");
  const prevIntStr = padStart ? prevIntStrRaw.padStart(finalIntLength, "0") : prevIntStrRaw;

  const adjustedPrevInt =
    prevIntStr.length > finalIntLength
      ? prevIntStr.slice(-finalIntLength)
      : prevIntStr.padStart(finalIntLength, "0");

  const adjustedPrevDec = newDecStrRaw
    ? prevDecStrRaw.length > newDecStrRaw.length
      ? prevDecStrRaw.slice(0, newDecStrRaw.length)
      : prevDecStrRaw.padEnd(newDecStrRaw.length, "0")
    : "";

  const intPlaces = Array.from({ length: finalIntLength }, (_, i) =>
    Math.pow(10, finalIntLength - i - 1)
  );
  const decPlaces = newDecStrRaw
    ? Array.from({ length: newDecStrRaw.length }, (_, i) =>
        Math.pow(10, newDecStrRaw.length - i - 1)
      )
    : [];

  const newDecValue = newDecStrRaw ? parseInt(newDecStrRaw, 10) : 0;
  const prevDecValue = adjustedPrevDec ? parseInt(adjustedPrevDec, 10) : 0;

  return {
    localRef,
    isInView,
    intPlaces,
    decPlaces,
    adjustedPrevInt: parseInt(adjustedPrevInt, 10),
    newIntValue: parseInt(newIntStr ?? "0", 10),
    newDecStrRaw,
    newDecValue,
    prevDecValue,
  };
}
