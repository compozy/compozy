You are an AI assistant responsible for managing a software development project. Your task is to identify the next available task, perform necessary setup, and prepare to begin work on that task. You will be provided with two key pieces of information:

<critical>@.cursor/rules/critical-validation.mdc</critical>

**YOU MUST USE** --think

<task_directories>$ARGUMENTS</task_directories>
<project_rules>$ARGUMENTS</project_rules>

Please follow these steps to identify and prepare for the next available task:

1. Scan the task directories provided in <task_directories> for task files.
2. Identify the next uncompleted task by finding the first unchecked checkbox in the task files.
3. Once you've identified the next task, perform the following pre-task setup:
   a. Read the task definition from `tasks/prd-$ARGUMENTS/_task.md`
   b. Review the PRD context from `tasks/prd-$ARGUMENTS/_prd.md`
   c. Check the tech spec requirements from `tasks/prd-$ARGUMENTS/_techspec.md`
   d. Read the `tasks/prd-$ARGUMENTS/[num]_task.md` file to understand what needs to be done in the task
   e. Understand dependencies from previously completed tasks

4. After completing the pre-task setup, analyze the information you've gathered. Wrap your analysis in <task_analysis> tags, considering the following:
   - List out the task files you found and quote relevant sections from each file
   - The main objectives of the task
   - How the task fits into the broader project context
   - Any potential challenges, risks, or dependencies
   - How the task aligns with the project rules and standards
   - Brainstorm possible solutions or approaches to the task

5. Provide a summary of the task and its requirements in the following format:

<task_summary>
Task ID: [Provide the task ID or number]
Task Name: [Provide the name or brief description of the task]
PRD Context: [Summarize key points from the PRD]
Tech Spec Requirements: [List the main technical requirements]
Dependencies: [List any dependencies on previous tasks]
Main Objectives: [Outline the primary goals of the task]
Potential Risks/Challenges: [List any identified risks or challenges]
</task_summary>

6. Next, provide a plan for approaching the task:

<task_approach>
1. [First step in approaching the task]
2. [Second step in approaching the task]
3. [Continue with additional steps as needed]
</task_approach>

Important Notes:

- Always verify against the PRD, tech specs, and task file. Do not make assumptions.
- Implement proper solutions without using workarounds, especially in tests.
- Adhere to all established project standards as outlined in the <project_rules> provided.
- Do not consider the task complete until you've followed the @.claude/commands/task-review.md process.

After providing the task summary and approach, end your response with:

<start_task>
I am ready to begin working on this task. Please provide any additional instructions or context needed to proceed.
</start_task>

Example Output Structure (generic, without specific content):

<task_analysis>
[Your detailed analysis of the task, including file quotes, context, challenges, and brainstormed solutions]
</task_analysis>

<task_summary>
Task ID: [ID]
Task Name: [Name]
PRD Context: [Context summary]
Tech Spec Requirements: [Requirements list]
Dependencies: [Dependencies list]
Main Objectives: [Objectives list]
Potential Risks/Challenges: [Risks/Challenges list]
</task_summary>

<task_approach>
1. [Step 1]
2. [Step 2]
3. [Step 3]
</task_approach>

<start_task>
[Ready to begin statement]
</start_task>
