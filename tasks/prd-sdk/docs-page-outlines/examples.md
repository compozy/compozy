# Page Outline: SDK Examples Index
- **Purpose:** Surface runnable examples and describe how to execute them. Sources: tasks/prd-sdk/05-examples.md (all sections), _docs.md examples plan.
- **Audience:** Hands-on engineers wanting reference implementations.
- **Prerequisites:** Getting Started guide completion.
- **Key Sections:**
  1. How examples are organized (table listing 11 examples, purpose, key builders) referencing 05-examples.md summary table.
  2. Running examples (commands, prerequisites) referencing 05-examples.md §"How to Run".
  3. Customizing examples (configuration toggles, environment variables) referencing 05-examples.md per-example notes.
  4. Troubleshooting examples (link to troubleshooting page and task_55.md).
- **Cross-links:** Getting Started (for environment), Testing (for verifying sample code), CLI docs for deployment commands.
- **Examples Strategy:** Provide canonical command `go run ./sdk/examples/<name>.go`; do not inline long code; embed callouts for context-first setup.
- **Notes:** Specify that example code resides in repository `sdk/examples/` and is the maintenance source of truth.
