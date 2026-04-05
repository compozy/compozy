---
name: exa-web-search-free
description: Free AI search via Exa MCP. Web search for news/info, code search for docs/examples from GitHub/StackOverflow, company research for business intel. No API key needed.
metadata: { "clawdbot": { "emoji": "🔍", "requires": { "bins": ["mcporter"] } } }
---

# Exa Web Search (Free)

Neural search for web, code, and company research. No API key required.

## Setup

Verify mcporter is configured:

```bash
mcporter list exa
```

If not listed:

```bash
mcporter config add exa https://mcp.exa.ai/mcp
```

## Core Tools

### web_search_exa

Search web for current info, news, or facts.

```bash
mcporter call 'exa.web_search_exa(query: "latest AI news 2026", numResults: 5)'
```

**Parameters:**

- `query` - Search query
- `numResults` (optional, default: 8)
- `type` (optional) - `"auto"`, `"fast"`, or `"deep"`

### get_code_context_exa

Find code examples and docs from GitHub, Stack Overflow.

```bash
mcporter call 'exa.get_code_context_exa(query: "React hooks examples", tokensNum: 3000)'
```

**Parameters:**

- `query` - Code/API search query
- `tokensNum` (optional, default: 5000) - Range: 1000-50000

### company_research_exa

Research companies for business info and news.

```bash
mcporter call 'exa.company_research_exa(companyName: "Anthropic", numResults: 3)'
```

**Parameters:**

- `companyName` - Company name
- `numResults` (optional, default: 5)

## Advanced Tools (Optional)

Six additional tools available by updating config URL:

- `web_search_advanced_exa` - Domain/date filters
- `deep_search_exa` - Query expansion
- `crawling_exa` - Full page extraction
- `people_search_exa` - Professional profiles
- `deep_researcher_start/check` - AI research agent

**Enable all tools:**

```bash
mcporter config add exa-full "https://mcp.exa.ai/mcp?tools=web_search_exa,web_search_advanced_exa,get_code_context_exa,deep_search_exa,crawling_exa,company_research_exa,people_search_exa,deep_researcher_start,deep_researcher_check"

# Then use:
mcporter call 'exa-full.deep_search_exa(query: "AI safety research")'
```

## Tips

- Web: Use `type: "fast"` for quick lookup, `"deep"` for thorough research
- Code: Lower `tokensNum` (1000-2000) for focused, higher (5000+) for comprehensive
- See [examples.md](references/examples.md) for more patterns

## Resources

- [GitHub](https://github.com/exa-labs/exa-mcp-server)
- [npm](https://www.npmjs.com/package/exa-mcp-server)
- [Docs](https://exa.ai/docs)
