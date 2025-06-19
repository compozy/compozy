# Weather Agent Example

This example demonstrates a multi-agent weather advisory system that provides comprehensive recommendations for tourists based on weather conditions, showcasing agent interactions, MCP integrations, and complex task orchestration.

## What it tests

- **Agent-based workflows** with LLM-powered decision making
- **MCP (Model Context Protocol) integrations** for file operations
- **AutoLoad feature** for automatic resource discovery from folders
- **Collection tasks** processing arrays of data in parallel
- **Router tasks** with conditional branching
- **Composite tasks** grouping multiple operations
- **Agent tool usage** and external API integrations
- **Complex data transformations** and aggregations

## Workflow structure

```
weather workflow
├── weather (basic)
│   └── agent: tourist_guide → action: get_weather
├── activities (basic)
│   └── agent: tourist_guide → action: suggest_activities
├── activity_analysis (collection, parallel)
│   └── analyze-activity-{index} (basic, for each activity)
│       └── agent: tourist_guide → action: analyze_activity
├── clothing (basic)
│   └── agent: tourist_guide → action: suggest_clothing
├── clothing_validation (collection, parallel)
│   └── validate-clothing-{index} (basic, for each clothing item)
│       └── agent: tourist_guide → action: validate_clothing
├── aggr (aggregate)
│   └── combines all previous task outputs
├── clothing_check (router)
│   ├── condition: has_clothes → save_results (composite)
│   │   ├── save_json (basic)
│   │   │   └── tool: save_data (format: json)
│   │   └── save_txt (basic)
│   │       └── tool: save_data (format: txt)
│   └── condition: no_clothes → no_results (basic)
│       └── tool: save_data (format: txt)
└── verify_saved_files (basic, final)
    └── agent: file_reader (MCP) → action: list_saved_files
```

## Tools

- **weather_tool**: Fetches real weather data from Open-Meteo API
- **save_data**: Saves weather reports to JSON or text files

## Agents

- **tourist_guide**: AI agent that analyzes weather and provides recommendations
- **file_reader**: MCP-enabled agent that verifies saved files using Docker gateway

## Running

```bash
cd examples/weather
../../compozy dev
```

Make sure you have a `GROQ_API_KEY` environment variable set for the AI agent.

### Trigger via API

See the `api.http` file in this directory for example API requests you can run directly in your editor or with tools like REST Client extensions.

## Expected behavior

1. The workflow fetches current weather data for the specified city
2. An AI agent suggests appropriate tourist activities based on weather
3. Each activity is analyzed in parallel for detailed recommendations
4. Clothing suggestions are generated and validated individually
5. All data is aggregated and saved in multiple formats
6. An MCP agent verifies the saved files using Docker tools

This example validates that:

- Agents can use tools to fetch external data
- AutoLoad automatically discovers agents and tools from configured folders
- Collection tasks process arrays efficiently in parallel
- Router tasks enable conditional workflow branching
- MCP integrations work with Docker-based tools
- Complex multi-step agent workflows execute reliably
- Data flows correctly between tasks and agents
