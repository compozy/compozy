---
name: nats
description: |
  NATS cloud-native messaging system. Covers Core NATS, JetStream persistence,
  and request/reply patterns. Use for lightweight, high-performance
  microservices communication.

  USE WHEN: user mentions "nats", "jetstream", "cloud-native messaging", "request/reply", "subject wildcards", asks about "lightweight messaging", "microservices communication", "nats streaming"

  DO NOT USE FOR: event sourcing - use `kafka` or `pulsar`; complex routing - use `rabbitmq`; AWS-native - use `sqs`; Azure-native - use `azure-service-bus`; JMS compliance - use `activemq`; persistent queues only - use dedicated broker
allowed-tools: Read, Grep, Glob, Write, Edit
---
# NATS Core Knowledge

> **Full Reference**: See [advanced.md](advanced.md) for JetStream patterns (Node.js, Java), security configuration, TLS setup, and clustering.

> **Deep Knowledge**: Use `mcp__documentation__fetch_docs` with technology: `nats` for comprehensive documentation.

## Quick Start (Docker)

```yaml
# docker-compose.yml
services:
  nats:
    image: nats:latest
    ports:
      - "4222:4222"   # Client
      - "8222:8222"   # Monitoring
    command: "-js -m 8222"  # Enable JetStream
```

## Core Concepts

| Feature | Core NATS | JetStream |
|---------|-----------|-----------|
| Persistence | No | Yes |
| Delivery | At-most-once | At-least-once |
| Replay | No | Yes |
| Consumer Groups | Queue Groups | Consumer Groups |

## Subject Patterns

```
orders.created     → Specific subject
orders.*           → orders.created, orders.updated (single wildcard)
orders.>           → orders.created, orders.us.east (multi wildcard)
```

## Core NATS (Node.js)

```typescript
import { connect, StringCodec } from 'nats';

const nc = await connect({ servers: 'localhost:4222' });
const sc = StringCodec();

// Simple publish
nc.publish('orders.created', sc.encode(JSON.stringify(order)));

// Subscribe
const sub = nc.subscribe('orders.*');
for await (const msg of sub) {
  const order = JSON.parse(sc.decode(msg.data));
  console.log(`Received on ${msg.subject}:`, order);
}

// Queue group (load balancing)
const qsub = nc.subscribe('orders.process', { queue: 'workers' });

// Request/Reply
const response = await nc.request('orders.validate', sc.encode(JSON.stringify(order)), {
  timeout: 5000,
});
const result = JSON.parse(sc.decode(response.data));

await nc.drain();
```

## Core NATS (Python)

```python
import nats
import json
import asyncio

async def main():
    nc = await nats.connect("nats://localhost:4222")

    # Publish
    await nc.publish("orders.created", json.dumps(order).encode())

    # Subscribe
    async def message_handler(msg):
        order = json.loads(msg.data.decode())
        print(f"Received: {order}")

    await nc.subscribe("orders.*", cb=message_handler)

    # Request/Reply
    response = await nc.request("orders.validate",
                                json.dumps(order).encode(),
                                timeout=5)
    result = json.loads(response.data.decode())

    await nc.drain()

asyncio.run(main())
```

## Core NATS (Go)

```go
nc, _ := nats.Connect(nats.DefaultURL)
defer nc.Drain()

// Publish
body, _ := json.Marshal(order)
nc.Publish("orders.created", body)

// Subscribe
nc.Subscribe("orders.*", func(msg *nats.Msg) {
    var order Order
    json.Unmarshal(msg.Data, &order)
    processOrder(order)
})

// Queue group
nc.QueueSubscribe("orders.process", "workers", func(msg *nats.Msg) {
    // Load balanced
})

// Request/Reply
response, _ := nc.Request("orders.validate", body, 5*time.Second)
```

## When NOT to Use This Skill

- Long-term event replay required - Kafka provides better retention
- Complex routing patterns - RabbitMQ exchanges are more flexible
- JMS compliance needed - Use ActiveMQ
- AWS-native integration - SQS integrates better
- Enterprise support required - Consider commercial alternatives

## Anti-Patterns

| Anti-Pattern | Why It's Bad | Solution |
|--------------|--------------|----------|
| Core NATS for critical data | Fire-and-forget | Use JetStream for persistence |
| No subject hierarchy | Hard to filter | Use dot-separated subjects |
| No timeout on requests | Hanging requests | Always set timeout |
| Large payloads | Network strain | Keep messages small |
| Single server in production | No HA | Deploy 3+ server cluster |

## Quick Troubleshooting

| Issue | Likely Cause | Fix |
|-------|--------------|-----|
| Messages not received | Wrong subject | Verify subject name, check wildcards |
| Request timeout | No responders | Verify responders exist |
| JetStream errors | Not enabled | Start server with `-js` flag |
| Consumer lag growing | Slow processing | Add consumers or optimize |
| ACK timeout | Processing too slow | Increase ack_wait |

## Production Checklist

- [ ] TLS enabled
- [ ] Authentication configured
- [ ] Authorization (permissions)
- [ ] JetStream replicas >= 3
- [ ] Stream limits configured
- [ ] Consumer max_deliver set
- [ ] Ack timeout configured
- [ ] Monitoring dashboards
- [ ] Cluster health checks

## Reference Documentation

Available topics: `basics`, `jetstream`, `patterns`, `production`
