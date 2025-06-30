Find and start the next available task automatically.

**Process:**
• Scan `tasks/prd-*/` directories for task files
• Identify the next uncompleted task (first unchecked checkbox)
• Read task definition, PRD, and tech spec for context
• Provide the same setup as `/project:task-start` but automatically

**Pre-Task Setup:**
• Read task definition from `tasks/prd-$ARGUMENTS/_task.md`
• Review PRD context: `tasks/prd-$ARGUMENTS/_prd.md`
• Check tech spec requirements: `tasks/prd-$ARGUMENTS/_techspec.md`
• Read `tasks/prd-$ARGUMENTS/[num]_task.md` file to understand what's need to be done in the task.
• Understand dependencies from previous completed tasks

**Important Notes:**
• **ALWAYS** verify against PRD, tech specs and task file - NEVER make assumptions
• **NEVER** use workarounds, especially in tests - implement proper solutions
• **MUST** follow all established project standards:
• **NEVER** finish the task without following the `.cursor/rules/task-review.mdc` process

- Architecture patterns: @architecture.mdc
- Go coding standards: @go-coding-standards.mdc
- Testing requirements: @testing-standards.mdc
- API standards: @api-standards.mdc
- Security & quality: @quality-security.mdc

**Usage:** `/task:next`
