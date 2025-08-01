"use client";

import { MagicCard } from "@/components/magicui/magic-card";
import { cn } from "@/lib/utils";
import Image from "next/image";
import React from "react";
import { tv, type VariantProps } from "tailwind-variants";
import styles from "./bento-grid.module.css";

// Tailwind Variants for Bento Grid
const bentoGrid = tv({
  base: "grid w-full auto-rows-auto grid-cols-1 gap-6 lg:grid-cols-12",
});

// Tailwind Variants for base Bento Card - only truly reusable styles
const bentoCard = tv({
  base: "group relative flex h-full flex-col justify-between rounded-2xl overflow-hidden @container",
});

// Type definitions
export interface BentoCardProps
  extends React.HTMLAttributes<HTMLDivElement>,
    VariantProps<typeof bentoCard> {
  background?: React.ReactNode;
  icon?: React.ReactNode;
}

// Base Bento Grid Component
const BentoGrid = React.forwardRef<
  HTMLDivElement,
  React.HTMLAttributes<HTMLDivElement> & VariantProps<typeof bentoGrid>
>(({ className, children, ...props }, ref) => (
  <div ref={ref} className={bentoGrid({ className })} {...props}>
    {children}
  </div>
));
BentoGrid.displayName = "BentoGrid";

// Base Bento Card Component
const BentoCard = ({ className, background, children, icon, ...props }: BentoCardProps) => {
  return (
    <MagicCard
      className={bentoCard({ className })}
      gradientSize={300}
      gradientColor="var(--primary)"
      gradientOpacity={0.1}
      gradientFrom="oklch(from var(--primary) calc(l + 0.1) c h)"
      gradientTo="oklch(from var(--primary) calc(l - 0.1) c h)"
      innerClassName="h-full"
    >
      {background && <div className="absolute inset-0 z-0">{background}</div>}
      {icon && <div className="absolute inset-0 z-0">{icon}</div>}
      <div className="relative z-10 flex h-full flex-col" {...props}>
        {children}
      </div>
    </MagicCard>
  );
};
BentoCard.displayName = "BentoCard";


// Card 1 Component - Enterprise-grade Reliability
const ReliabilityCard: React.FC = () => {
  return (
    <BentoCard
      className="md:col-span-4 min-h-[28rem] @md:min-h-[30rem] p-8"
      background={<div className={styles.card1Bg} />}
      icon={
        <Image
          src="/home/card1-icon.svg"
          alt=""
          width={195}
          height={80}
          className={styles.card1Icon}
          quality={100}
        />
      }
    >
      <div className="flex flex-col gap-10">
        <div className="flex flex-col gap-4">
          <div>
            <h3 className="text-4xl font-light leading-tight">
              <span className="text-gradient-gray">Enterprise-grade</span>
            </h3>
            <h3 className="text-3xl font-medium leading-tight mt-1">
              <span className="text-gradient-green">Reliability</span>
            </h3>
          </div>
          <p className="text-lg text-[#C1C1C1] leading-[1.78] max-w-[90%]">
            Compozy is built on Temporal—a proven workflow engine trusted by Fortune 500
            companies—delivering durable, fault-tolerant workflows that withstand failures
          </p>
        </div>
      </div>
    </BentoCard>
  );
};

// Card 2 Component - Scalable and Secure
const ScalableCard: React.FC = () => {
  return (
    <BentoCard
      className="md:col-span-4 min-h-[28rem] @md:min-h-[30rem] p-8"
      background={<div className={styles.card2Bg} />}
      icon={
        <Image
          src="/home/card2-icon.svg"
          alt=""
          width={254}
          height={270}
          className={styles.card2Icon}
          quality={100}
        />
      }
    >
      <div className="flex flex-col gap-10">
        <div className="flex flex-col gap-4 mt-32">
          <div>
            <h3 className="text-4xl font-light leading-tight">
              <span className="text-gradient-gray">Scalable</span>
            </h3>
            <h3 className="text-3xl font-medium leading-tight mt-1">
              <span className="text-gradient-green">and Secure</span>
            </h3>
          </div>
          <p className="text-lg text-[#C1C1C1] leading-[1.78] max-w-[85%]">
            Designed with Go at its core, Compozy enables secure, scalable AI workflows and gives
            developers greater control than no-code platforms.
          </p>
        </div>
      </div>
    </BentoCard>
  );
};

// Card 3 Component - Open Source Advantage
const OpenSourceCard: React.FC = () => {
  return (
    <BentoCard
      className="md:col-span-4 min-h-[28rem] @md:min-h-[30rem] p-8"
      background={<div className={styles.card3Bg} />}
      icon={
        <Image
          src="/home/card3-icon.svg"
          alt=""
          width={252}
          height={258}
          className={styles.card3Icon}
          quality={100}
        />
      }
    >
      <div className="flex flex-col gap-4">
        <div>
          <h3 className="text-[31px] font-light leading-tight">
            <span className="text-gradient-gray">Open Source</span>
          </h3>
          <h3 className="text-3xl font-medium leading-tight">
            <span className="text-gradient-green">Advantage</span>
          </h3>
        </div>
        <p className="text-lg text-[#C1C1C1] leading-[1.78] max-w-[80%] mt-4">
          Fully customizable and free from vendor lock-in, our multi-agent platform delivers robust
          enterprise features right out of the box.
        </p>
      </div>
    </BentoCard>
  );
};

// Card 4 Component - Full-Featured Orchestration
const OrchestrationCard: React.FC = () => {
  return (
    <BentoCard
      className="md:col-span-6 min-h-[37rem] p-12"
      background={<div className={styles.card4Bg} />}
      icon={
        <Image
          src="/home/card4-icon.svg"
          alt=""
          width={357}
          height={267}
          className={styles.card4Icon}
          quality={100}
        />
      }
    >
      <div className="flex flex-col gap-4">
        <div>
          <h3 className="text-[30px] font-medium leading-tight">
            <span className="text-gradient-green">Full-Featured</span>
          </h3>
          <h3 className="text-[37px] font-light leading-tight">
            <span className="text-gradient-gray">Orchestration</span>
          </h3>
        </div>
        <p className="text-lg text-[#C1C1C1] leading-[1.78] max-w-[90%] mt-4">
          Offering support for AI agents, a broad spectrum of task types, and custom JS/TS tooling
          via a modern runtime, our platform seamlessly integrates with external systems, handles
          signals, manages scheduled workflows, and advanced memory management.
        </p>
      </div>
    </BentoCard>
  );
};

// Card 5 Component - Language-agnostic Design approach
const LanguageAgnosticCard: React.FC = () => {
  return (
    <BentoCard
      className="md:col-span-6 min-h-[37rem] p-12"
      background={<div className={styles.card5Bg} />}
      icon={
        <Image
          src="/home/card5-icon.svg"
          alt=""
          width={260}
          height={252}
          className={styles.card5Icon}
          quality={100}
        />
      }
    >
      <div className="flex flex-col gap-4">
        <div>
          <h3 className="text-[30px] font-medium leading-tight">
            <span className="text-gradient-green">Language-agnostic</span>
          </h3>
          <h3 className="text-[37px] font-light leading-tight">
            <span className="text-gradient-gray">by Design</span>
          </h3>
        </div>
        <p className="text-lg text-[#C1C1C1] leading-[1.78] max-w-[90%] mt-4">
          Our declarative YAML-based approach makes workflows easy for LLMs to learn and generate,
          while enabling future multi-language support—all powered by Go's efficient backend core as
          the platform foundation, offering greater accessibility than code-heavy competitors.
        </p>
      </div>
    </BentoCard>
  );
};

// Main Bento Grid Component
export default function CompozyBentoGrid() {
  return (
    <section
      className="w-full px-4 sm:px-6 lg:px-8 py-16 md:py-24 lg:py-32"
      aria-labelledby="features-title"
    >
      <h2 id="features-title" className="sr-only">
        Compozy Platform Features
      </h2>

      <div className="container">
        <BentoGrid>
          <ReliabilityCard />
          <ScalableCard />
          <OpenSourceCard />
          <OrchestrationCard />
          <LanguageAgnosticCard />
        </BentoGrid>
      </div>
    </section>
  );
}

export { BentoCard, BentoGrid };
