Systematic debugging requirements for recovery:

- Read the failure evidence before changing code.
- Reproduce or explain the failing invariant from logs, exit codes, and scoped files.
- Identify the root cause before proposing a fix.
- Make the smallest production-code change that addresses the root cause.
- If the failure is outside this project or cannot be fixed safely in this workspace, return a reject verdict.
