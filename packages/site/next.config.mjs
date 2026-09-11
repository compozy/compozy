import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { createMDX } from "fumadocs-mdx/next";

const withMDX = createMDX();
const repoRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..", "..");

/** @type {import('next').NextConfig} */
const config = {
  // React Compiler (Next 16 stable): auto-memoization so the react-doctor
  // react-compiler-no-manual-memoization rule holds across web + ui + site.
  reactCompiler: true,
  reactStrictMode: true,
  trailingSlash: true,
  redirects() {
    return [
      {
        source: "/blog/crewai-alternatives/",
        destination: "/blog/git-repository-briefing-python/",
        permanent: true,
      },
      {
        source: "/blog/langchain-alternatives-production-ai-agents/",
        destination: "/blog/agent-framework-architecture/",
        permanent: true,
      },
      {
        source: "/blog/langgraph-alternatives/",
        destination: "/blog/ai-agent-retries-idempotency/",
        permanent: true,
      },
      {
        source: "/blog/what-is-an-os-for-ai-agents/",
        destination: "/blog/defining-agent-sessions-compozyos/",
        permanent: true,
      },
      {
        source: "/blog/orca-vs-openhands/",
        destination: "/blog/cursor-vs-claude-code/",
        permanent: true,
      },
    ];
  },
  turbopack: {
    root: repoRoot,
  },
  outputFileTracingRoot: repoRoot,
  outputFileTracingIncludes: {
    "/api/search": [
      "../../extensions/bridges/*/extension.toml",
      "../../extensions/spec-cycle/extension.json",
      "../../extensions/spec-cycle/agents/*/AGENT.md",
      "../../extensions/spec-cycle/loops/*/loop.yaml",
      "../../extensions/spec-cycle/skills/*/SKILL.md",
      "../../extensions/open-design/extension.json",
      "../../extensions/open-design/loops/*/loop.yaml",
      "../../extensions/open-design/agents/*/AGENT.md",
      "../../extensions/open-design/skills/*/SKILL.md",
      "../../skills/**/SKILL.md",
    ],
  },
  images: {
    formats: ["image/avif", "image/webp"],
    qualities: [75, 90],
  },
};

export default withMDX(config);
