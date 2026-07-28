# NATS - Advanced Patterns

## JetStream Node.js

```typescript
import { connect, AckPolicy, DeliverPolicy } from 'nats';

const nc = await connect({ servers: 'localhost:4222' });
const js = nc.jetstream();
const jsm = await nc.jetstreamManager();

// Create stream
await jsm.streams.add({
  name: 'ORDERS',
  subjects: ['orders.*'],
  storage: 'file',
  retention: 'limits',
  max_msgs: 1000000,
  max_age: 7 * 24 * 60 * 60 * 1000000000, // 7 days in nanoseconds
  replicas: 3,
});

// Publish to stream
const pa = await js.publish('orders.created', sc.encode(JSON.stringify(order)), {
  msgID: order.id, // Deduplication
});
console.log(`Published: seq=${pa.seq}`);

// Create durable consumer
await jsm.consumers.add('ORDERS', {
  durable_name: 'order-processor',
  ack_policy: AckPolicy.Explicit,
  deliver_policy: DeliverPolicy.All,
  max_deliver: 3,
  ack_wait: 30000000000, // 30 seconds in nanoseconds
});

// Pull consumer
const consumer = await js.consumers.get('ORDERS', 'order-processor');

(async () => {
  const messages = await consumer.consume({ max_messages: 10 });

  for await (const msg of messages) {
    try {
      const order = JSON.parse(sc.decode(msg.data));
      await processOrder(order);
      msg.ack();
    } catch (error) {
      if (msg.info.redeliveryCount >= 3) {
        msg.term(); // Terminal failure
      } else {
        msg.nak(); // Retry
      }
    }
  }
})();

// Push consumer with callback
const psub = await js.subscribe('orders.*', {
  stream: 'ORDERS',
  config: {
    durable_name: 'push-processor',
    deliver_subject: 'orders.deliver',
    ack_policy: AckPolicy.Explicit,
  },
  callback: (err, msg) => {
    if (err) {
      console.error(err);
      return;
    }
    processOrder(JSON.parse(sc.decode(msg.data)));
    msg.ack();
  },
});
```

## JetStream Java

```java
@Service
public class JetStreamService {
    @Autowired
    private Connection nats;

    private JetStreamManagement jsm;
    private JetStream js;

    @PostConstruct
    public void init() throws Exception {
        jsm = nats.jetStreamManagement();
        js = nats.jetStream();

        // Create stream
        StreamConfiguration streamConfig = StreamConfiguration.builder()
            .name("ORDERS")
            .subjects("orders.*")
            .storageType(StorageType.File)
            .retentionPolicy(RetentionPolicy.Limits)
            .maxMessages(1000000)
            .maxAge(Duration.ofDays(7))
            .replicas(3)
            .build();

        jsm.addStream(streamConfig);

        // Create consumer
        ConsumerConfiguration consumerConfig = ConsumerConfiguration.builder()
            .durable("order-processor")
            .ackPolicy(AckPolicy.Explicit)
            .deliverPolicy(DeliverPolicy.All)
            .maxDeliver(3)
            .ackWait(Duration.ofSeconds(30))
            .build();

        jsm.addOrUpdateConsumer("ORDERS", consumerConfig);
    }

    public void publish(Order order) throws Exception {
        PublishOptions options = PublishOptions.builder()
            .messageId(order.getId())
            .build();

        PublishAck ack = js.publish("orders.created",
            objectMapper.writeValueAsBytes(order), options);
        log.info("Published: seq={}", ack.getSeqno());
    }

    public void consume() throws Exception {
        PullSubscribeOptions options = PullSubscribeOptions.builder()
            .durable("order-processor")
            .build();

        JetStreamSubscription sub = js.subscribe("orders.*", options);

        while (true) {
            List<Message> messages = sub.fetch(10, Duration.ofSeconds(5));

            for (Message msg : messages) {
                try {
                    Order order = objectMapper.readValue(msg.getData(), Order.class);
                    processOrder(order);
                    msg.ack();
                } catch (Exception e) {
                    if (msg.metaData().deliveredCount() >= 3) {
                        msg.term();
                    } else {
                        msg.nak();
                    }
                }
            }
        }
    }
}
```

## Security Configuration

```conf
# nats-server.conf
authorization {
  users = [
    { user: "publisher", password: "secret", permissions: { publish: ">" } }
    { user: "consumer", password: "secret", permissions: { subscribe: ">" } }
  ]
}

# TLS
tls {
  cert_file: "/path/to/server.crt"
  key_file: "/path/to/server.key"
  ca_file: "/path/to/ca.crt"
  verify: true
}
```

```typescript
const nc = await connect({
  servers: 'tls://nats.example.com:4222',
  user: 'publisher',
  pass: 'secret',
  tls: {
    ca: fs.readFileSync('/path/to/ca.crt'),
  },
});
```

## Clustering

```conf
# Server 1
cluster {
  name: nats-cluster
  listen: 0.0.0.0:6222
  routes = [
    nats://nats-2:6222
    nats://nats-3:6222
  ]
}

jetstream {
  store_dir: /data/jetstream
  max_mem: 1G
  max_file: 10G
}
```

## Monitoring Metrics

| Metric | Alert Threshold |
|--------|-----------------|
| Stream messages | Growth anomaly |
| Consumer pending | > 10000 |
| Consumer ack pending | > 1000 |
| Redelivery count | > 3 |
| Connection count | > expected |
