"use client";

/** The comparison's check mark, popping in row by row the first time the table is seen. */
import { Check } from "@compozy/ui/icons";
import { m } from "motion/react";

export function PopCheck({ index }: { index: number }) {
  return (
    <m.span
      className="inline-block"
      initial={{ scale: 0 }}
      whileInView={{ scale: 1 }}
      viewport={{ once: true }}
      transition={{
        delay: 0.1 * index,
        type: "spring",
        stiffness: 320,
        damping: 14,
      }}
    >
      <Check aria-label="Yes" className="size-5 text-white" />
    </m.span>
  );
}
