# Builder Outline: Project
- **Purpose:** Document `sdk/project` builder usage for project-level configuration. Source: tasks/prd-sdk/03-sdk-entities.md §"Project Configuration".
- **Audience:** Engineers structuring SDK projects.
- **Sections:**
  1. Overview & responsibilities (project metadata, resource registry).
  2. Fluent API summary table (Name, Version, Description, Add* methods) referencing 03-sdk-entities.md method list.
  3. AutoLoad & hybrid support referencing 06-migration-guide.md §"Hybrid".
  4. Validation rules & BuildError patterns referencing 03-sdk-entities.md validation subsection.
  5. Related resources (link to workflow, runtime, monitoring builders).
- **Content Sources:** 03-sdk-entities.md, 04-implementation-plan.md project bootstrap tasks.
- **Cross-links:** Core project YAML doc for parity, migration guide, architecture page (engine registration flow).
- **Examples:** Link to `sdk/examples/01_simple_workflow.go` and `08_hybrid_project.go`.
