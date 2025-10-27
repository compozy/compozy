# Builder Outline: Client SDK
- **Purpose:** Explain `sdk/client` builder and usage for invoking workflows. Source: tasks/prd-sdk/03-sdk-entities.md §"Client SDK", 05-examples.md client samples.
- **Audience:** Application developers integrating with Compozy services.
- **Sections:**
  1. Client initialization & configuration (auth, endpoints) referencing method list.
  2. Workflow execution APIs (ExecuteWorkflow, status polling) referencing 03-sdk-entities.md.
  3. Error handling patterns referencing task_55.md client errors.
  4. Testing clients referencing 07-testing-strategy.md client mocks.
  5. Integration with API docs referencing API reference endpoints.
- **Content Sources:** 03-sdk-entities.md, 05-examples.md §"Client".
- **Cross-links:** API overview, Core workflow execution doc, troubleshooting (client errors).
- **Examples:** `sdk/examples/11_client_runner.go`.
