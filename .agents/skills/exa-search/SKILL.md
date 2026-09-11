---
name: exa-search
description: "Call Exa Search directly with cURL or raw HTTP. Use when an agent needs Exa semantic web retrieval from POST /search without an SDK, including ranked results, domain or category filters, freshness-aware result content, highlights or text extraction, structured output, or streaming search responses."
---

# Exa Search

> Requires API key: Get one at https://dashboard.exa.ai/api-keys
>
> Header: `x-api-key: $EXA_API_KEY`

Use `POST https://api.exa.ai/search` for semantic web retrieval, ranked results, and optional result-level extraction in one raw HTTP call. Start with `type: "auto"` for general retrieval. Add `contents` only when the caller needs page text, highlights, summaries, freshness-controlled crawling, subpages, or extracted links.

## Quick Start (cURL)

### Basic search

```bash
curl -sS -X POST "https://api.exa.ai/search" \
  -H "Content-Type: application/json" \
  -H "x-api-key: $EXA_API_KEY" \
  -d '{
    "query": "latest developments in LLMs",
    "type": "auto",
    "numResults": 10
  }'
```

### Search with highlights

```bash
curl -sS -X POST "https://api.exa.ai/search" \
  -H "Content-Type: application/json" \
  -H "x-api-key: $EXA_API_KEY" \
  -d '{
    "query": "latest developments in LLMs",
    "type": "auto",
    "numResults": 5,
    "contents": {
      "highlights": true
    }
  }'
```

### With filters and freshness

```bash
curl -sS -X POST "https://api.exa.ai/search" \
  -H "Content-Type: application/json" \
  -H "x-api-key: $EXA_API_KEY" \
  -d '{
    "query": "AI regulation policy updates",
    "type": "auto",
    "category": "news",
    "numResults": 10,
    "includeDomains": ["reuters.com", "bbc.com"],
    "startPublishedDate": "2025-01-01",
    "contents": {
      "text": {
        "maxCharacters": 2000
      },
      "maxAgeHours": 24,
      "livecrawlTimeout": 12000
    }
  }'
```

### Deep search

```bash
curl -sS -X POST "https://api.exa.ai/search" \
  -H "Content-Type: application/json" \
  -H "x-api-key: $EXA_API_KEY" \
  -d '{
    "query": "map the major technical and commercial tradeoffs in sodium-ion batteries for grid storage",
    "type": "deep",
    "numResults": 8
  }'
```

## Endpoint

```text
POST https://api.exa.ai/search
```

Authentication: `x-api-key: <API_KEY>` header. Exa also accepts `Authorization: Bearer <API_KEY>`, but prefer `x-api-key` in cURL examples for consistency.

Use this endpoint when the agent needs search results. If the agent already has URLs and only needs extraction, use `POST /contents` instead.

## Parameters

### Core request parameters

| Parameter | Type | Required | Default | Description |
| --- | --- | --- | --- | --- |
| `query` | string | Yes | - | Natural-language search query. Long, semantically rich descriptions work well. |
| `type` | string | No | `auto` | Search method: `auto`, `fast`, `instant`, `deep-lite`, `deep`, or `deep-reasoning`. |
| `numResults` | integer | No | `10` | Number of results to return. Use small values for agent loops; maximum is 100. |
| `category` | string | No | - | Specialized result type: `company`, `people`, `research paper`, `news`, `personal site`, or `financial report`. |
| `includeDomains` | string[] | No | - | Only return results from these domains, paths, or wildcard patterns. Max 1200. |
| `excludeDomains` | string[] | No | - | Exclude these domains, paths, or wildcard patterns. Max 1200. |
| `startPublishedDate` | string | No | - | ISO 8601 lower bound for result publication date. |
| `endPublishedDate` | string | No | - | ISO 8601 upper bound for result publication date. |
| `userLocation` | string | No | - | Two-letter ISO country code such as `US` or `GB`. |
| `moderation` | boolean | No | `false` | Filter unsafe content from results. |
| `additionalQueries` | string[] | No | - | Extra query variants for deep-search variants. Use alongside the main `query`. |
| `systemPrompt` | string | No | - | Instructions for synthesized output and deep-search planning, such as source preferences. |
| `outputSchema` | object | No | - | JSON Schema controlling `output.content`. Adds synthesized output and grounding. |
| `stream` | boolean | No | `false` | If `true`, returns SSE instead of a single JSON response. |
| `compliance` | string | No | - | Enterprise-only compliance mode, such as `hipaa`, when enabled for the account. |

### Content parameters nested under `contents`

On `/search`, `text`, `highlights`, and `summary` must be nested under `contents`.

| Parameter | Type | Required | Default | Description |
| --- | --- | --- | --- | --- |
| `contents.text` | boolean or object | No | - | Return full page text as markdown. Object form supports `maxCharacters`, `includeHtmlTags`, `verbosity`, `includeSections`, and `excludeSections`. |
| `contents.highlights` | boolean or object | No | - | Return query-relevant excerpts. Prefer `true` for agent workflows unless a fixed character budget is required. |
| `contents.summary` | boolean or object | No | - | Return per-result LLM summaries. Use sparingly because each result adds synthesis work. |
| `contents.maxAgeHours` | integer | No | - | Freshness control. `0` always live crawls; `-1` uses cache only; omit for default cache-first behavior with crawl fallback. |
| `contents.livecrawlTimeout` | integer | No | `10000` | Timeout for live crawling in milliseconds. Use `10000` to `15000` for most freshness-sensitive calls. |
| `contents.subpages` | integer | No | `0` | Number of linked subpages to crawl per result. |
| `contents.subpageTarget` | string or string[] | No | - | Terms used to prioritize which subpages matter, such as `["api", "pricing"]`. |
| `contents.extras.links` | integer | No | `0` | Number of links to extract from each result page. |
| `contents.extras.imageLinks` | integer | No | `0` | Number of image URLs to extract from each result page. |

### Text object options

| Parameter | Type | Default | Description |
| --- | --- | --- | --- |
| `maxCharacters` | integer | - | Character limit for returned text. Use this instead of `tokensNum`. |
| `includeHtmlTags` | boolean | `false` | Preserve HTML tags in output. |
| `verbosity` | string | `compact` | `compact`, `standard`, or `full`. Pair fresh section-aware extraction with `contents.maxAgeHours: 0`. |
| `includeSections` | string[] | - | Only include selected sections: `header`, `navigation`, `banner`, `body`, `sidebar`, `footer`, `metadata`. |
| `excludeSections` | string[] | - | Exclude selected sections from the same section list. |

### Highlights object options

Prefer `contents.highlights: true` for the highest-quality default. Only use object form when the agent needs a custom focus or budget.

| Parameter | Type | Default | Description |
| --- | --- | --- | --- |
| `query` | string | - | Custom query guiding which excerpts are returned. |
| `maxCharacters` | integer | - | Cap highlight characters per URL. Omit unless the caller has a strict budget. |

### Summary object options

| Parameter | Type | Default | Description |
| --- | --- | --- | --- |
| `query` | string | - | Custom query for the summary. |
| `schema` | object | - | JSON Schema for structured per-result summaries. |

## Search Types

Search `type` controls the retrieval and synthesis mode. Pick the mode for the workflow, not just the output format. `outputSchema` can be used with any search type; use deeper modes when the search process itself needs more planning, synthesis, or reasoning.

| Type | Best for | Tradeoff |
| --- | --- | --- |
| `auto` | General default search and most new integrations | Balances speed and quality without requiring the caller to tune retrieval strategy. |
| `fast` | Low-latency agent loops and product paths | Faster than `auto`; use when responsiveness matters more than maximum reasoning depth. |
| `instant` | Real-time UI, chat, voice, and autocomplete-style paths | Lowest latency path; use for quick retrieval rather than deep synthesis. |
| `deep-lite` | Lightweight research or synthesis | Adds more planning and synthesis than `auto` while staying lighter than full `deep`. |
| `deep` | Multi-step research, comparisons, and synthesis-heavy retrieval | Higher latency; better when the query needs exploration across several sources. |
| `deep-reasoning` | Hard research tasks with high ambiguity or complex tradeoffs | Highest latency and reasoning depth. |

Use `auto` unless latency or reasoning depth is the primary constraint. Use `fast` or `instant` for time-sensitive calls. Use `deep`, `deep-lite`, or `deep-reasoning` when the query needs multi-step source discovery, comparison, or synthesis.

### Mode-only examples

```json
{
  "query": "recent product launches from major AI chip companies",
  "type": "fast",
  "numResults": 5
}
```

```json
{
  "query": "compare competing explanations for the recent rise in grid-scale battery deployments",
  "type": "deep",
  "numResults": 8
}
```

## Structured Output

Use `systemPrompt` for behavior and `outputSchema` for shape.

```bash
curl -sS -X POST "https://api.exa.ai/search" \
  -H "Content-Type: application/json" \
  -H "x-api-key: $EXA_API_KEY" \
  -d '{
    "query": "compare the latest frontier AI model releases",
    "type": "deep",
    "systemPrompt": "Prefer official sources and avoid duplicate results.",
    "outputSchema": {
      "type": "object",
      "properties": {
        "models": {
          "type": "array",
          "items": {
            "type": "object",
            "properties": {
              "name": { "type": "string" },
              "notable_claims": {
                "type": "array",
                "items": { "type": "string" }
              }
            },
            "required": ["name", "notable_claims"]
          }
        }
      },
      "required": ["models"]
    },
    "contents": {
      "highlights": true
    }
  }'
```

Keep schemas compact and bounded. Do not add citation fields to the schema; grounding is returned separately in `output.grounding`.

## Streaming

Streaming applies to synthesized output, so include `outputSchema` along with `-N`, `Accept: text/event-stream`, and `stream: true`. Without `outputSchema`, the endpoint returns the normal JSON search response even when `stream` is `true`.

```bash
curl -sS -N -X POST "https://api.exa.ai/search" \
  -H "Content-Type: application/json" \
  -H "Accept: text/event-stream" \
  -H "x-api-key: $EXA_API_KEY" \
  -d '{
    "query": "recent grid-scale battery deployments",
    "type": "deep",
    "stream": true,
    "outputSchema": {
      "type": "object",
      "properties": {
        "summary": { "type": "string" }
      },
      "required": ["summary"]
    },
    "contents": {
      "highlights": true
    }
  }'
```

Treat streaming as SSE rather than JSON. Each `data:` frame contains an OpenAI-compatible chat completion chunk; read partial text from `choices[0].delta.content` and handle completion or error frames defensively.

## Response Fields

| Field | Type | Description |
| --- | --- | --- |
| `requestId` | string | Unique request identifier. |
| `results` | array | Ranked result objects. |
| `results[].title` | string | Page title. |
| `results[].url` | string | Page URL. |
| `results[].publishedDate` | string or null | Estimated publication date when available. |
| `results[].author` | string or null | Author when available. |
| `results[].text` | string | Returned when `contents.text` is requested. |
| `results[].highlights` | string[] | Returned when `contents.highlights` is requested. |
| `results[].highlightScores` | number[] | Similarity scores for highlights. |
| `results[].summary` | string | Returned when `contents.summary` is requested. |
| `results[].subpages` | array | Nested result objects from subpage crawling. |
| `results[].extras.links` | string[] | Extracted links when requested. |
| `output.content` | string or object | Synthesized output when `outputSchema` is provided. |
| `output.grounding` | array | Citations and confidence labels for synthesized fields. |
| `costDollars.total` | number | Total request cost when returned. |
| `searchTime` | number | Search latency when returned. |

## Critical Pitfalls

- Keep `text`, `highlights`, and `summary` inside `contents` on `/search`.
- Do not send top-level `text`, `highlights`, or `summary`; that shape belongs to `/contents`.
- Do not send `tokensNum`; use `contents.text.maxCharacters` to cap extracted text.
- Do not use `useAutoprompt`, `numSentences`, or `highlightsPerUrl` in new requests.
- Use `contents.maxAgeHours` instead of `livecrawl`.
- Use documented categories only: `company`, `people`, `research paper`, `news`, `personal site`, and `financial report`.
- Avoid invalid category/filter combinations. `company` and `people` do not support `startPublishedDate` or `endPublishedDate`. `company` supports `excludeDomains`; `people` does not, and `people` only accepts LinkedIn domains in `includeDomains`.
- Pick one of `contents.highlights`, `contents.text`, or `contents.summary` by default. Stack modes only when the caller truly needs multiple views of each page.
- Expect SSE only when `stream: true` is paired with `outputSchema`; otherwise `/search` returns its normal JSON response.
