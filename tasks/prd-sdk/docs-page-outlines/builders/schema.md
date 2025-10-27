# Builder Outline: Schema
- **Purpose:** Document `sdk/schema` builders (Schema, Property) for validation. Source: tasks/prd-sdk/03-sdk-entities.md §"Schema Validation".
- **Audience:** Engineers defining structured inputs/outputs.
- **Sections:**
  1. Schema builder overview (types, constraints, enums) referencing method list.
  2. Property builder usage (nested structures) referencing 03-sdk-entities.md examples.
  3. Validation pipeline (compile-time vs runtime) referencing 02-architecture.md schema enforcement.
  4. Integration with tool & workflow tasks referencing 04-implementation-plan.md.
  5. Testing schemas referencing 07-testing-strategy.md.
- **Content Sources:** 03-sdk-entities.md, 02-architecture.md, 07-testing-strategy.md.
- **Cross-links:** Core schema definition doc, tool builder page, troubleshooting for validation errors.
- **Examples:** `sdk/examples/03_tool_call.go`, schema snippets in example matrix.
