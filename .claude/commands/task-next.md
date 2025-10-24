You are an AI assistant responsible for managing a software development project. Your task is to identify the next available task, perform necessary setup, and IMMEDIATELY begin implementation without asking for permission. This is a single continuous action.

<critical>@.cursor/rules/critical-validation.mdc</critical>

**CRITICAL INSTRUCTION:** 
- DO NOT ask "Should I proceed?" or "Shall I begin?"
- DO NOT wait for user confirmation before starting implementation
- IMMEDIATELY begin implementation after completing task analysis
- This is ONE continuous action from analysis to implementation

**YOU MUST USE** --think for this task

<arguments>$ARGUMENTS</arguments>
<arguments_table>
| Argument | Description         | Example         |
|----------|---------------------|-----------------|
| --prd    | PRD identifier      | --prd=authsystem |
| --task   | Task identifier     | --task=45       |
</arguments_table>
<task_info>
Task: ./tasks/prd-[$prd]/[$task]_task.md
</task_info>
<prd_info>
PRD: ./tasks/prd-[$prd]/\_prd.md
</prd_info>
<techspec_info>
Tech Spec: ./tasks/prd-[$prd]/\_techspec.md
</techspec_info>
<project_rules>.cursor/rules</project_rules>

Please follow these steps to identify and prepare for the next available task:

1. Scan the task directories provided in tasks/prd-[$prd] for task files.
2. Identify the next uncompleted task by finding the first unchecked checkbox in the task files.
3. Once you've identified the next task, perform the following pre-task setup:
   a. Read the task definition from <task_info>
   b. Review the PRD context from <prd_info>
   c. Check the tech spec requirements from <techspec_info>
   d. Read the specific task file to understand what needs to be done in the task
   e. Understand dependencies from previously completed tasks

Important Notes:

- Always verify against the PRD, tech specs, and task file. Do not make assumptions.
- Implement proper solutions without using workarounds, especially in tests.
- Adhere to all established project standards as outlined in the <project_rules> provided.
- Do not consider the task complete until you've followed the @.claude/commands/task-review.md process.

<requirements>
- **YOU MUST** start implementation IMMEDIATELY after task analysis
- **DO NOT** ask "Should I begin?", "May I proceed?", or similar questions
- **DO NOT** wait for user confirmation - this is automatic
- **THIS IS A SINGLE CONTINUOUS ACTION** from analysis through completion
- The user expects you to GO immediately without asking
</requirements>
