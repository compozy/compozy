# LLM Orchestrator Prompt Templates

The orchestrator now renders agent prompts from Go templates stored in this directory. Templates are parsed at startup via `prompts.TemplateFS` and selected dynamically according to provider capabilities and runtime context.

## Template Files

- `action_prompt_default.tmpl` – baseline prompt render when no structured schema is required. Adds tool call guidance when tools are available and slots for dynamic examples or failure guidance.
- `action_prompt_json_native.tmpl` – used when the provider advertises native JSON-schema support. Keeps instructions concise and relies on the provider to enforce the schema.
- `action_prompt_json_fallback.tmpl` – fallback when a schema is present but native JSON mode is unavailable. Embeds the schema inline and reiterates strict JSON output requirements.
- `system_prompt_with_builtins.tmpl` – composes agent instructions with the canonical built-in tools guide and wraps the result in the `<built-in-tools>` stanza consumed by the orchestrator.

## Dynamic Slots

The action templates receive a `TemplatePayload` containing:

- `ActionPrompt` – original prompt from the `agent.ActionConfig`.
- `HasTools` – adds `<critical>` guidance when tools are registered.
- `SchemaJSON` – stringified JSON schema for fallback rendering.
- `Examples` – slice of contextual examples (`Summary`, `Content`). Populated from the request and enriched with successful tool outputs.
- `FailureGuidance` – up to three observations derived from recent tool failures.

Whitespace is trimmed before returning the rendered prompt to avoid violating the `no-linebreaks` rule.

The system prompt template receives:

- `HasInstructions` – boolean indicating whether an agent authored system prompt should be emitted ahead of the built-in tools guide.
- `Instructions` – raw agent instructions preserved exactly as supplied.
- `BuiltinTools` – canonical tools documentation sourced from `builtin_tools_prompt.txt`.

## Adding Variants

To introduce a new variant:

1. Create a template file under `templates/` with the desired suffix (e.g., `action_prompt_providerX.tmpl`).
2. Handle selection logic in `engine/llm/prompt_builder.go` (update the variant switch block).
3. Keep instructions minimal and add whitespace chomping directives (`{{- ... -}}`) to prevent extra blank lines.
4. Ensure new dynamic data is represented in `PromptDynamicContext` and sanitized before rendering.

A matching unit test should be added to `prompt_builder_test.go` covering the new variant.
