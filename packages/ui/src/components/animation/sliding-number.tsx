"use client";

import * as React from "react";

import {
  m,
  useSpring,
  useTransform,
  type HTMLMotionProps,
  type MotionValue,
  type SpringOptions,
} from "motion/react";

import { type UseIsInViewOptions } from "../../hooks/use-is-in-view";
import { useMeasure } from "../../hooks/use-measure";
import { useSlidingNumber } from "../../hooks/use-sliding-number";
import { cn } from "../../lib/utils";

interface SlidingNumberRollerProps {
  prevValue: number;
  value: number;
  place: number;
  transition: SpringOptions;
  delay?: number;
}

function SlidingNumberRoller({
  prevValue,
  value,
  place,
  transition,
  delay = 0,
}: SlidingNumberRollerProps) {
  const startNumber = Math.floor(prevValue / place) % 10;
  const targetNumber = Math.floor(value / place) % 10;
  const animatedValue = useSpring(startNumber, transition);

  React.useEffect(() => {
    const timeoutId = setTimeout(() => {
      animatedValue.set(targetNumber);
    }, delay);
    return () => clearTimeout(timeoutId);
  }, [targetNumber, animatedValue, delay]);

  const [measureRef, { height }] = useMeasure();

  return (
    <span
      ref={measureRef}
      data-slot="sliding-number-roller"
      style={{
        position: "relative",
        display: "inline-block",
        width: "1ch",
        overflowX: "visible",
        overflowY: "clip",
        lineHeight: 1,
        fontVariantNumeric: "tabular-nums",
      }}
    >
      <span style={{ visibility: "hidden" }}>0</span>
      {Array.from({ length: 10 }, (_, i) => (
        <SlidingNumberDisplay
          key={i}
          motionValue={animatedValue}
          number={i}
          height={height}
          transition={transition}
        />
      ))}
    </span>
  );
}

interface SlidingNumberDisplayProps {
  motionValue: MotionValue<number>;
  number: number;
  height: number;
  transition: SpringOptions;
}

function SlidingNumberDisplay({
  motionValue,
  number,
  height,
  transition,
}: SlidingNumberDisplayProps) {
  const y = useTransform(motionValue, latest => {
    if (!height) return 0;
    const currentNumber = latest % 10;
    const offset = (10 + number - currentNumber) % 10;
    let translateY = offset * height;
    if (offset > 5) translateY -= 10 * height;
    return translateY;
  });

  if (!height) {
    return <span style={{ visibility: "hidden", position: "absolute" }}>{number}</span>;
  }

  return (
    <m.span
      data-slot="sliding-number-display"
      style={{
        y,
        position: "absolute",
        inset: 0,
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
      }}
      transition={{ ...transition, type: "spring" }}
    >
      {number}
    </m.span>
  );
}

export interface SlidingNumberProps
  extends Omit<HTMLMotionProps<"span">, "children">, UseIsInViewOptions {
  number: number;
  fromNumber?: number;
  onNumberChange?: (number: number) => void;
  padStart?: boolean;
  decimalSeparator?: string;
  decimalPlaces?: number;
  thousandSeparator?: string;
  transition?: SpringOptions;
  delay?: number;
  initiallyStable?: boolean;
}

export function SlidingNumber({
  ref,
  number,
  fromNumber,
  onNumberChange,
  inView = false,
  inViewMargin = "0px",
  inViewOnce = true,
  padStart = false,
  decimalSeparator = ".",
  decimalPlaces = 0,
  thousandSeparator,
  transition = { stiffness: 200, damping: 20, mass: 0.4 },
  delay = 0,
  initiallyStable = false,
  className,
  ...props
}: SlidingNumberProps) {
  const {
    localRef,
    isInView,
    intPlaces,
    decPlaces,
    adjustedPrevInt,
    newIntValue,
    newDecStrRaw,
    newDecValue,
    prevDecValue,
  } = useSlidingNumber({
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
  });

  return (
    <m.span
      ref={localRef}
      className={cn("inline-flex items-center", className)}
      data-slot="sliding-number"
      {...props}
    >
      {isInView && Number(number) < 0 && <span style={{ marginRight: "0.25rem" }}>-</span>}

      {intPlaces.map((place, idx) => {
        const digitsToRight = intPlaces.length - idx - 1;
        const isSeparatorPosition =
          typeof thousandSeparator !== "undefined" && digitsToRight > 0 && digitsToRight % 3 === 0;

        return (
          <React.Fragment key={`int-${place}`}>
            <SlidingNumberRoller
              prevValue={adjustedPrevInt}
              value={newIntValue}
              place={place}
              transition={transition}
            />
            {isSeparatorPosition && <span>{thousandSeparator}</span>}
          </React.Fragment>
        );
      })}

      {newDecStrRaw && (
        <>
          <span>{decimalSeparator}</span>
          {decPlaces.map(place => (
            <SlidingNumberRoller
              key={`dec-${place}`}
              prevValue={prevDecValue}
              value={newDecValue}
              place={place}
              transition={transition}
              delay={delay}
            />
          ))}
        </>
      )}
    </m.span>
  );
}
