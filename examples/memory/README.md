# Memory Example

This example demonstrates memory capabilities in Compozy through two different workflows:

1. **Agent-based memory** (`workflow.yaml`) - Conversational agent using tools for memory operations
2. **Direct memory tasks** (`memory-task-workflow.yaml`) - Direct memory operations using memory task type

## What it demonstrates

- **User memory persistence** - Store and retrieve user information across sessions
- **Multiple memory operations** - Read, write, append, delete, stats, health, clear
- **Memory task type** - Direct memory operations without tools
- **Session management** - User profile and session tracking
- **Memory statistics** - Get insights into memory usage and health

## How it works

### Workflow 1: Agent-based Memory (`workflow.yaml`)

A simple conversational agent that:

1. **First message**: User introduces themselves: "Hi, I'm John and I live in San Francisco"
2. **Agent responds**: Acknowledges and stores the information in memory
3. **Second message**: User asks: "Where do I live?"
4. **Agent responds**: Uses memory to answer: "You live in San Francisco"

### Workflow 2: Direct Memory Tasks (`memory-task-workflow.yaml`)

Demonstrates direct memory operations:

1. **Initialize**: Creates user profile with name, email, preferences
2. **Update Profile**: Appends profile updates to memory
3. **Get Stats**: Retrieves memory usage statistics and health metrics
4. **Cleanup**: Safely clears user data with backup option

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

## Testing the workflows

### Testing Agent-based Memory Workflow (`memory-demo`)

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

### Testing Direct Memory Tasks Workflow (`memory-task-demo`)

1. **Initialize user profile:**

    ```bash
    curl -X POST http://localhost:3001/api/v0/workflows/memory-task-demo/executions \
        -H "Content-Type: application/json" \
        -d '{
        "input": {
          "user_id": "alice_123",
          "session_id": "session_001",
          "action": "initialize",
          "user_data": {
            "name": "Alice",
            "email": "alice@example.com",
            "preferences": {
              "theme": "dark",
              "language": "en"
            }
          }
        }
      }'
    ```

2. **Update user profile:**

    ```bash
    curl -X POST http://localhost:3001/api/v0/workflows/memory-task-demo/executions \
        -H "Content-Type: application/json" \
        -d '{
        "input": {
          "user_id": "alice_123",
          "session_id": "session_002",
          "action": "update_profile",
          "user_data": {
            "email": "alice.new@example.com"
          }
        }
      }'
    ```

3. **Get memory statistics:**

    ```bash
    curl -X POST http://localhost:3001/api/v0/workflows/memory-task-demo/executions \
        -H "Content-Type: application/json" \
        -d '{
        "input": {
          "user_id": "alice_123",
          "session_id": "session_003",
          "action": "get_stats"
        }
      }'
    ```

4. **Cleanup user data:**

    ```bash
    curl -X POST http://localhost:3001/api/v0/workflows/memory-task-demo/executions \
        -H "Content-Type: application/json" \
        -d '{
        "input": {
          "user_id": "alice_123",
          "session_id": "session_004",
          "action": "cleanup"
        }
      }'
    ```

## Expected Flows

### Agent-based Memory Flow

1. **First interaction**: User introduces themselves with name and city
2. **Agent stores**: Information is saved to user's memory
3. **Follow-up question**: User asks where they live
4. **Agent recalls**: Uses memory to answer correctly

### Direct Memory Tasks Flow

1. **Initialize**: User profile created with personal data and preferences
2. **Update**: Profile information appended to existing memory
3. **Stats**: Memory usage statistics and health metrics retrieved
4. **Cleanup**: User data safely cleared with backup confirmation

## Key Features

### Shared Features

- **User isolation**: Each user has their own memory space
- **Template-based keys**: Dynamic memory keys using user and session IDs
- **Automatic persistence**: Data stored reliably across executions

### Agent-based Features

- **Conversational flow**: Natural back-and-forth conversation
- **Context awareness**: Agent references previous messages
- **Tool-mediated memory**: Memory operations through custom tools

### Direct Memory Task Features

- **Direct operations**: No tools required for memory operations
- **Multiple operations**: Read, write, append, stats, health, clear
- **Safety features**: Confirmation required for destructive operations
- **Health monitoring**: Built-in memory diagnostics and statistics

## Components

### Shared Components

- **Memory**: `user_memory` - Stores user data with token and message limits

### Agent-based Components

- **Agent**: `conversation_agent` - Friendly agent that remembers user info
- **Tool**: `memory_tool` - Handles read/append operations

### Direct Memory Task Components

- **Router Task**: Routes actions to appropriate memory operations
- **Memory Tasks**: Direct memory operations (read, write, append, etc.)
- **Resource Binding**: Explicit memory resource declarations

## Files

- `workflow.yaml` - Agent-based memory workflow
- `memory-task-workflow.yaml` - Direct memory task workflow
- `memory/user_memory.yaml` - Shared memory resource configuration
- `memory_tool.ts` - Tool for agent-based memory operations
- `api.http` - Example API requests for testing both workflows

These examples demonstrate two complementary approaches to memory management in Compozy:

1. **Agent-based**: For conversational applications requiring AI processing
2. **Direct tasks**: For data management operations requiring precise control
