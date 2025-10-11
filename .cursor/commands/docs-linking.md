# Documentation Improvement Prompt

## Objective

Transform documentation pages from basic reference material into comprehensive, interconnected
guides that help users understand concepts in context and navigate efficiently to related
information.

## Characteristics of a Well-Documented Page

### 1. Contextual Introduction

- Start with a clear explanation of what the feature/concept is and why it matters
- Include links to beginner-friendly tutorials for newcomers (Quick Start, First Tutorial)
- Mention where this fits in the broader system architecture
- Link to execution/usage documentation (CLI commands, APIs) when applicable

### 2. Strategic Cross-Linking

Instead of generic filler text, every paragraph should include purposeful links that:

- Connect to Prerequisites: Link to concepts users need to understand first
- Point to Related Features: Show how this feature interacts with others
- Provide Learning Paths: Guide beginners to tutorials and advanced users to references
- Enable Problem Solving: Link to debugging, troubleshooting, and validation docs
- Ensure Security: Link to security best practices when handling sensitive data

### 3. Code Examples with Context

- Every code example should be followed by explanations that link to:
  - Where these files are stored (project structure)
  - How to execute/test them (CLI/API docs)
  - What each syntax element means (template system, directives)
  - Common patterns and anti-patterns

### 4. Hierarchical Information Architecture

- Main sections for major concepts
- Subsections with specific, descriptive titles
- Callout boxes for important tips that link to expanded information
- Reference cards at the bottom organizing related documentation

### 5. Multiple Learning Approaches

Cater to different user needs:

- Conceptual: Link to overview and architecture docs
- Practical: Link to tutorials and examples
- Reference: Link to schema documentation and API specs
- Operational: Link to execution, monitoring, and debugging docs

### 6. Avoid Redundancy Through Smart Linking

- Don't duplicate information that exists elsewhere
- Link to schema documentation instead of reproducing property tables
- Reference existing explanations rather than rewriting them
- Use phrases like "Learn more about [concept]" with appropriate links

## Specific Improvements to Make

### 1. In Overview Sections:

- Add links to getting started guides for beginners
- Link to related execution documentation (CLI/API)
- Connect to the broader system context

### 2. For Configuration Sections:

- Link to configuration precedence and project setup
- Connect to runtime environments where configs are used
- Reference security practices for sensitive configuration

### 3. For Feature Explanations:

- Link to the underlying technology (e.g., "uses JSON Schema" → link to JSON Schema)
- Connect to related features that work together
- Point to troubleshooting and debugging resources

### 4. For Code Examples:

- Link template syntax to template documentation
- Connect to validation and debugging guides
- Reference where example patterns are used in tutorials

### 5. At Section Ends:

- Add reference cards for related documentation
- Organize cards by user journey (beginner → advanced)
- Use descriptive text that explains what users will find

## Example Transformation

Before:
"Triggers enable workflows to respond to external events, making them reactive and event-driven."

After:
"Triggers enable workflows to automatically start when specific events occur in your system. The
most common trigger type is signal, which creates a publish-subscribe pattern where workflows
react to named events. When a signal is sent (either through the /docs/core/signals/event-api or
from a /docs/core/signals/signal-tasks), any workflow listening for that signal name will
automatically start with the signal payload as input."

## Key Principles

1. Every link should answer a user's potential question
2. Cross-references should form learning paths, not mazes
3. Prioritize linking to practical, actionable documentation
4. Group related links in reference card sections
5. Use callout boxes to highlight important related documentation
6. Maintain a balance between detail and discoverability

## Anti-Patterns to Avoid

- Adding text just to meet line count requirements
- Linking to tangentially related content
- Duplicating information available elsewhere
- Creating circular reference loops
- Over-linking within the same sentence
- Using vague link text like "click here" or "see this"

This approach transforms documentation from isolated pages into an interconnected knowledge base
that guides users naturally through their learning journey while providing quick access to
reference material when needed.
