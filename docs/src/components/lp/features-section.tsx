"use client";

// Extend TypeScript types for webkit properties and modern events
declare global {
  interface CSSStyleDeclaration {
    webkitScrollSnapType?: string;
  }

  interface HTMLElementEventMap {
    scrollend: Event;
  }
}

import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";
import {
  AnimatePresence,
  motion,
  useMotionValue,
  useScroll,
  useSpring,
  useTransform,
} from "framer-motion";
import { DynamicCodeBlock } from "fumadocs-ui/components/dynamic-codeblock";
import { Bot, Calendar, Cpu, Database, Globe, Radio, Workflow, Zap } from "lucide-react";
import { useEffect, useLayoutEffect, useRef, useState } from "react";
import { tv, type VariantProps } from "tailwind-variants";

const features = [
  {
    id: "workflow",
    title: "Workflow Management",
    description:
      "Design complex AI workflows using intuitive YAML templates with dynamic variables, directives, and Sprig functions for flexible orchestration—leveraging Temporal for durable, scalable execution.",
    icon: Workflow,
    code: `id: go-code-analyzer
version: 1.0.0
description: AI-powered Go code analysis workflow

tasks:
  - id: parallel_analysis
    type: parallel
    description: Run performance and best practices analysis
    strategy: wait_all
    tasks:
      - id: performance_analysis
        type: basic
        $use: agent(local::agents.#(id=="analyzer"))
        action: analyze
        with:
          file_path: "{{ .workflow.input.file_path }}"
          content: "{{ .workflow.input.file_content }}"

      - id: best_practices_analysis
        type: basic
        $use: agent(local::agents.#(id=="analyzer"))
        action: analyze
        with:
          file_path: "{{ .workflow.input.file_path }}"
          content: "{{ .workflow.input.file_content }}"`,
    language: "yaml",
  },
  {
    id: "agent",
    title: "Simple Agent Definition",
    description:
      "Create intelligent agents with LLM integration, tool support, memory management, and structured outputs for sophisticated AI behaviors, surpassing simpler frameworks like LangChain in enterprise readiness.",
    icon: Bot,
    code: `resource: agent
id: analyzer
description: Specialized in Go code analysis for performance and best practices
version: 1.0.0

config:
  $ref: global::models.#(provider=="openai")

instructions: |
  You are an expert Go developer specializing in performance optimization,
  best practices, concurrency, and memory management. Analyze code and
  provide actionable reports with specific, implementable suggestions.

tools:
  - $ref: local::tools.#(id=="write_file")

actions:
  - id: analyze
    prompt: |-
      Analyze the following Go code file and save the review as a markdown file.
      Create a comprehensive markdown report with proper structure.
      Return a summary of your findings after saving the review file.`,
    language: "yaml",
  },
  {
    id: "task",
    title: "Powerful Task System",
    description:
      "Execute diverse tasks including basic operations, aggregates, collections, routers, signals, and waits with built-in error handling and parallel processing, enabled by Go's native concurrency.",
    icon: Zap,
    code: `# Basic Task - Single operation execution
- id: analyze_code
  type: basic
  description: Analyze Go code for performance issues
  $use: agent(local::agents.#(id=="analyzer"))
  with:
    file_path: "{{ .input.file_path }}"
    content: "{{ .input.file_content }}"

# Parallel Task - Concurrent execution
- id: multi_analysis
  type: parallel
  strategy: wait_all
  tasks:
    - id: performance_check
      type: basic
      # ... performance analysis
    - id: security_scan
      type: basic
      # ... security analysis

# Collection Task - Process arrays
- id: process_files
  type: collection
  items: "{{ .input.file_list }}"
  task:
    type: basic
    $use: tool(local::tools.#(id=="file_processor"))
    with:
      file: "{{ .item }}"`,
    language: "yaml",
  },
  {
    id: "tools",
    title: "Agnostic Runtime for Tools",
    description:
      "Execute custom JavaScript/TypeScript code within tasks and agents using secure Bun runtime with granular permissions for safe, high-performance extensions—more flexible and efficient than rigid alternatives.",
    icon: Cpu,
    code: `import { z } from 'zod';

const inputSchema = z.object({
  name: z.string(),
  language: z.enum(['en', 'es', 'fr']).default('en')
});

export async function greeting(input: z.infer<typeof inputSchema>) {
  const { name, language } = inputSchema.parse(input);
  const greetings = {
    en: \`Hello, \${name}!\`,
    es: \`¡Hola, \${name}!\`,
    fr: \`Bonjour, \${name}!\`
  };

  return {
    message: greetings[language],
  };
}`,
    language: "typescript",
  },
  {
    id: "mcp",
    title: "MCP Integration",
    description:
      "Discover and execute external tools via Model Context Protocol (MCP) servers through our secure proxy server, enabling seamless integration with existing systems—more robust than basic API calls in competitors like Airflow.",
    icon: Globe,
    code: `# Global MCP server defined at workflow level
mcps:
  - id: context7
    transport: stdio
    command: "npx"
    args: ["-y", "@upstash/context7-mcp"]

# Agent that uses the global MCP server
agents:
  - id: docs-assistant
    config:
      $ref: global::models.#(provider=="openai")

    instructions: |
      You are a documentation assistant. Use the Context7 MCP
      server to fetch up-to-date library documentation.

    actions:
      - id: get-docs
        prompt: |
          Find documentation for: {{ .input.library }}
          Topic: {{ .input.topic }}
          Use the Context7 MCP tools to retrieve the latest docs.`,
    language: "yaml",
  },
  {
    id: "signals",
    title: "Signal-Based Events",
    description:
      "Implement event-driven architectures with signal tasks, wait tasks, and triggers for decoupled workflow coordination and real-time responses, outpacing no-code tools like Zapier in programmability.",
    icon: Radio,
    code: `# Signal Task - Publish events
- id: publish_analysis_complete
  type: signal
  signal:
    id: "analysis.complete"
    payload:
      analysis_id: "{{ .workflow.id }}"
      file_path: "{{ .input.file_path }}"

# Wait Task - Subscribe to events
- id: wait_for_approval
  type: wait
  wait_for: "analysis.approved"
  timeout: "5m"
  condition: 'signal.payload.analysis_id == "{{ .workflow.id }}"'
  processor:
    $use: tool(local::tools.#(id=="some_tool"))

# Event-triggered workflow
id: deployment-workflow
triggers:
  - type: signal
    name: "analysis.approved"`,
    language: "yaml",
  },
  {
    id: "scheduled",
    title: "Scheduled Workflows",
    description:
      "Create cron-job like automated workflows with built-in scheduling for recurring AI tasks and batch processing—powered by Temporal for reliable, fault-tolerant execution beyond basic schedulers in competitors.",
    icon: Calendar,
    code: `# Scheduled Workflow Configuration
id: daily-code-analysis
schedule:
  cron: "0 2 * * *"  # Daily at 2 AM
  timezone: "UTC"

# Batch processing with scheduling
tasks:
  - id: discover_repositories
    type: basic
    $use: tool(local::tools.#(id=="git_scanner"))
    with:
      scan_path: "/repos"

  - id: analyze_all_repos
    type: collection
    items: "{{ .tasks.discover_repositories.output.repositories }}"
    concurrency: 5  # Process 5 repos in parallel
    task:
      type: basic
      $use: agent(local::agents.#(id=="analyzer"))
      with:
        file_path: "{{ .item.path }}"
        content: "{{ .item.content }}"`,
    language: "yaml",
  },
  {
    id: "memory",
    title: "Memory Management",
    description:
      "Store and retrieve conversation history with configurable backends, flush strategies, and privacy controls for context-aware AI applications, integrated seamlessly with Temporal for fault-tolerant state management.",
    icon: Database,
    code: `# Memory Configuration
memory:
  - id: conversation_history
    type: token
    max_tokens: 4000
    key_template: "user:{{ .input.user_id }}:session:{{ .input.session_id }}"
    ttl: "24h"

# Using memory in agents
agents:
  - id: code_reviewer
    resource: agent
    memory:
      - id: conversation_history
        key: "user:{{ .input.user_id }}:session:{{ .input.session_id }}"
        mode: read-write
    config:
      $ref: global::models.#(provider=="openai")
    instructions: |
      You are a code reviewer with access to conversation history.
      Remember previous analysis and user preferences.`,
    language: "yaml",
  },
];

// Feature card styles using tailwind-variants with design tokens
const featureCard = tv({
  slots: {
    container: "p-1",
    card: "group relative flex cursor-pointer rounded-xl border overflow-hidden transition-all",
    content: "flex w-full gap-3 md:gap-4",
    icon: "flex aspect-square shrink-0 items-center justify-center rounded-lg transition-colors",
    textContainer: "min-w-0 flex-1",
    title: "transition-colors tracking-wide",
    description: "overflow-hidden",
    descriptionText: "text-muted-foreground",
  },
  variants: {
    state: {
      active: {
        container: "my-2",
        card: "border-border bg-accent shadow-sm px-4 py-4 md:px-5 md:py-5",
        content: "items-start",
        icon: "w-10 md:w-11 bg-primary text-primary-foreground",
        title: "font-medium text-foreground text-base md:text-lg lg:text-xl mb-2 leading-tight",
        descriptionText: "text-xs md:text-sm lg:text-base leading-relaxed",
      },
      inactive: {
        container: "",
        card: "border-transparent hover:border-border hover:bg-accent/30 px-3 py-1 md:px-4 md:py-1",
        content: "items-center",
        icon: "w-8 md:w-9 bg-muted text-muted-foreground",
        title: "text-muted-foreground text-sm md:text-base leading-tight",
        descriptionText: "text-xs md:text-sm lg:text-base leading-relaxed",
      },
    },
  },
  defaultVariants: {
    state: "inactive",
  },
});

// Animation constants
const ANIMATIONS = {
  duration: 0.2,
  spring: { stiffness: 200, damping: 25 },
  delays: { icon: 0.02, title: 0.04, description: 0.06 },
} as const;

// Feature card component with tailwind-variants
interface FeatureCardProps extends VariantProps<typeof featureCard> {
  feature: (typeof features)[0];
  isActive: boolean;
  onClick: () => void;
}

function FeatureCard({ feature, isActive, onClick }: FeatureCardProps) {
  const Icon = feature.icon;
  const styles = featureCard({ state: isActive ? "active" : "inactive" });

  return (
    <motion.div
      className={styles.container()}
      layout
      layoutId={`card-container-${feature.id}`}
      transition={{ type: "spring", ...ANIMATIONS.spring, duration: ANIMATIONS.duration }}
    >
      <motion.div
        className={styles.card()}
        onClick={onClick}
        layout
        layoutId={`card-${feature.id}`}
        animate={{
          scale: isActive ? 1 : 0.98,
          y: isActive ? 0 : 1,
        }}
        whileHover={
          isActive
            ? {}
            : {
                y: -1,
              }
        }
        whileTap={{ scale: isActive ? 0.995 : 0.975 }}
        transition={{ type: "spring", ...ANIMATIONS.spring, duration: ANIMATIONS.duration }}
      >
        <motion.div className={styles.content()} layout layoutId={`content-${feature.id}`}>
          <FeatureIcon icon={Icon} isActive={isActive} styles={styles} featureId={feature.id} />
          <FeatureContent feature={feature} isActive={isActive} styles={styles} />
        </motion.div>
      </motion.div>
    </motion.div>
  );
}

// Feature icon component
interface FeatureIconProps {
  icon: React.ComponentType<{ className?: string }>;
  isActive: boolean;
  styles: ReturnType<typeof featureCard>;
  featureId: string;
}

function FeatureIcon({ icon: Icon, isActive, styles, featureId }: FeatureIconProps) {
  return (
    <motion.div
      className={styles.icon()}
      layout
      layoutId={`icon-${featureId}`}
      animate={isActive ? { scale: [1, 1.05, 1] } : { scale: 1 }}
      transition={{ duration: ANIMATIONS.duration, delay: isActive ? ANIMATIONS.delays.icon : 0 }}
    >
      <Icon className={cn(isActive ? "size-5 md:size-6" : "size-4 md:size-5")} />
    </motion.div>
  );
}

// Feature content component
interface FeatureContentProps {
  feature: (typeof features)[0];
  isActive: boolean;
  styles: ReturnType<typeof featureCard>;
}

function FeatureContent({ feature, isActive, styles }: FeatureContentProps) {
  return (
    <motion.div className={styles.textContainer()} layout layoutId={`text-container-${feature.id}`}>
      <motion.h3
        className={styles.title()}
        layout
        layoutId={`title-${feature.id}`}
        animate={isActive ? { x: [0, 2, 0] } : { x: 0 }}
        transition={{
          duration: ANIMATIONS.duration,
          delay: isActive ? ANIMATIONS.delays.title : 0,
        }}
        suppressHydrationWarning
      >
        {feature.title}
      </motion.h3>
      <FeatureDescription
        description={feature.description}
        isActive={isActive}
        styles={styles}
        feature={feature}
      />
    </motion.div>
  );
}

// Feature description component
interface FeatureDescriptionProps {
  description: string;
  isActive: boolean;
  styles: ReturnType<typeof featureCard>;
  feature: (typeof features)[0];
}

function FeatureDescription({ description, isActive, styles, feature }: FeatureDescriptionProps) {
  return (
    <AnimatePresence>
      {isActive && (
        <motion.div
          className={styles.description()}
          layout
          layoutId={`description-${feature.id}`}
          initial={{ opacity: 0, height: 0 }}
          animate={{ opacity: 1, height: "auto" }}
          exit={{ opacity: 0, height: 0 }}
          transition={{
            duration: ANIMATIONS.duration,
            ease: "easeInOut",
            delay: ANIMATIONS.delays.description,
          }}
        >
          <motion.p
            className={styles.descriptionText()}
            initial={{ y: 5 }}
            animate={{ y: 0 }}
            exit={{ y: 5 }}
            transition={{ duration: ANIMATIONS.duration, delay: 0.08 }}
            suppressHydrationWarning
          >
            {description}
          </motion.p>
        </motion.div>
      )}
    </AnimatePresence>
  );
}

export default function FeaturesSection() {
  const [activeFeature, setActiveFeature] = useState(0);
  const [_, setActiveSlide] = useState(0);
  const dotsContainerRef = useRef<HTMLDivElement>(null);
  const dotsRowRef = useRef<HTMLDivElement>(null);
  const desktopDotsRowRef = useRef<HTMLDivElement>(null);
  const carouselRef = useRef<HTMLDivElement>(null);

  const [dotStep, setDotStep] = useState(0);

  // Track carousel scroll progress with Motion's useScroll
  const { scrollXProgress } = useScroll({
    container: carouselRef,
    axis: "x",
  });

  // Create indicator position based on scroll (mobile) or active feature (desktop)
  const scrollBasedX = useTransform(scrollXProgress, [0, 1], [0, dotStep * (features.length - 1)]);

  const clickBasedX = useMotionValue(0);

  // Mobile spring animation
  const springX = useSpring(scrollBasedX, {
    stiffness: 700,
    damping: 40,
    mass: 0.2,
  });

  // Desktop spring animation
  const desktopSpringX = useSpring(clickBasedX, {
    stiffness: 700,
    damping: 40,
    mass: 0.2,
  });

  // Update click-based position when activeFeature changes (for desktop)
  useEffect(() => {
    clickBasedX.set(activeFeature * dotStep);
  }, [activeFeature, dotStep, clickBasedX]);

  // Track previous active feature to skip movement flag on first render
  const prevActiveFeatureRef = useRef(activeFeature);
  useEffect(() => {
    if (prevActiveFeatureRef.current !== activeFeature) {
      // Only mark as moving when the active feature actually changes
      setIsDesktopMoving(true);
    }
    prevActiveFeatureRef.current = activeFeature;
  }, [activeFeature]);

  useEffect(() => {
    const unsubscribe = desktopSpringX.on("change", latest => {
      const target = clickBasedX.get();
      if (Math.abs(latest - target) <= 0.5) {
        setIsDesktopMoving(false);
      }
    });

    return () => {
      unsubscribe();
    };
  }, [desktopSpringX, clickBasedX]);

  // Track if user is actively scrolling vs at rest position
  const [isScrolling, setIsScrolling] = useState(false);
  const [isDesktopMoving, setIsDesktopMoving] = useState(false);

  // Measure distance between dots for accurate positioning
  useLayoutEffect(() => {
    const measure = () => {
      // Use desktop dots for measurement on large screens, mobile dots otherwise
      const isDesktop = window.innerWidth >= 1024;
      const row = isDesktop ? desktopDotsRowRef.current : dotsRowRef.current;
      if (!row) return;
      const dots = row.querySelectorAll<HTMLButtonElement>("button");
      if (dots.length < 2) return;
      const step = dots[1].offsetLeft - dots[0].offsetLeft;
      setDotStep(step);
    };

    // Use timeout to ensure DOM is fully rendered
    const timeoutId = setTimeout(measure, 0);
    measure();

    // Listen for orientation changes and layout changes
    window.addEventListener("resize", measure);
    window.addEventListener("orientationchange", measure);
    return () => {
      clearTimeout(timeoutId);
      window.removeEventListener("resize", measure);
      window.removeEventListener("orientationchange", measure);
    };
  }, []);

  useEffect(() => {
    const carousel = carouselRef.current;
    if (!carousel) return;

    const slides = Array.from(carousel.querySelectorAll<HTMLDivElement>("[data-slide-index]"));

    // Track scroll events to detect when user is actively scrolling
    let scrollTimeout: NodeJS.Timeout;
    const handleScroll = () => {
      setIsScrolling(true);
      clearTimeout(scrollTimeout);
      scrollTimeout = setTimeout(() => {
        setIsScrolling(false);
      }, 150); // Consider stopped after 150ms of no scroll
    };

    const handleScrollEnd = () => {
      setIsScrolling(false);
    };

    // Use viewport as root with centered margin to fix iOS horizontal IO bug
    const observer = new IntersectionObserver(
      entries => {
        entries.forEach(entry => {
          if (entry.isIntersecting) {
            const index = Number(entry.target.getAttribute("data-slide-index"));
            setActiveSlide(index);
          }
        });
      },
      {
        root: null, // Use viewport instead of carousel
        rootMargin: "0px -40% 0px -40%", // Central 20% strip
        threshold: 0.5, // Lower threshold for narrow phones
      }
    );

    // Add scroll listeners
    carousel.addEventListener("scroll", handleScroll);
    if ("onscrollend" in carousel) {
      carousel.addEventListener("scrollend", handleScrollEnd);
    }

    slides.forEach(slide => observer.observe(slide));
    return () => {
      observer.disconnect();
      carousel.removeEventListener("scroll", handleScroll);
      if ("onscrollend" in carousel) {
        carousel.removeEventListener("scrollend", handleScrollEnd);
      }
      clearTimeout(scrollTimeout);
    };
  }, []);

  const scrollToIndex = (index: number) => {
    const carousel = carouselRef.current;
    if (!carousel) return;

    const slides = carousel.querySelectorAll<HTMLDivElement>("[data-slide-index]");
    const targetSlide = slides[index];
    if (!targetSlide) return;

    // Disable snap temporarily to avoid jerky jump
    const originalSnapType = carousel.style.scrollSnapType;
    const originalWebkitSnapType = carousel.style.webkitScrollSnapType;
    carousel.style.scrollSnapType = "none";
    carousel.style.webkitScrollSnapType = "none";

    // Get computed gap for accurate positioning
    const gap = parseFloat(getComputedStyle(carousel).columnGap || "0");

    // Calculate offset so the slide is centered
    const left =
      targetSlide.offsetLeft - (carousel.clientWidth - targetSlide.clientWidth) / 2 - gap / 2;

    carousel.scrollTo({ left, behavior: "smooth" });

    // Utility to restore snapping
    const enableSnap = () => {
      carousel.style.scrollSnapType = originalSnapType || "";
      carousel.style.webkitScrollSnapType = originalWebkitSnapType || "";
      carousel.removeEventListener("scroll", scrollStopDetector);
    };

    // Fallback scroll detection for iOS < 17
    let scrollTimeout: NodeJS.Timeout;
    const scrollStopDetector = () => {
      clearTimeout(scrollTimeout);
      scrollTimeout = setTimeout(enableSnap, 150);
    };

    // Try modern scrollend event first, fallback to scroll detection
    const onScrollEnd = () => {
      enableSnap();
      carousel.removeEventListener("scrollend", onScrollEnd);
    };

    // Check if scrollend event is supported
    const supportsScrollEnd = "onscrollend" in window;
    if (supportsScrollEnd) {
      carousel.addEventListener("scrollend", onScrollEnd, { once: true });
    } else {
      carousel.addEventListener("scroll", scrollStopDetector);
    }

    // Safety timeout
    setTimeout(enableSnap, 600);
  };

  return (
    <section id="features" className="py-12 md:py-24 lg:py-32">
      <div className="container mx-auto px-4">
        <div className="mb-8 text-center md:mb-12">
          <Badge variant="secondary" className="mb-3">
            Powerful Features
          </Badge>
          <h2 className="!text-foreground max-w-2xl mx-auto text-3xl leading-tight md:text-4xl lg:text-5xl">
            Unlock Advanced Multi-agent Orchestration
          </h2>
          <p className="mx-auto mt-3 max-w-3xl text-sm text-muted-foreground md:mt-4 md:text-base">
            Create, deploy, and manage robust multi-agent systems effortlessly with Compozy’s
            open-source engine, unifying agents, tasks, tools, and signals into scalable workflows
            with YAML simplicity. Powered by Go’s high performance and Temporal’s fault-tolerant
            reliability, it tackles real-world challenges like cost optimization, debugging, and
            untrusted data, giving enterprises full infrastructure control.
          </p>
        </div>

        <div className="overflow-visible my-24">
          <div className="mx-auto grid max-w-6xl gap-6 lg:grid-cols-[320px_1fr] lg:gap-8 xl:grid-cols-[384px_1fr] xl:gap-16 2xl:grid-cols-[400px_1fr]">
            {/* Mobile Carousel */}
            <div
              ref={carouselRef}
              className="lg:hidden col-span-2 scrollbar-none flex snap-x snap-mandatory gap-3 overflow-x-auto [-ms-overflow-style:'none'] [scrollbar-width:none] [&::-webkit-scrollbar]:hidden [overscroll-behavior-x:contain] [-webkit-overflow-scrolling:touch]"
            >
              {features.map((feature, index) => {
                const Icon = feature.icon;
                return (
                  <div
                    key={feature.id}
                    data-slide-index={index}
                    className="relative h-[min(30rem,65vh)] w-[min(92vw,100%)] shrink-0 cursor-pointer snap-center snap-always overflow-hidden rounded-xl border border-border bg-background first:ml-[4vw] last:mr-[4vw]"
                  >
                    <div className="h-full w-full p-4 flex flex-col">
                      <div className="mb-4">
                        <div className="flex items-center gap-3 mb-3">
                          <div className="flex size-10 items-center justify-center rounded-lg bg-primary p-2 text-primary-foreground">
                            <Icon className="size-5" />
                          </div>
                          <div>
                            <h3 className="text-lg font-semibold text-foreground">
                              {feature.title}
                            </h3>
                          </div>
                        </div>
                        <p className="text-sm text-muted-foreground leading-relaxed px-1">
                          {feature.description}
                        </p>
                      </div>
                      <div className="flex-1 overflow-auto">
                        <DynamicCodeBlock
                          lang={feature.language}
                          code={feature.code}
                          options={{
                            themes: {
                              light: "vitesse-light",
                              dark: "vitesse-dark",
                            },
                          }}
                        />
                      </div>
                    </div>
                  </div>
                );
              })}
            </div>

            {/* Mobile Dots with sliding indicator */}
            <div ref={dotsContainerRef} className="col-span-2 mb-4 flex justify-center lg:hidden">
              {/* Inner wrapper creates proper positioning context */}
              <div ref={dotsRowRef} className="relative flex gap-2">
                {/* Static grey dots */}
                {features.map((_, index) => (
                  <button
                    key={index}
                    className="size-2 rounded-full bg-muted hover:bg-muted-foreground/50"
                    onClick={() => {
                      scrollToIndex(index);
                    }}
                    aria-label={`Go to slide ${index + 1}`}
                  />
                ))}

                {/* Sliding green indicator */}
                <motion.div className="absolute top-0 left-0 h-2" style={{ x: springX }}>
                  <motion.span
                    className="block h-full rounded-full bg-primary"
                    initial={{ width: "8px" }}
                    animate={{
                      width: isScrolling ? "8px" : "24px",
                      marginLeft: isScrolling ? "0px" : "-8px",
                    }}
                    transition={{
                      type: "spring",
                      stiffness: 400,
                      damping: 25,
                      duration: 0.08,
                    }}
                  />
                </motion.div>
                {/* End sliding indicator */}
              </div>
            </div>

            {/* Desktop Feature List */}
            <div className="hidden lg:flex lg:flex-col">
              {features.map((feature, index) => (
                <FeatureCard
                  key={feature.id}
                  feature={feature}
                  isActive={index === activeFeature}
                  onClick={() => setActiveFeature(index)}
                />
              ))}
            </div>

            {/* Desktop Code Display */}
            <div className="relative hidden lg:flex lg:flex-col">
              <motion.div
                className="overflow-hidden rounded-xl border border-border shadow-sm bg-background flex flex-col h-fit"
                layout
                transition={{ type: "spring", stiffness: 300, damping: 30 }}
              >
                <AnimatePresence mode="wait">
                  <motion.div
                    key={`header-${activeFeature}`}
                    className="p-6 pb-4 flex-shrink-0"
                    initial={{ opacity: 0, y: -10 }}
                    animate={{ opacity: 1, y: 0 }}
                    exit={{ opacity: 0, y: 10 }}
                    transition={{ duration: 0.2 }}
                  >
                    <div className="flex items-center gap-3">
                      <motion.div
                        className="flex aspect-square w-10 items-center justify-center rounded-lg bg-primary text-primary-foreground"
                        initial={{ scale: 0, rotate: -180 }}
                        animate={{ scale: 1, rotate: 0 }}
                        transition={{
                          type: "spring",
                          stiffness: 300,
                          damping: 25,
                          delay: 0.1,
                        }}
                      >
                        {(() => {
                          const Icon = features[activeFeature].icon;
                          return <Icon className="size-5" />;
                        })()}
                      </motion.div>
                      <motion.h3
                        className="text-xl font-semibold text-foreground"
                        initial={{ opacity: 0, x: -10 }}
                        animate={{ opacity: 1, x: 0 }}
                        transition={{ delay: 0.15, duration: 0.3 }}
                      >
                        {features[activeFeature].title}
                      </motion.h3>
                    </div>
                  </motion.div>
                </AnimatePresence>

                <AnimatePresence mode="wait">
                  <motion.div
                    key={`code-${activeFeature}`}
                    className="px-6 pb-6 overflow-auto"
                    initial={{ opacity: 0, y: 20 }}
                    animate={{ opacity: 1, y: 0 }}
                    exit={{ opacity: 0, y: -20 }}
                    transition={{
                      duration: 0.3,
                      delay: 0.1,
                      ease: "easeInOut",
                    }}
                  >
                    <DynamicCodeBlock
                      lang={features[activeFeature].language}
                      code={features[activeFeature].code}
                      options={{
                        themes: {
                          light: "vitesse-light",
                          dark: "vitesse-dark",
                        },
                      }}
                    />
                  </motion.div>
                </AnimatePresence>
              </motion.div>

              {/* Desktop Dots with sliding indicator */}
              <div className="mt-6 flex justify-center">
                {/* Inner wrapper creates proper positioning context */}
                <div ref={desktopDotsRowRef} className="relative flex gap-2">
                  {/* Static grey dots */}
                  {features.map((_, index) => (
                    <button
                      key={index}
                      className="size-2 rounded-full bg-muted hover:bg-muted-foreground/50"
                      onClick={() => setActiveFeature(index)}
                      aria-label={`Go to slide ${index + 1}`}
                    />
                  ))}

                  {/* Sliding green indicator */}
                  <motion.div className="absolute top-0 left-0 h-2" style={{ x: desktopSpringX }}>
                    <motion.span
                      className="block h-full rounded-full bg-primary"
                      initial={{ width: "8px" }}
                      animate={{
                        width: isDesktopMoving ? "8px" : "24px",
                        marginLeft: isDesktopMoving ? "0px" : "-8px",
                      }}
                      transition={{
                        type: "spring",
                        stiffness: 400,
                        damping: 25,
                        duration: 0.08,
                      }}
                    />
                  </motion.div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </section>
  );
}
