"use client";
import { cn } from "@/lib/utils";
import type { TargetAndTransition, Transition, Variant, Variants } from "motion/react";
import { AnimatePresence, motion } from "motion/react";
import React from "react";

export type PresetType = "blur" | "fade-in-blur" | "scale" | "fade" | "slide";

export type PerType = "word" | "char" | "line";

export type TextEffectProps = {
  children: React.ReactNode;
  per?: PerType;
  as?: keyof React.JSX.IntrinsicElements;
  variants?: {
    container?: Variants;
    item?: Variants;
  };
  className?: string;
  preset?: PresetType;
  delay?: number;
  speedReveal?: number;
  speedSegment?: number;
  trigger?: boolean;
  onAnimationComplete?: () => void;
  onAnimationStart?: () => void;
  segmentWrapperClassName?: string;
  containerTransition?: Transition;
  segmentTransition?: Transition;
  style?: React.CSSProperties;
};

const defaultStaggerTimes: Record<PerType, number> = {
  char: 0.03,
  word: 0.05,
  line: 0.1,
};

const defaultContainerVariants: Variants = {
  hidden: { opacity: 0 },
  visible: {
    opacity: 1,
    transition: {
      staggerChildren: 0.05,
    },
  },
  exit: {
    transition: { staggerChildren: 0.05, staggerDirection: -1 },
  },
};

const defaultItemVariants: Variants = {
  hidden: { opacity: 0 },
  visible: {
    opacity: 1,
  },
  exit: { opacity: 0 },
};

const presetVariants: Record<PresetType, { container: Variants; item: Variants }> = {
  blur: {
    container: defaultContainerVariants,
    item: {
      hidden: { opacity: 0, filter: "blur(12px)" },
      visible: { opacity: 1, filter: "blur(0px)" },
      exit: { opacity: 0, filter: "blur(12px)" },
    },
  },
  "fade-in-blur": {
    container: defaultContainerVariants,
    item: {
      hidden: { opacity: 0, y: 20, filter: "blur(12px)" },
      visible: { opacity: 1, y: 0, filter: "blur(0px)" },
      exit: { opacity: 0, y: 20, filter: "blur(12px)" },
    },
  },
  scale: {
    container: defaultContainerVariants,
    item: {
      hidden: { opacity: 0, scale: 0 },
      visible: { opacity: 1, scale: 1 },
      exit: { opacity: 0, scale: 0 },
    },
  },
  fade: {
    container: defaultContainerVariants,
    item: {
      hidden: { opacity: 0 },
      visible: { opacity: 1 },
      exit: { opacity: 0 },
    },
  },
  slide: {
    container: defaultContainerVariants,
    item: {
      hidden: { opacity: 0, y: 20 },
      visible: { opacity: 1, y: 0 },
      exit: { opacity: 0, y: 20 },
    },
  },
};

const AnimationComponent: React.FC<{
  segment: React.ReactNode;
  variants: Variants;
  per: "line" | "word" | "char";
  segmentWrapperClassName?: string;
}> = React.memo(({ segment, variants, per, segmentWrapperClassName }) => {
  const renderContent = () => {
    // If segment is already a React element (like a span with styling), wrap it
    if (React.isValidElement(segment)) {
      return (
        <motion.span
          variants={variants}
          className={per === "word" ? "inline-block" : "block"}
          style={{ display: per === "word" ? "inline-block" : "block" }}
        >
          {segment}
        </motion.span>
      );
    }

    // For string segments, handle as before
    const text = String(segment);
    if (per === "line") {
      return (
        <motion.span variants={variants} className="block">
          {text}
        </motion.span>
      );
    } else if (per === "word") {
      return (
        <motion.span aria-hidden="true" variants={variants} className="inline-block whitespace-pre">
          {text}
        </motion.span>
      );
    } else {
      return (
        <motion.span className="inline-block whitespace-pre">
          {text.split("").map((char, charIndex) => (
            <motion.span
              key={`char-${charIndex}`}
              aria-hidden="true"
              variants={variants}
              className="inline-block whitespace-pre"
            >
              {char}
            </motion.span>
          ))}
        </motion.span>
      );
    }
  };

  const content = renderContent();

  if (!segmentWrapperClassName) {
    return content;
  }

  const defaultWrapperClassName = per === "line" ? "block" : "inline-block";

  return <span className={cn(defaultWrapperClassName, segmentWrapperClassName)}>{content}</span>;
});

AnimationComponent.displayName = "AnimationComponent";

type SegmentType = string | React.ReactElement;

const splitChildren = (children: React.ReactNode, per: PerType): SegmentType[] => {
  const segments: SegmentType[] = [];

  const processNode = (node: React.ReactNode): void => {
    if (typeof node === "string" || typeof node === "number") {
      const text = String(node);
      if (per === "line") {
        segments.push(...text.split("\n"));
      } else if (per === "word") {
        // Split by whitespace but preserve the whitespace
        const parts = text.split(/(\s+)/);
        parts.forEach(part => {
          if (part) segments.push(part);
        });
      } else {
        segments.push(text);
      }
    } else if (React.isValidElement(node)) {
      // For React elements, keep them intact
      segments.push(node);
    } else if (Array.isArray(node)) {
      node.forEach(child => processNode(child));
    }
  };

  processNode(children);
  return segments;
};

const hasTransition = (
  variant?: Variant
): variant is TargetAndTransition & { transition?: Transition } => {
  if (!variant) return false;
  return typeof variant === "object" && "transition" in variant;
};

const createVariantsWithTransition = (
  baseVariants: Variants,
  transition?: Transition & { exit?: Transition }
): Variants => {
  if (!transition) return baseVariants;

  const { exit: _, ...mainTransition } = transition;

  return {
    ...baseVariants,
    visible: {
      ...baseVariants.visible,
      transition: {
        ...(hasTransition(baseVariants.visible) ? baseVariants.visible.transition : {}),
        ...mainTransition,
      },
    },
    exit: {
      ...baseVariants.exit,
      transition: {
        ...(hasTransition(baseVariants.exit) ? baseVariants.exit.transition : {}),
        ...mainTransition,
        staggerDirection: -1,
      },
    },
  };
};

export function TextEffect({
  children,
  per = "word",
  as = "p",
  variants,
  className,
  preset = "fade",
  delay = 0,
  speedReveal = 1,
  speedSegment = 1,
  trigger = true,
  onAnimationComplete,
  onAnimationStart,
  segmentWrapperClassName,
  containerTransition,
  segmentTransition,
  style,
}: TextEffectProps) {
  const segments = splitChildren(children, per);
  const MotionTag = motion[as as keyof typeof motion] as typeof motion.div;

  const baseVariants = preset
    ? presetVariants[preset]
    : { container: defaultContainerVariants, item: defaultItemVariants };

  const stagger = defaultStaggerTimes[per] / speedReveal;

  const baseDuration = 0.3 / speedSegment;

  const customStagger = hasTransition(variants?.container?.visible)
    ? (variants.container.visible as TargetAndTransition).transition?.staggerChildren
    : undefined;

  const customDelay = hasTransition(variants?.container?.visible)
    ? (variants.container.visible as TargetAndTransition).transition?.delayChildren
    : undefined;

  const computedVariants = {
    container: createVariantsWithTransition(variants?.container || baseVariants.container, {
      staggerChildren: customStagger ?? stagger,
      delayChildren: customDelay ?? delay,
      ...containerTransition,
      exit: {
        staggerChildren: customStagger ?? stagger,
        staggerDirection: -1,
      },
    }),
    item: createVariantsWithTransition(variants?.item || baseVariants.item, {
      duration: baseDuration,
      ...segmentTransition,
    }),
  };

  return (
    <AnimatePresence mode="popLayout">
      {trigger && (
        <MotionTag
          initial="hidden"
          animate="visible"
          exit="exit"
          variants={computedVariants.container}
          className={className}
          onAnimationComplete={onAnimationComplete}
          onAnimationStart={onAnimationStart}
          style={style}
        >
          {per !== "line" ? <span className="sr-only">{children}</span> : null}
          {segments.map((segment, index) => (
            <AnimationComponent
              key={`${per}-${index}-${React.isValidElement(segment) ? index : segment}`}
              segment={segment}
              variants={computedVariants.item}
              per={per}
              segmentWrapperClassName={segmentWrapperClassName}
            />
          ))}
        </MotionTag>
      )}
    </AnimatePresence>
  );
}
