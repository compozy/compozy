# Weather Agent Collection Example

This example demonstrates the Collection Task feature in Compozy by processing weather information for multiple cities in parallel.

## Features Demonstrated

### Collection Task Capabilities
- **Parallel Processing**: Process multiple cities simultaneously with configurable concurrency
- **Filtering**: Filter cities by priority level (high, medium, low, or all)
- **Error Handling**: Continue processing other cities even if some fail (best_effort strategy)
- **Dynamic Configuration**: Configurable max parallel workers and filtering

### Workflow Structure
1. **Collection Task 1 (process_cities)**: Fetches weather data for each city in parallel
2. **Collection Task 2 (get_activities_and_clothing)**: For successful weather fetches, gets activity and clothing suggestions
3. **Parallel Saving**: Saves results in both JSON and CSV formats

## Input Schema

The workflow accepts an input with multiple cities:

```json
{
  "cities": [
    {
      "name": "London",
      "priority": "high"
    },
    {
      "name": "Tokyo", 
      "priority": "medium"
    }
  ],
  "max_parallel": 3,
  "filter_priority": "all"
}
```

### City Object Properties
- `name` (required): City name for weather lookup
- `priority` (optional): "high", "medium", or "low" - defaults to "medium"
- `skip_if_weather` (optional): Array of weather conditions to skip (not implemented in current version)

### Global Configuration
- `max_parallel`: Maximum cities to process simultaneously (1-10, default: 3)
- `filter_priority`: Only process cities with this priority or higher ("high", "medium", "low", "all")

## Usage

### Run with Sample Data
```bash
# From the examples/weather-agent directory
compozy run workflow.yaml --input sample_input.json
```

### Run with Custom Input
```bash
# Create your own input file
cat > my_cities.json << 'EOF'
{
  "cities": [
    {"name": "San Francisco", "priority": "high"},
    {"name": "Berlin", "priority": "medium"},
    {"name": "Mumbai", "priority": "high"}
  ],
  "max_parallel": 2,
  "filter_priority": "high"
}
EOF

compozy run workflow.yaml --input my_cities.json
```

### Filter by Priority
```bash
# Only process high priority cities
cat > high_priority.json << 'EOF'
{
  "cities": [
    {"name": "London", "priority": "high"},
    {"name": "Tokyo", "priority": "medium"},
    {"name": "New York", "priority": "high"}
  ],
  "filter_priority": "high"
}
EOF

compozy run workflow.yaml --input high_priority.json
```

## Output

The workflow generates two files:

### results.json
```json
{
  "summary": {
    "total_cities": 5,
    "processed_cities": 4,
    "failed_cities": 1,
    "mode": "parallel",
    "strategy": "best_effort",
    "timestamp": "2024-01-15T10:30:00Z"
  },
  "cities": [
    {
      "city": "London",
      "priority": "high",
      "temperature": 15,
      "humidity": 78,
      "weather": "Partly cloudy",
      "activities": ["Visit museums", "Indoor shopping"],
      "clothing": ["Light jacket", "Comfortable shoes"],
      "timestamp": "2024-01-15T10:25:30Z"
    }
  ]
}
```

### results.csv
```csv
city,temperature,weather,clothing,activities,priority
London,15,Partly cloudy,"Light jacket, Comfortable shoes","Visit museums, Indoor shopping",high
Tokyo,22,Clear sky,"T-shirt, Shorts","Walking tours, Outdoor dining",medium
```

## Collection Task Configuration

### Parallel Mode with Filtering
```yaml
- id: process_cities
  type: collection
  items: "{{ .workflow.input.cities }}"
  filter: '{{ eq .workflow.input.filter_priority "all" or ... }}'
  mode: parallel
  max_workers: "{{ .workflow.input.max_parallel | default 3 }}"
  strategy: best_effort
  continue_on_error: true
  item_var: city
  index_var: city_index
  timeout: 120s
```

### Key Features
- **Items Expression**: `"{{ .workflow.input.cities }}"` - Iterates over input cities array
- **Dynamic Filtering**: Complex filter expression based on priority levels
- **Parallel Processing**: `mode: parallel` with configurable `max_workers`
- **Error Resilience**: `strategy: best_effort` and `continue_on_error: true`
- **Custom Variables**: `item_var: city` makes each city available as `{{ .city }}`
- **Timeout Protection**: 120-second timeout per city

## Environment Setup

Make sure you have your API keys configured:

```bash
export GROQ_API_KEY="your-groq-api-key"
# or use Ollama locally (no key needed)
```

## What to Expect

1. **Parallel Execution**: You'll see multiple cities being processed simultaneously
2. **Error Handling**: If one city fails (e.g., invalid name), others continue processing
3. **Priority Filtering**: Only cities matching the priority filter are processed
4. **Rich Output**: Detailed results with weather, activities, and clothing suggestions
5. **Collection Statistics**: Summary showing total/processed/failed counts

This example showcases how Collection Tasks can efficiently handle batch processing with filtering, error resilience, and parallel execution - perfect for real-world scenarios where you need to process multiple items with varying success rates.