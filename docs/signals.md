# Event-Driven Workflows with Signal Triggers

Compozy supports event-driven workflow execution through signal triggers, allowing external systems to trigger workflows by sending events via HTTP API. This enables reactive architectures where workflows respond to real-time events like user actions, system notifications, or external service webhooks.

## Overview

The signals feature consists of three main components:

1. **Signal Triggers**: Workflow configuration that maps event names to workflows
2. **Events API**: REST endpoint for sending events that trigger workflows
3. **Dispatcher Workflow**: Background service that routes events to target workflows

### Architecture

```
External System → POST /api/v1/events → Dispatcher Workflow → Target Workflow(s)
```

The Dispatcher Workflow is a long-running singleton that:

- Listens for incoming event signals
- Routes events to appropriate workflows based on configuration
- Starts target workflows as child workflows with event payload as input
- Validates event payloads against optional JSON schemas

## Platform Developer Guide

### Configuring Signal Triggers

To make a workflow respond to events, add a `triggers` section to your workflow YAML:

```yaml
id: order-processor
version: "1.0.0"
description: "Process new orders from external systems"

# Configure workflow to respond to events
triggers:
    - type: signal
      name: "order.created"
      schema: # Optional payload validation
          type: object
          properties:
              orderId:
                  type: string
                  description: "Unique order identifier"
              customerId:
                  type: string
              amount:
                  type: number
                  minimum: 0
          required:
              - orderId
              - customerId

config:
    input:
        type: object
        properties:
            orderId:
                type: string
            customerId:
                type: string
            amount:
                type: number

tasks:
    - id: process-order
      type: basic
      $use: agent(local::agents.#(id=="order-processor"))
      action: default
      final: true
```

### Trigger Configuration

| Field    | Type   | Required | Description                                                 |
| -------- | ------ | -------- | ----------------------------------------------------------- |
| `type`   | string | Yes      | Must be `"signal"`                                          |
| `name`   | string | Yes      | Unique event name (e.g., "user.created", "order.completed") |
| `schema` | object | No       | JSON schema for payload validation                          |

### Event Naming Conventions

Use descriptive, hierarchical names:

- ✅ `user.created`, `user.updated`, `user.deleted`
- ✅ `order.placed`, `order.shipped`, `order.completed`
- ✅ `payment.processed`, `payment.failed`
- ❌ `event1`, `trigger`, `notification`

### Accessing Event Data

Event payloads are available in your workflow through template variables:

```yaml
tasks:
    - id: send-welcome-email
      type: basic
      $use: agent(local::agents.#(id=="email-agent"))
      action: send_email
      with:
          recipient: "{{ .input.email }}" # From event payload
          user_name: "{{ .input.firstName }}" # From event payload
          order_id: "{{ .input.orderId }}" # From event payload
```

### Schema Validation

Optional schema validation ensures event payloads match expected structure:

```yaml
triggers:
    - type: signal
      name: "product.updated"
      schema:
          type: object
          properties:
              productId:
                  type: string
                  pattern: "^prod_[a-zA-Z0-9]+$"
              changes:
                  type: array
                  items:
                      type: object
                      properties:
                          field:
                              type: string
                          oldValue:
                              type: string
                          newValue:
                              type: string
                      required: [field, newValue]
          required:
              - productId
              - changes
```

When validation fails:

- Event is rejected and child workflow is not started
- Error is logged with validation details
- HTTP API returns error response to sender

### Multiple Workflows per Event

Multiple workflows can listen to the same event:

```yaml
# Workflow 1: Send notification
id: notification-sender
triggers:
    - type: signal
      name: "order.created"
tasks:
    - id: send-notification
      # ... send email/SMS notification

---
# Workflow 2: Update inventory
id: inventory-updater
triggers:
    - type: signal
      name: "order.created"
tasks:
    - id: update-stock
      # ... decrease inventory
```

Both workflows will execute independently when an "order.created" event is received.

### Example: E-commerce Order Processing

Complete example showing event-driven order processing:

```yaml
id: ecommerce-order-flow
version: "1.0.0"
description: "Complete order processing triggered by order creation events"

triggers:
    - type: signal
      name: "order.created"
      schema:
          type: object
          properties:
              orderId:
                  type: string
              customerId:
                  type: string
              items:
                  type: array
                  items:
                      type: object
                      properties:
                          productId:
                              type: string
                          quantity:
                              type: integer
                              minimum: 1
                          price:
                              type: number
                              minimum: 0
              totalAmount:
                  type: number
                  minimum: 0
              customerEmail:
                  type: string
                  format: email
          required:
              - orderId
              - customerId
              - items
              - totalAmount
              - customerEmail

config:
    input:
        $ref: "#/triggers/0/schema"

agents:
    - id: order-processor
      config:
          $ref: global::models.#(id=="gpt-4o")
      instructions: |
          You are an order processing assistant. You handle:
          1. Order validation and verification
          2. Inventory management
          3. Customer communication
          4. Payment processing coordination

tools:
    - id: inventory-check
      description: "Check product availability"
      execute: ./tools/check_inventory.ts
    - id: send-email
      description: "Send email notifications"
      execute: ./tools/send_email.ts
    - id: process-payment
      description: "Process customer payment"
      execute: ./tools/process_payment.ts

tasks:
    - id: validate-order
      type: basic
      $use: agent(local::agents.#(id=="order-processor"))
      action: default
      with:
          task: "Validate order {{ .input.orderId }} for customer {{ .input.customerId }}"
          order_data: "{{ .input }}"
      on_success:
          next: check-inventory
      on_error:
          next: send-error-notification

    - id: check-inventory
      type: basic
      $use: tool(local::tools.#(id=="inventory-check"))
      with:
          items: "{{ .input.items }}"
      on_success:
          next: process-payment-step
      on_error:
          next: send-inventory-error

    - id: process-payment-step
      type: basic
      $use: tool(local::tools.#(id=="process-payment"))
      with:
          amount: "{{ .input.totalAmount }}"
          customer_id: "{{ .input.customerId }}"
      on_success:
          next: send-confirmation
      on_error:
          next: send-payment-error

    - id: send-confirmation
      type: basic
      $use: tool(local::tools.#(id=="send-email"))
      with:
          recipient: "{{ .input.customerEmail }}"
          subject: "Order Confirmation - {{ .input.orderId }}"
          template: "order_confirmation"
          data: "{{ .input }}"
      final: true

    - id: send-error-notification
      type: basic
      $use: tool(local::tools.#(id=="send-email"))
      with:
          recipient: "{{ .input.customerEmail }}"
          subject: "Order Processing Error - {{ .input.orderId }}"
          template: "order_error"
          error: "{{ .error }}"
      final: true

    - id: send-inventory-error
      type: basic
      $use: tool(local::tools.#(id=="send-email"))
      with:
          recipient: "{{ .input.customerEmail }}"
          subject: "Inventory Issue - {{ .input.orderId }}"
          template: "inventory_error"
          items: "{{ .input.items }}"
      final: true

    - id: send-payment-error
      type: basic
      $use: tool(local::tools.#(id=="send-email"))
      with:
          recipient: "{{ .input.customerEmail }}"
          subject: "Payment Processing Error - {{ .input.orderId }}"
          template: "payment_error"
          error: "{{ .error }}"
      final: true
```

## API Consumer Guide

External systems trigger workflows by sending events to the Events API endpoint `POST /api/v1/events` with an event name and optional payload.

For complete API reference including authentication, request/response formats, SDK examples, error handling, and integration patterns, see the **[Events API Reference](./events-api.md)**.

## Monitoring and Observability

### Logging

The system provides structured logging for:

- **Event Reception**: When events are received via API
- **Signal Routing**: Which workflows are triggered by events
- **Validation Failures**: When event payloads fail schema validation
- **Workflow Dispatch**: When child workflows are started
- **Error Conditions**: Any failures in the event processing pipeline

### Temporal UI

Monitor signal-triggered workflows in Temporal UI:

1. **Dispatcher Workflow**: View the long-running dispatcher that processes all events
2. **Signal History**: See all events received and processed
3. **Child Workflows**: Monitor individual workflow executions triggered by events
4. **Error Investigation**: Debug failed workflows and validation errors

### Best Practices

1. **Event Naming**: Use clear, hierarchical naming (e.g., `service.action`)
2. **Schema Validation**: Always define schemas for production workflows
3. **Error Handling**: Handle validation and processing failures gracefully
4. **Authentication**: Use proper bearer tokens for production deployments
5. **Monitoring**: Set up alerting for failed events and validation errors
6. **Testing**: Test event flows end-to-end before production deployment

## Examples

### Complete Order Processing System

The `examples/order-processor/` directory contains a comprehensive e-commerce order processing workflow that demonstrates:

- **Event-Driven Triggers**: Responds to `order.created` events
- **Schema Validation**: Validates complex order payloads
- **Multi-Stage Processing**: Order validation → Inventory → Shipping → Payment → Confirmation
- **AI Integration**: Uses AI agents for validation and customer communication
- **Error Handling**: Comprehensive error handling with customer notifications

To run the example:

```bash
cd examples/order-processor
export OPENAI_API_KEY="your-api-key"
compozy dev
```

Then send a test event:

```bash
curl -X POST http://localhost:8080/api/v1/events \
    -H "Authorization: Bearer test-token" \
    -H "Content-Type: application/json" \
    -d '{
    "name": "order.created",
    "payload": {
      "orderId": "order_12345",
      "customerId": "customer_67890",
      "customerEmail": "customer@example.com",
      "items": [
        {
          "productId": "prod_001",
          "name": "Wireless Headphones",
          "quantity": 1,
          "price": 99.99
        }
      ],
      "totalAmount": 99.99,
      "shippingAddress": {
        "street": "123 Main St",
        "city": "New York",
        "zipCode": "10001",
        "country": "US"
      }
    }
  }'
```

### Multiple Workflows Example

The `examples/notification-sender/` workflow demonstrates how multiple workflows can respond to the same event. It listens for the same `order.created` events and sends notifications via email, SMS, and push channels.

This shows how to build modular, event-driven architectures where different services handle different aspects of the same business event.

### Security Considerations

- **Authentication Required**: All API calls must include valid Bearer tokens
- **Schema Validation**: Validates event payloads to prevent malicious data
- **Event Source Validation**: Consider whitelisting trusted event sources
- **Rate Limiting**: Implement rate limits to prevent abuse (configure in API gateway)
- **Audit Logging**: All events are logged for security and compliance auditing
