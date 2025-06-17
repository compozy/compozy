Analyze current work in progress changes using Zen MCP and project rules.  
**Rules Applied:**
• @go-coding-standards.mdc - Code style, error handling, naming conventions
• @architecture.mdc - Clean architecture, dependency inversion
• @testing-standards.mdc - Test coverage, mock patterns
• @api-standards.mdc - RESTful design, documentation
• @quality-security.mdc - Security patterns, performance

**Process:**
• Gets modified files from `git diff --name-only` (staged + unstaged)
• Filters out non-relevant files (generated, vendor, etc.)
• Focuses on files related to current scope of work
• Uses Zen MCP codereview tool for targeted analysis
• Provides actionable improvements for work in progress
• Suggests refactoring before committing changes

**Usage:** `/analyze:changes`
