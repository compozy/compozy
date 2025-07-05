# Memory Task API Endpoints

## Execute Memory Task

### Request

```http
POST /workflows/{workflowId}/tasks/{taskId}/execute
Content-Type: application/json

{
  "type": "memory",
  "operation": "write",
  "memory_ref": "conversation_memory",
  "key_template": "conv:{{.user_id}}:{{.session_id}}",
  "payload": {
    "role": "user",
    "content": "Hello"
  },
  "input": {
    "user_id": "user123",
    "session_id": "sess456"
  }
}
```

### Response

```json
{
  "status": "completed",
  "output": {
    "success": true,
    "key": "conv:user123:sess456",
    "operation": "write"
  }
}
```

## Memory Operations

### Write Operation

Stores a new message in memory.

**Request:**
```json
{
  "type": "memory",
  "operation": "write",
  "memory_ref": "conversation_memory",
  "key_template": "conv:{{.user_id}}:{{.session_id}}",
  "payload": {
    "role": "user",
    "content": "Hello AI",
    "timestamp": "2024-01-01T00:00:00Z"
  }
}
```

**Response:**
```json
{
  "status": "completed",
  "output": {
    "success": true,
    "key": "conv:user123:sess456",
    "operation": "write",
    "message_count": 1
  }
}
```

### Read Operation

Retrieves messages from memory.

**Request:**
```json
{
  "type": "memory",
  "operation": "read",
  "memory_ref": "conversation_memory",
  "key_template": "conv:{{.user_id}}:{{.session_id}}",
  "read_config": {
    "limit": 10,
    "offset": 0
  }
}
```

**Response:**
```json
{
  "status": "completed",
  "output": {
    "success": true,
    "key": "conv:user123:sess456",
    "operation": "read",
    "messages": [
      {
        "role": "user",
        "content": "Hello AI",
        "timestamp": "2024-01-01T00:00:00Z"
      }
    ],
    "total_count": 1,
    "token_count": 15
  }
}
```

### Append Operation

Adds messages to existing memory.

**Request:**
```json
{
  "type": "memory",
  "operation": "append",
  "memory_ref": "conversation_memory",
  "key_template": "conv:{{.user_id}}:{{.session_id}}",
  "payload": [
    {
      "role": "assistant",
      "content": "Hello! How can I help you?",
      "timestamp": "2024-01-01T00:01:00Z"
    }
  ]
}
```

**Response:**
```json
{
  "status": "completed",
  "output": {
    "success": true,
    "key": "conv:user123:sess456",
    "operation": "append",
    "messages_added": 1,
    "total_messages": 2
  }
}
```

### Delete Operation

Removes specific memory.

**Request:**
```json
{
  "type": "memory",
  "operation": "delete",
  "memory_ref": "conversation_memory",
  "key_template": "conv:{{.user_id}}:{{.session_id}}"
}
```

**Response:**
```json
{
  "status": "completed",
  "output": {
    "success": true,
    "key": "conv:user123:sess456",
    "operation": "delete",
    "deleted": true
  }
}
```

### Flush Operation

Archives old messages when limits are reached.

**Request:**
```json
{
  "type": "memory",
  "operation": "flush",
  "memory_ref": "conversation_memory",
  "key_template": "conv:{{.user_id}}:{{.session_id}}",
  "flush_config": {
    "dry_run": false,
    "force": false
  }
}
```

**Response:**
```json
{
  "status": "completed",
  "output": {
    "success": true,
    "key": "conv:user123:sess456",
    "operation": "flush",
    "messages_flushed": 5,
    "remaining_messages": 10,
    "summary_created": true
  }
}
```

### Health Operation

Checks memory system health.

**Request:**
```json
{
  "type": "memory",
  "operation": "health",
  "memory_ref": "conversation_memory",
  "health_config": {
    "include_stats": true
  }
}
```

**Response:**
```json
{
  "status": "completed",
  "output": {
    "success": true,
    "operation": "health",
    "healthy": true,
    "redis_status": "connected",
    "memory_stats": {
      "total_keys": 42,
      "total_messages": 156,
      "average_tokens_per_key": 235
    }
  }
}
```

### Clear Operation

Removes all messages from memory.

**Request:**
```json
{
  "type": "memory",
  "operation": "clear",
  "memory_ref": "conversation_memory",
  "key_template": "conv:{{.user_id}}:{{.session_id}}",
  "clear_config": {
    "confirm": true,
    "backup": true
  }
}
```

**Response:**
```json
{
  "status": "completed",
  "output": {
    "success": true,
    "key": "conv:user123:sess456",
    "operation": "clear",
    "messages_cleared": 15,
    "backup_created": true,
    "backup_key": "backup:conv:user123:sess456:20240101"
  }
}
```

### Stats Operation

Gets memory usage statistics.

**Request:**
```json
{
  "type": "memory",
  "operation": "stats",
  "memory_ref": "conversation_memory",
  "key_template": "conv:{{.user_id}}:{{.session_id}}",
  "stats_config": {
    "include_content": false
  }
}
```

**Response:**
```json
{
  "status": "completed",
  "output": {
    "success": true,
    "key": "conv:user123:sess456",
    "operation": "stats",
    "message_count": 15,
    "token_count": 450,
    "token_limit": 4000,
    "utilization": 0.1125,
    "last_updated": "2024-01-01T00:15:00Z",
    "ttl_remaining": "23h45m"
  }
}
```

## Error Responses

### Memory Resource Not Found

```json
{
  "status": "failed",
  "error": {
    "code": "MEMORY_RESOURCE_NOT_FOUND",
    "message": "Memory resource 'invalid_memory' not found",
    "details": {
      "memory_ref": "invalid_memory",
      "available_resources": ["conversation_memory", "session_memory"]
    }
  }
}
```

### Invalid Operation

```json
{
  "status": "failed",
  "error": {
    "code": "INVALID_MEMORY_OPERATION",
    "message": "Unsupported memory operation: invalid_op",
    "details": {
      "operation": "invalid_op",
      "supported_operations": ["read", "write", "append", "delete", "flush", "health", "clear", "stats"]
    }
  }
}
```

### Redis Connection Error

```json
{
  "status": "failed",
  "error": {
    "code": "MEMORY_PERSISTENCE_ERROR",
    "message": "Failed to connect to Redis",
    "details": {
      "redis_error": "connection timeout",
      "retry_after": 5
    }
  }
}
```

### Memory Limit Exceeded

```json
{
  "status": "failed",
  "error": {
    "code": "MEMORY_LIMIT_EXCEEDED",
    "message": "Token limit exceeded for memory key",
    "details": {
      "current_tokens": 4200,
      "max_tokens": 4000,
      "suggested_action": "flush"
    }
  }
}
```