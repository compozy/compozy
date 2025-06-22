# Memory Example

This example demonstrates a simple conversational agent that can remember user information using Compozy's memory system.

## What it demonstrates

- **User memory persistence** - Agent remembers who you are and where you live
- **Simple conversation flow** - Natural back-and-forth conversation
- **Memory-based responses** - Agent uses stored information to answer questions

## How it works

The workflow creates a simple conversational agent that:

1. **First message**: User introduces themselves: "Hi, I'm John and I live in San Francisco"
2. **Agent responds**: Acknowledges and stores the information in memory
3. **Second message**: User asks: "Where do I live?"
4. **Agent responds**: Uses memory to answer: "You live in San Francisco"

## Memory Configuration

The example includes one memory resource:

- **`user_memory`**: Stores user conversation history and personal information (max 2000 tokens)

## Running the example

```bash
# Start the development server
make dev EXAMPLE=memory
```

Make sure you have these environment variables set:

- `GROQ_API_KEY` for the AI agent
- Redis should be running (default: localhost:6379)

## Testing the workflow

1. **User introduces themselves:**

    ```bash
    curl -X POST http://localhost:3001/api/v0/workflows/memory-demo/executions \
        -H "Content-Type: application/json" \
        -d '{
        "input": {
          "user_id": "john_doe",
          "message": "Hi, I'\''m John and I live in San Francisco"
        }
      }'
    ```

2. **Ask where you live (agent should remember):**

    ```bash
    curl -X POST http://localhost:3001/api/v0/workflows/memory-demo/executions \
        -H "Content-Type: application/json" \
        -d '{
        "input": {
          "user_id": "john_doe",
          "message": "Where do I live?"
        }
      }'
    ```

3. **Test with different user:**

    ```bash
    curl -X POST http://localhost:3001/api/v0/workflows/memory-demo/executions \
        -H "Content-Type: application/json" \
        -d '{
        "input": {
          "user_id": "jane_smith",
          "message": "Hello, my name is Jane and I'\''m from New York"
        }
      }'
    ```

## Expected Flow

1. **First interaction**: User introduces themselves with name and city
2. **Agent stores**: Information is saved to user's memory
3. **Follow-up question**: User asks where they live
4. **Agent recalls**: Uses memory to answer correctly

## Key Features

- **Simple memory pattern**: Uses `user:{{.user_id}}` key for each user
- **Automatic persistence**: Conversations are stored after each interaction
- **Context awareness**: Agent can reference previous messages
- **User isolation**: Each user has their own memory space

## Components

- **Agent**: `conversation_agent` - Friendly agent that remembers user info
- **Tool**: `memory_tool` - Handles read/append operations
- **Memory**: `user_memory` - Stores conversation history per user

## Files

- `workflow.yaml` - Main workflow definition
- `memory/user_memory.yaml` - Memory resource configuration
- `memory_tool.ts` - Tool for memory operations
- `api.http` - Example API requests for testing

This simple example shows how Compozy's memory system enables agents to maintain context across conversations.
