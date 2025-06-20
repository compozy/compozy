# Events API Reference

The Events API allows external systems to trigger Compozy workflows by sending events via HTTP.

## Base URLs

| Environment       | URL                             |
| ----------------- | ------------------------------- |
| Local Development | `http://localhost:8080/api/v1`  |
| Production        | `https://api.compozy.io/api/v1` |

**Security Note:** Always use HTTPS in production environments.

## Authentication

All requests require a Bearer token in the Authorization header:

```
Authorization: Bearer your-token-here
```

### Token Configuration

Tokens are configured through the `COMPOZY_API_TOKEN` environment variable. For development, the system falls back to `dev-token` if no environment variable is set.

**Production Setup:**
Set your API token in the environment:

```bash
export COMPOZY_API_TOKEN="your-secure-production-token"
```

**Development:**
The system uses `dev-token` as fallback when `COMPOZY_API_TOKEN` is not configured.

## Endpoints

### Send Event

Trigger workflows by sending an event.

**Endpoint:** `POST /events`

**Headers:**

- `Authorization: Bearer {token}` (required)
- `Content-Type: application/json` (required)

**Request Body:**

```json
{
    "name": "string",
    "payload": {}
}
```

Fields:

- `name` (string, required): Event name
- `payload` (object, optional): Event data

**Response:**

- **202 Accepted** - Event successfully received
- **400 Bad Request** - Invalid request format
- **401 Unauthorized** - Missing or invalid token
- **403 Forbidden** - Insufficient permissions
- **500 Internal Server Error** - Server error

#### Success Response (202)

```json
{
    "success": true,
    "message": "Event received and queued for processing",
    "event_id": "01234567-89ab-cdef-0123-456789abcdef"
}
```

#### Error Response (400)

```json
{
    "success": false,
    "error": {
        "code": "validation_error",
        "message": "Invalid event format",
        "details": {
            "field": "name",
            "issue": "field is required"
        }
    }
}
```

## Examples

### Basic Event

```bash
curl -X POST http://localhost:8080/api/v1/events \
    -H "Authorization: Bearer test-token" \
    -H "Content-Type: application/json" \
    -d '{
    "name": "user.created",
    "payload": {
      "userId": "user_123",
      "email": "user@example.com"
    }
  }'
```

### Complex Event with Validation

```bash
curl -X POST http://localhost:8080/api/v1/events \
    -H "Authorization: Bearer test-token" \
    -H "Content-Type: application/json" \
    -d '{
    "name": "order.created",
    "payload": {
      "orderId": "order_456",
      "customerId": "customer_789",
      "items": [
        {
          "productId": "prod_001",
          "quantity": 2,
          "price": 25.00
        }
      ],
      "totalAmount": 50.00
    }
  }'
```

### Event Without Payload

```bash
curl -X POST http://localhost:8080/api/v1/events \
    -H "Authorization: Bearer test-token" \
    -H "Content-Type: application/json" \
    -d '{
    "name": "system.maintenance"
  }'
```

## Event Names

Use descriptive, hierarchical event names:

- ✅ `user.created`, `user.updated`, `user.deleted`
- ✅ `order.placed`, `order.shipped`, `order.completed`
- ✅ `payment.processed`, `payment.failed`, `payment.refunded`
- ✅ `system.startup`, `system.shutdown`, `system.error`

Avoid generic names:

- ❌ `event`, `trigger`, `notification`, `update`

## HTTP Status Codes

| Code | Meaning               | Description                                                 |
| ---- | --------------------- | ----------------------------------------------------------- |
| 202  | Accepted              | Event successfully received and queued for processing       |
| 400  | Bad Request           | Invalid JSON, missing required fields, or malformed request |
| 401  | Unauthorized          | Missing Authorization header or invalid token format        |
| 403  | Forbidden             | Valid token but insufficient permissions for `events:write` |
| 500  | Internal Server Error | Server error, dispatcher unavailable, or system failure     |

## Rate Limits

- **Default**: No rate limits in development
- **Production**: Implement rate limiting at API gateway level
- **Recommended**: 1000 requests per minute per token

## Error Handling

### Client-Side Retry Logic

```javascript
async function sendEventWithRetry(eventName, payload, maxRetries = 3) {
    for (let attempt = 1; attempt <= maxRetries; attempt++) {
        try {
            const response = await fetch("http://localhost:8080/api/v1/events", {
                method: "POST",
                headers: {
                    Authorization: "Bearer test-token",
                    "Content-Type": "application/json",
                },
                body: JSON.stringify({ name: eventName, payload }),
            });

            if (response.ok) {
                const result = await response.json();
                return result.data.event_id;
            }

            // Don't retry for client errors (4xx)
            if (response.status >= 400 && response.status < 500) {
                const error = await response.json();
                throw new Error(`Client error: ${error.error.message}`);
            }

            // Retry for server errors (5xx)
            if (attempt === maxRetries) {
                throw new Error(`Server error after ${maxRetries} attempts`);
            }
        } catch (error) {
            if (attempt === maxRetries) {
                throw error;
            }

            // Exponential backoff
            await new Promise((resolve) => setTimeout(resolve, Math.pow(2, attempt) * 1000));
        }
    }
}
```

## Security

### Authentication

- **Bearer tokens required** for all requests
- **Project-scoped**: Tokens are tied to specific projects
- **Permission-based**: Requires `events:write` permission

### Payload Validation

- **Schema enforcement**: Events are validated against workflow schemas
- **Type checking**: Ensures payload matches expected data types
- **Required fields**: Validates all required fields are present

### Best Practices

1. **Use HTTPS** in production
2. **Rotate tokens** regularly
3. **Validate payloads** on client side before sending
4. **Log events** for audit trails
5. **Monitor failures** and set up alerts
6. **Implement timeouts** for client requests
7. **Use proper error handling** with exponential backoff

## Monitoring

### Observability

The Events API provides structured logging for:

- Event reception and validation
- Authentication and authorization
- Schema validation failures
- Dispatcher workflow triggers
- Error conditions and failures

### Temporal Integration

Monitor event-triggered workflows in Temporal UI:

1. **Navigate to** `http://localhost:8080` (Temporal Web UI)
2. **View Dispatcher** workflow for event processing
3. **Monitor Child Workflows** triggered by events
4. **Debug Issues** using workflow history and logs

### Health Checks

Check API health:

```bash
curl -X GET http://localhost:8080/health
```

## SDK Examples

### Node.js/JavaScript

```javascript
class CompozyEventsClient {
    constructor(baseUrl, token) {
        this.baseUrl = baseUrl;
        this.token = token;
    }

    async sendEvent(name, payload = {}) {
        const response = await fetch(`${this.baseUrl}/api/v1/events`, {
            method: "POST",
            headers: {
                Authorization: `Bearer ${this.token}`,
                "Content-Type": "application/json",
            },
            body: JSON.stringify({ name, payload }),
        });

        if (!response.ok) {
            const error = await response.json();
            throw new Error(`Event failed: ${error.error.message}`);
        }

        const result = await response.json();
        return result.data.event_id;
    }
}

// Usage
const client = new CompozyEventsClient("http://localhost:8080", "test-token");
const eventId = await client.sendEvent("user.signup", {
    userId: "user_123",
    email: "user@example.com",
});
```

### Python

```python
import requests
import json

class CompozyEventsClient:
    def __init__(self, base_url, token):
        self.base_url = base_url
        self.token = token
        self.headers = {
            'Authorization': f'Bearer {token}',
            'Content-Type': 'application/json'
        }

    def send_event(self, name, payload=None):
        """Send an event to trigger workflows"""
        url = f'{self.base_url}/api/v1/events'
        data = {'name': name}
        if payload:
            data['payload'] = payload

        response = requests.post(url, headers=self.headers, json=data)

        if not response.ok:
            error_data = response.json()
            raise Exception(f"Event failed: {error_data['error']['message']}")

        result = response.json()
        return result['data']['event_id']

# Usage
client = CompozyEventsClient('http://localhost:8080', 'test-token')
event_id = client.send_event('order.created', {
    'orderId': 'order_123',
    'customerId': 'customer_456'
})
```

### Go

```go
package main

import (
    "bytes"
    "encoding/json"
    "fmt"
    "net/http"
)

type EventRequest struct {
    Name    string      `json:"name"`
    Payload interface{} `json:"payload,omitempty"`
}

type EventResponse struct {
    Success bool `json:"success"`
    Data    struct {
        EventID string `json:"event_id"`
        Message string `json:"message"`
    } `json:"data"`
}

func sendEvent(baseURL, token, eventName string, payload interface{}) (string, error) {
    reqBody := EventRequest{
        Name:    eventName,
        Payload: payload,
    }

    jsonData, err := json.Marshal(reqBody)
    if err != nil {
        return "", err
    }

    req, err := http.NewRequest("POST", baseURL+"/api/v1/events", bytes.NewBuffer(jsonData))
    if err != nil {
        return "", err
    }

    req.Header.Set("Authorization", "Bearer "+token)
    req.Header.Set("Content-Type", "application/json")

    client := &http.Client{}
    resp, err := client.Do(req)
    if err != nil {
        return "", err
    }
    defer resp.Body.Close()

    var result EventResponse
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return "", err
    }

    if !result.Success {
        return "", fmt.Errorf("event failed")
    }

    return result.Data.EventID, nil
}

// Usage
eventID, err := sendEvent("http://localhost:8080", "test-token", "user.created", map[string]interface{}{
    "userId": "user_123",
    "email":  "user@example.com",
})
```

## Integration Patterns

### Webhook Integration

Integrate with external service webhooks:

```javascript
// Express.js webhook handler
app.post("/webhooks/stripe", async (req, res) => {
    const event = req.body;

    // Map Stripe events to Compozy events
    const eventMap = {
        "payment_intent.succeeded": "payment.completed",
        "customer.created": "customer.created",
    };

    const compozyEvent = eventMap[event.type];
    if (compozyEvent) {
        await sendEvent(compozyEvent, event.data.object);
    }

    res.json({ received: true });
});
```

### Message Queue Integration

Consume from message queues:

```javascript
// Redis Pub/Sub example
const redis = require("redis");
const subscriber = redis.createClient();

subscriber.on("message", async (channel, message) => {
    const data = JSON.parse(message);
    await sendEvent(`queue.${channel}`, data);
});

subscriber.subscribe("orders", "payments", "users");
```

### Database Triggers

React to database changes:

```sql
-- PostgreSQL trigger function
CREATE OR REPLACE FUNCTION notify_order_created()
RETURNS trigger AS $$
BEGIN
  PERFORM pg_notify('order_created', row_to_json(NEW)::text);
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Trigger
CREATE TRIGGER order_created_trigger
  AFTER INSERT ON orders
  FOR EACH ROW
  EXECUTE FUNCTION notify_order_created();
```

```javascript
// Listen for database notifications
const client = new Client({
    /* postgres config */
});
client.on("notification", async (msg) => {
    if (msg.channel === "order_created") {
        const orderData = JSON.parse(msg.payload);
        await sendEvent("order.created", orderData);
    }
});
```
