Analyze specific folder/directory using Zen MCP and project rules.  
**Rules Applied:**
• @project-structure.mdc - Package organization, domain boundaries
• @go-coding-standards.mdc - Code consistency across files
• @architecture.mdc - Layer separation, interface design
• @testing-standards.mdc - Test organization, coverage patterns
• @section_comments.mdc - Code organization standards

**Process:**
• Reads all Go files in specified directory recursively
• Uses Zen MCP analyze tool for architectural review
• Checks package cohesion and coupling
• Identifies refactoring opportunities (extract, rename, reorganize)
• Suggests architectural improvements

**Usage:** `/analyze:folder $ARGUMENTS`  
**Example:** `/analyze:folder engine/infra/monitoring`

Analyze the folder: $ARGUMENTS
