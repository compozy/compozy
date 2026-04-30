---
title: 'Major release: Complete AI-assisted development pipeline'
type: feature
---

Major release introducing Compozy as a comprehensive AI-assisted development orchestration platform.

**New Features:**
- Multi-agent orchestration for 40+ AI coding agents (Claude Code, Codex, Cursor, Droid, OpenCode, Pi, Gemini)
- End-to-end workflow pipeline: idea → PRD → TechSpec → Tasks → Execution → Review
- Codebase-aware task enrichment via parallel exploration
- Reusable agent system with .compozy/agents/ directory structure
- Provider-agnostic reviews (CodeRabbit, GitHub, AI-powered) with automatic thread resolution
- Workflow memory with context persistence and automatic compaction
- Extension system with TypeScript and Go SDKs for custom hooks and providers

**Installation:** Available via Homebrew, NPM, Go install, or from source.

**Architecture:** Single binary, local-first, embeddable Go package with markdown-based artifacts.
