Analyze code against all `.cursor/rules/` standards and report violations.  
Checks:
• Go coding standards (function length, error handling, constructor patterns)
• Architecture compliance (dependency direction, interface usage)
• Testing patterns (`t.Run("Should...")`, testify/mock usage)
• API standards (Swagger docs, response formats)
• Security requirements (no secrets in logs, input validation)

**Usage:** `/rules:check [file-path]`
