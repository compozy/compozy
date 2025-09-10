# Pokémon Vision Example

This example demonstrates a two-step image workflow:

1. Recognize which Pokémon appears in an input image using a vision-capable agent.
2. Pass the recognized name to a Pokédex agent that returns basic info (types, abilities, summary, etc.).

It uses Compozy’s core LLM engine (OpenAI `gpt-4o-mini`) for vision and an agent action for Pokédex information, orchestrated with structured JSON outputs.

## What it shows

- Image input via workflow schema (`image_url`)
- Vision model integration via core LLM engine (no external TS tool)
- Second-stage agent that provides Pokédex basics from recognized name
- Agent/tool orchestration with JSON-mode output

## Requirements

- Set `OPENAI_API_KEY` in your environment. Network access must be enabled.

## Running

```bash
cd examples/pokemon-img
../../compozy dev
```

Then trigger the workflow via API (see `api.http`). The final output includes both recognition fields and a `pokedex` object with basic information.

## Notes

- The example configures `openai:gpt-4o-mini` as the default model in `compozy.yaml`. Image analysis is performed directly by the LLM engine using the `vision.recognize_from_image` action and the `with.image_url` parameter.
- Input is an image URL (`image_url`). The image must be publicly accessible over http/https (no auth).
