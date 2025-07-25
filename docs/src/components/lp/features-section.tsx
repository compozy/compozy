"use client";

import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";
import { AnimatePresence, motion } from "framer-motion";
import { DynamicCodeBlock } from "fumadocs-ui/components/dynamic-codeblock";
import { Bot, Calendar, Cpu, Database, Globe, Radio, Workflow, Zap } from "lucide-react";
import { useState } from "react";
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
        $use: agent(local::agents.#(id=="go_code_analyzer"))
        action: analyze_performance
        with:
          file_path: "{{ .workflow.input.file_path }}"
          output_path: "{{ .workflow.input.output_path }}"

      - id: best_practices_analysis
        type: basic
        $use: agent(local::agents.#(id=="go_code_analyzer"))
        action: analyze_best_practices
        with:
          file_path: "{{ .workflow.input.file_path }}"
          output_path: "{{ .workflow.input.output_path }}"`,
    language: "yaml",
  },
  {
    id: "agent",
    title: "Agent Integration",
    description:
      "Create intelligent agents with LLM integration, tool support, memory management, and structured outputs for sophisticated AI behaviors, surpassing simpler frameworks like LangChain in enterprise readiness.",
    icon: Bot,
    code: `resource: agent
id: go_code_analyzer
description: AI agent specialized in Go code analysis
version: 1.0.0

config:
  provider: anthropic
  model: claude-3-opus-20240229
  api_key: "{{ .env.CLAUDE_API_KEY }}"

instructions: |
  You are an expert Go developer and code reviewer with deep knowledge of:
  - Go performance optimization and profiling
  - Go best practices and idiomatic patterns
  - Concurrent programming in Go
  - Memory management and garbage collection

  Your role is to analyze Go code and provide detailed, actionable reports.
  Always provide specific, implementable suggestions with code examples.

tools:
  - $ref: local::tools.#(id=="claude_code_analyzer")

actions:
  - id: analyze_performance
    prompt: |
      Analyze the Go file: {{ .input.file_path }} for performance issues.
      Create a detailed markdown report with optimization recommendations.`,
    language: "yaml",
  },
  {
    id: "task",
    title: "Task System",
    description:
      "Execute diverse tasks including basic operations, aggregates, collections, routers, signals, and waits with built-in error handling and parallel processing, enabled by Go's native concurrency.",
    icon: Zap,
    code: `# Basic Task - Single operation execution
- id: analyze_code
  type: basic
  description: Analyze Go code for performance issues
  $use: agent(local::agents.#(id=="code_analyzer"))
  with:
    code_path: "{{ .input.file_path }}"

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
    $use: tool(file_processor)
    with:
      file: "{{ .item }}"`,
    language: "yaml",
  },
  {
    id: "tools",
    title: "Custom Tools & Runtime",
    description:
      "Execute custom JavaScript/TypeScript code within tasks and agents using secure Bun runtime with granular permissions for safe, high-performance extensions—more flexible and efficient than rigid alternatives.",
    icon: Cpu,
    code: `// TypeScript tool with secure Bun runtime
import { readFile } from "fs/promises";
import { existsSync } from "fs";

interface CodeAnalyzerInput {
  file_path: string;
  analysis_type: "performance" | "security" | "style";
}

interface CodeAnalyzerOutput {
  analysis_result: string;
  issues_found: number;
  suggestions: string[];
}

export async function analyzeCode(
  input: CodeAnalyzerInput
): Promise<CodeAnalyzerOutput> {
  // Secure file access with Bun permissions
  if (!existsSync(input.file_path)) {
    throw new Error(\`File not found: \${input.file_path}\`);
  }

  const code = await readFile(input.file_path, "utf8");

  // Analysis logic here
  return {
    analysis_result: "Analysis complete",
    issues_found: 3,
    suggestions: ["Use interfaces", "Add error handling"]
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
    code: `# MCP Server Configuration
mcp:
  servers:
    - name: filesystem-tools
      transport:
        type: stdio
        command: "npx"
        args: ["@anthropic-ai/mcp-server-filesystem"]
        cwd: "/tmp"

    - name: github-integration
      transport:
        type: http/sse
        url: "http://localhost:3001/sse"
      auth:
        type: bearer
        token: "{{ .env.GITHUB_TOKEN }}"

# Using MCP tools in workflows
- id: analyze_repository
  type: basic
  $use: mcp_tool(filesystem-tools::read_file)
  with:
    path: "{{ .input.repo_path }}/README.md"

- id: create_issue
  type: basic
  $use: mcp_tool(github-integration::create_issue)
  with:
    title: "Code analysis results"
    body: "{{ .tasks.analyze_repository.output }}"`,
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
  signal_name: "analysis.complete"
  payload:
    analysis_id: "{{ .workflow.execution_id }}"
    file_path: "{{ .input.file_path }}"
    results: "{{ .tasks.analyze_code.output }}"
    timestamp: "{{ now }}"

# Wait Task - Subscribe to events
- id: wait_for_approval
  type: wait
  signal_name: "analysis.approved"
  timeout: "5m"
  filter: |
    .payload.analysis_id == "{{ .workflow.execution_id }}"

# Event-driven workflow coordination
- id: process_after_approval
  type: basic
  depends_on: [wait_for_approval]
  $use: tool(deploy_changes)
  with:
    approved_analysis: "{{ .tasks.wait_for_approval.output }}"`,
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
    $use: tool(git_scanner)
    with:
      scan_path: "/repos"

  - id: analyze_all_repos
    type: collection
    items: "{{ .tasks.discover_repositories.output.repositories }}"
    concurrency: 5  # Process 5 repos in parallel
    task:
      type: basic
      $use: agent(code_analyzer)
      with:
        repo_path: "{{ .item.path }}"

  - id: generate_report
    type: basic
    $use: tool(report_generator)
    with:
      analysis_results: "{{ .tasks.analyze_all_repos.output }}"
      output_path: "/reports/daily-{{ .schedule.date }}.md"`,
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

  - id: analysis_cache
    type: buffer
    max_messages: 50
    key_template: "analysis:{{ .input.file_hash }}"
    ttl: "7d"
    flush_strategy: "lru"

# Using memory in agents
agents:
  - id: code_reviewer
    memory:
      - $ref: local::memory.#(id=="conversation_history")
    config:
      provider: anthropic
      model: claude-3-sonnet
    instructions: |
      You are a code reviewer with access to conversation history.
      Remember previous analysis and user preferences.

# Privacy controls
privacy:
  pii_detection: true
  redaction_patterns:
    - "\\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\\.[A-Z|a-z]{2,}\\b"  # Email
    - "\\b\\d{3}-\\d{2}-\\d{4}\\b"  # SSN`,
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
    title: "transition-colors leading-tight tracking-wide",
    description: "overflow-hidden",
    descriptionText: "text-muted-foreground leading-relaxed",
  },
  variants: {
    state: {
      active: {
        container: "my-2",
        card: "border-border bg-accent shadow-sm px-4 py-4 md:px-5 md:py-5",
        content: "items-start",
        icon: "w-10 md:w-11 bg-primary text-primary-foreground",
        title: "font-medium text-foreground text-base md:text-lg lg:text-xl mb-2",
        descriptionText: "text-xs md:text-sm lg:text-base",
      },
      inactive: {
        container: "",
        card: "border-transparent hover:border-border hover:bg-accent/30 px-3 py-1 md:px-4 md:py-1",
        content: "items-center",
        icon: "w-8 md:w-9 bg-muted text-muted-foreground",
        title: "text-muted-foreground text-sm md:text-base",
        descriptionText: "text-xs md:text-sm lg:text-base",
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

  return (
    <section className="py-12 md:py-24 lg:py-32">
      <div className="container mx-auto px-4">
        <div className="mb-8 text-center md:mb-12">
          <Badge variant="secondary" className="mb-3">
            Powerful Features
          </Badge>
          <h2 className="!text-foreground text-3xl leading-tight md:text-4xl lg:text-5xl">
            Powerful Features for AI Orchestration
          </h2>
          <p className="mx-auto mt-3 max-w-3xl text-sm text-muted-foreground md:mt-4 md:text-base">
            Build, deploy, and manage powerful AI-powered applications effortlessly. Compozy is the
            open-source orchestration engine that combines agents, tasks, tools, and signals for
            seamless AI automation—powered by Go for unmatched speed and Temporal for
            enterprise-grade reliability.
          </p>
        </div>

        <div className="overflow-visible my-24">
          <div className="mx-auto grid max-w-6xl gap-6 md:grid-cols-[320px_1fr] md:gap-8 lg:grid-cols-[384px_1fr] lg:gap-16 xl:grid-cols-[400px_1fr]">
            {/* Mobile Carousel */}
            <div className="md:hidden col-span-2 scrollbar-none flex snap-x snap-mandatory gap-3 overflow-x-auto [-ms-overflow-style:'none'] [scrollbar-width:none] [&::-webkit-scrollbar]:hidden">
              {features.map((feature, index) => {
                const Icon = feature.icon;
                return (
                  <div
                    key={feature.id}
                    className="relative h-[min(30rem,65vh)] w-[min(100%,100vw)] shrink-0 cursor-pointer snap-center overflow-hidden rounded-xl border border-border bg-background"
                  >
                    <div className="h-full w-full p-4 flex flex-col">
                      <div className="flex items-center gap-3 mb-4">
                        <div className="flex size-10 items-center justify-center rounded-lg bg-primary p-2 text-primary-foreground">
                          <Icon className="size-5" />
                        </div>
                        <div>
                          <h3 className="text-lg font-semibold text-foreground">{feature.title}</h3>
                        </div>
                      </div>
                      <div className="flex-1 overflow-hidden">
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

            {/* Mobile Dots */}
            <div className="md:hidden col-span-2 mb-4 flex justify-center gap-2">
              {features.map((_, index) => (
                <button
                  key={index}
                  className={cn(
                    "size-2 rounded-full transition-all",
                    index === 0 ? "w-6 bg-primary" : "bg-muted hover:bg-muted-foreground/50"
                  )}
                  aria-label={`Go to slide ${index + 1}`}
                />
              ))}
            </div>

            {/* Desktop Feature List */}
            <div className="hidden md:flex md:flex-col">
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
            <div className="relative hidden md:flex md:flex-col">
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

              {/* Desktop Dots */}
              <div className="mt-4 flex justify-center gap-2">
                {features.map((_, index) => (
                  <button
                    key={index}
                    className={cn(
                      "size-2 rounded-full transition-all",
                      index === activeFeature
                        ? "w-6 bg-primary"
                        : "bg-muted hover:bg-muted-foreground/50"
                    )}
                    onClick={() => setActiveFeature(index)}
                    aria-label={`Go to slide ${index + 1}`}
                  />
                ))}
              </div>
            </div>
          </div>
        </div>
      </div>
    </section>
  );
}
