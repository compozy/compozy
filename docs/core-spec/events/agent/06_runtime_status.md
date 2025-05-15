## State Event: `AgentRuntimeStatus`

**Description:** An `agent.Runtime` instance has changed its registration status (registered or deregistered).
**Produced By:** `agent.Runtime` (on startup/shutdown) or `agent.Dispatcher` (on health check changes)
**Consumed By:** `state.Manager, agent.Dispatcher, (Operational) monitoring.Systems`
**Lifecycle Stage:** Agent execution environment instance becomes available or unavailable.
**NATS Communication Pattern:** Asynchronous

### NATS Subject

`compozy.<correlation_id>.agent.events.<runtime_id>.{registered, deregistered}`

### JSON Payload Example (Registered):

```json
{
  "metadata": {
    "correlation_id": "<uuid>",
    "event_id": "<uuid>",
    "event_timestamp": "2025-05-13T10:00:00Z",
    "source_component": "agent.Runtime_worker-alpha-01",
    "created_by": "system"
  },
  "agent_runtime": {
    "runtime_id": "<uuid>",
    "hostname": "worker-alpha-01",
    "ip_address": "192.168.1.101",
    "capabilities": {
      "supported_providers": ["openai", "groq"],
      "supported_models": ["gpt-4-turbo", "llama3-70b"],
      "max_concurrent_executions": 5
    }
  },
  "payload": {
    "status": "REGISTERED",
    "result": null,
    "duration_ms": null,
    "context": {
      "reason": "Instance started and registered successfully.",
      "registration_timestamp": "2025-05-13T10:00:00Z"
    }
  }
}
```

### JSON Payload Example (Deregistered):

```json
{
  "metadata": {
    "event_id": "<uuid>",
    "event_timestamp": "2025-05-14T18:30:00Z",
    "source_component": "agent.Runtime_worker-alpha-01",
    "created_by": "system"
  },
  "agent_runtime": {
    "runtime_id": "<uuid>",
    "hostname": "worker-alpha-01"
  },
  "payload": {
    "status": "{REGISTERED, DEREGISTERED}",
    "result": null,
    "duration_ms": null,
    "context": {
      "reason": "Graceful shutdown initiated by administrator.",
      "deregistration_timestamp": "2025-05-14T18:30:00Z"
    }
  }
}
```

### Payload Properties

The `payload` object contains the following fields:
-   **`status`** (`string`, Required)
    -   Description: Indicates the registration status of the agent runtime.
    -   Values: `"REGISTERED"`, `"DEREGISTERED"`
-   **`result`** (`object`, Optional)
    -   Description: Holds the result of an operation, if applicable. Not typically used for status changes.
-   **`duration_ms`** (`integer`, Optional)
    -   Description: Duration related to the event, if applicable.
-   **`context`** (`object`, Optional)
    -   Description: Event-specific contextual data.