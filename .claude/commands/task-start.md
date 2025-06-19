Start working on task $ARGUMENTS with proper setup and context.

**Pre-Task Setup:**
• Read task definition from `tasks/prd-$ARGUMENTS/_task.md`
• Review PRD context: `tasks/prd-$ARGUMENTS/_prd.md`
• Check tech spec requirements: `tasks/prd-$ARGUMENTS/_techspec.md`
• Understand dependencies from previous completed tasks

**Important Notes:**
• **ALWAYS** verify against PRD and tech specs - NEVER make assumptions
• **NEVER** use workarounds, especially in tests - implement proper solutions
• **MUST** follow all established project standards:

- Architecture patterns: @architecture.mdc
- Go coding standards: @go-coding-standards.mdc
- Testing requirements: @testing-standards.mdc
- API standards: @api-standards.mdc
- Security & quality: @quality-security.mdc

**Development Workflow:**
• Use `/project:analyze-changes` during development for code review
• Use `/project:rules-check` to validate against project standards
• Run `make lint` and `make test` frequently
• Complete with `/project:task-review` when ready

**Usage:** `/task:start <task-number>`
