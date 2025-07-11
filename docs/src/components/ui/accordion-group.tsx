"use client";

import { cn } from "@/lib/utils";
import * as React from "react";
import {
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
  Accordion as BaseAccordion,
} from "./accordion";

interface AccordionGroupProps {
  children: React.ReactNode;
  className?: string;
}

interface AccordionSectionProps {
  title: string;
  children: React.ReactNode;
}

// Wrapper component for individual accordion sections within a group
export function Accordion({ title, children }: AccordionSectionProps) {
  return (
    <BaseAccordion type="single" collapsible className="w-full">
      <AccordionItem value={`section-${title}`}>
        <AccordionTrigger className="text-base font-semibold">{title}</AccordionTrigger>
        <AccordionContent>
          <div className="prose prose-sm dark:prose-invert max-w-none">{children}</div>
        </AccordionContent>
      </AccordionItem>
    </BaseAccordion>
  );
}

export function AccordionGroup({ children, className }: AccordionGroupProps) {
  return <div className={cn("space-y-4", className)}>{children}</div>;
}
