# Exa Search Examples

## Web Search Examples

### Latest News & Current Events

```bash
mcporter call 'exa.web_search_exa(query: "latest AI breakthroughs 2026", numResults: 5)'
mcporter call 'exa.web_search_exa(query: "quantum computing news", type: "fast")'
```

### Research Topics

```bash
mcporter call 'exa.web_search_exa(query: "how does RAG work in LLMs", type: "deep", numResults: 8)'
mcporter call 'exa.web_search_exa(query: "best practices for API design", numResults: 5)'
```

### Product Information

```bash
mcporter call 'exa.web_search_exa(query: "M4 Mac Mini specifications and reviews")'
mcporter call 'exa.web_search_exa(query: "comparison of vector databases", type: "deep")'
```

## Code Context Search Examples

### Programming Language Basics

```bash
mcporter call 'exa.get_code_context_exa(query: "Python asyncio basics and examples", tokensNum: 3000)'
mcporter call 'exa.get_code_context_exa(query: "Rust ownership and borrowing tutorial")'
```

### Framework & Library Usage

```bash
mcporter call 'exa.get_code_context_exa(query: "React useState and useEffect hooks examples", tokensNum: 2000)'
mcporter call 'exa.get_code_context_exa(query: "Next.js 14 app router authentication middleware")'
mcporter call 'exa.get_code_context_exa(query: "Express.js error handling best practices", tokensNum: 4000)'
```

### Specific API & SDK Documentation

```bash
mcporter call 'exa.get_code_context_exa(query: "Stripe checkout session implementation", tokensNum: 5000)'
mcporter call 'exa.get_code_context_exa(query: "AWS S3 SDK upload examples Python")'
mcporter call 'exa.get_code_context_exa(query: "Discord.js bot slash commands")'
```

### Debugging & Solutions

```bash
mcporter call 'exa.get_code_context_exa(query: "fixing CORS errors in Node.js Express")'
mcporter call 'exa.get_code_context_exa(query: "pandas dataframe memory optimization techniques", tokensNum: 4000)'
```

## Company Research Examples

### Startups & Tech Companies

```bash
mcporter call 'exa.company_research_exa(companyName: "Anthropic", numResults: 3)'
mcporter call 'exa.company_research_exa(companyName: "Perplexity AI")'
mcporter call 'exa.company_research_exa(companyName: "Scale AI", numResults: 5)'
```

### Public Companies

```bash
mcporter call 'exa.company_research_exa(companyName: "Microsoft")'
mcporter call 'exa.company_research_exa(companyName: "NVIDIA", numResults: 5)'
```

### Research Queries

```bash
# Find funding info
mcporter call 'exa.company_research_exa(companyName: "OpenAI", numResults: 5)'

# Recent news
mcporter call 'exa.company_research_exa(companyName: "Tesla", numResults: 3)'
```

## Parameter Guidance

### `type` parameter (web_search_exa)

- `"auto"` - Balanced search (default)
- `"fast"` - Quick results, less comprehensive
- `"deep"` - Thorough research, slower but more complete

### `tokensNum` parameter (get_code_context_exa)

- `1000-2000` - Focused queries, specific examples
- `3000-5000` - Standard documentation lookup (default: 5000)
- `5000-10000` - Comprehensive guides and tutorials
- `10000-50000` - Deep dives, full API documentation

### `numResults` parameter

- `3-5` - Quick lookup, specific answer
- `5-8` - Standard research (default for web: 8, company: 5)
- `10+` - Comprehensive research, multiple perspectives

## Advanced Tools Examples (Off by Default)

### Deep Search

```bash
# Comprehensive research
mcporter call 'exa-full.deep_search_exa(query: "AI safety alignment research comprehensive overview")'

# Multi-perspective exploration
mcporter call 'exa-full.deep_search_exa(query: "climate change solutions technology innovation")'
```

### Advanced Web Search

```bash
# Search with domain filters
mcporter call 'exa-full.web_search_advanced_exa(query: "machine learning tutorials", includeDomains: ["github.com", "arxiv.org"])'

# Search with date range
mcporter call 'exa-full.web_search_advanced_exa(query: "AI developments", startPublishedDate: "2026-01-01")'
```

### Crawling

```bash
# Extract content from specific URL
mcporter call 'exa-full.crawling_exa(url: "https://anthropic.com/news/claude-3-5-sonnet")'

# Get clean text from article
mcporter call 'exa-full.crawling_exa(url: "https://example.com/article")'
```

### People Search

```bash
# Find professional profiles
mcporter call 'exa-full.people_search_exa(query: "Yann LeCun AI researcher")'

# Research individuals
mcporter call 'exa-full.people_search_exa(query: "Demis Hassabis DeepMind")'
```

### Deep Researcher

```bash
# Start a research task
mcporter call 'exa-full.deep_researcher_start(topic: "quantum computing applications in cryptography", depth: "comprehensive")'

# Check research status (use taskId from start response)
mcporter call 'exa-full.deep_researcher_check(taskId: "abc123")'
```
