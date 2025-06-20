Find and start the next available task automatically.

**Process:**
• Scan `tasks/prd-*/` directories for task files ([number]\_task.md)
• Identify the next uncompleted task (first unchecked checkbox)
• Read task definition, PRD, and tech spec for context
• Provide the same setup as `/project:task-start` but automatically

**Pre-Task Setup:**
• Read task definition from the identified next task file
• Review PRD context and tech spec requirements
• Understand dependencies from previous completed tasks
• Display task number, title, and description

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
• Run `make lint` and `make test` frequently
• Complete with `/project:task-review` when ready

**Usage:** `/task:next`
