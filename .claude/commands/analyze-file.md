Analyze current file with unstaged changes using Zen MCP and project rules.  
**Rules Applied:**
• @go-coding-standards.mdc - Function length, error handling, constructor patterns
• @architecture.mdc - Dependency direction, interface usage, SOLID principles  
• @testing-standards.mdc - Test patterns, testify/mock usage
• @api-standards.mdc - Swagger docs, response formats (if API file)
• @quality-security.mdc - Security requirements, input validation

**Process:**
• Reads current file with git diff for unstaged changes
• Uses Zen MCP codereview tool for deep analysis
• Provides specific refactoring recommendations
• Highlights rule violations with fix suggestions

**Usage:** `/analyze:file`
