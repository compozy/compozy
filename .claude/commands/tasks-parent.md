Read the PRD and Tech Spec for $ARGUMENTS.

Follow the updated task-generate-list.mdc workflow:

1. **Generate Merged File**: Run `mf ./tasks/prd-$ARGUMENTS` to create merged PRD + techspec
2. **Create Taskmaster Tag**: Use taskmaster MCP to create a tag for this feature
3. **Generate Tasks**: Use taskmaster parse-prd with the merged file to generate tasks.json
4. **Complexity Analysis**: Generate complexity report using taskmaster
5. **Create Tasks Summary**: Generate the \_tasks.md file based on taskmaster output
6. **Parallel Agent Analysis**: ⚠️**CRITICAL** - Spin up parallel agents for each task to:
    - Check architecture duplication risks
    - Identify missing components/integration points
    - Validate standards compliance
    - Analyze dependencies and integration challenges

**Path Variables**: Use `$ARGUMENTS` consistently for the feature slug throughout the workflow.

**Output**: Complete taskmaster workflow with tasks.json, complexity report, \_tasks.md, and parallel agent analysis results.

**Pause Point**: After parallel agent analysis, wait for user confirmation ("Go") before proceeding to implementation.
