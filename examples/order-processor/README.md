# Order Processor Example

This example demonstrates Compozy's event-driven workflow capabilities using signal triggers. The workflow processes order creation events through a complete e-commerce order pipeline including validation, inventory checking, payment processing, and customer notifications.

## Features

- **Event-Driven Triggers**: Responds to `order.created` events via HTTP API
- **Schema Validation**: Validates event payloads against JSON schema
- **Multi-Stage Processing**: Order validation → Inventory check → Shipping calculation → Payment processing
- **Error Handling**: Comprehensive error handling with customer notifications
- **AI Integration**: Uses AI agents for order validation and customer communication
- **External Tool Integration**: Mock tools for inventory, payment, and shipping

## Quick Start

1. **Setup Environment**:

    ```bash
    export OPENAI_API_KEY="your-openai-api-key"
    ```

2. **Start Compozy**:

    ```bash
    compozy dev
    ```

3. **Send Test Event**:
    ```bash
    curl -X POST http://localhost:8080/api/v1/events \
        -H "Authorization: Bearer test-token" \
        -H "Content-Type: application/json" \
        -d @test-order.json
    ```

## Test Event Example

Create a `test-order.json` file:

```json
{
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
            },
            {
                "productId": "prod_002",
                "name": "Phone Case",
                "quantity": 2,
                "price": 15.99
            }
        ],
        "totalAmount": 131.97,
        "shippingAddress": {
            "street": "123 Main St",
            "city": "New York",
            "zipCode": "10001",
            "country": "US"
        }
    }
}
```

## Workflow Steps

1. **Order Validation**: AI agent validates order details and checks for issues
2. **Inventory Check**: Verifies product availability
3. **Shipping Calculation**: Calculates shipping costs and delivery estimates
4. **Payment Processing**: Processes customer payment
5. **Confirmation Email**: Sends order confirmation to customer

If any step fails, appropriate error notifications are sent to the customer.

## Tools Overview

### check_inventory.ts

- Checks product availability against mock inventory
- Returns availability status and warnings for low stock
- Simulates real inventory management system

### calculate_shipping.ts

- Calculates shipping costs based on weight and destination
- Provides delivery estimates
- Handles free shipping and expedited options

### process_payment.ts

- Mock payment processing with success/failure simulation
- Calculates processing fees
- Returns transaction details

### send_email.ts

- Sends email notifications using templates
- Supports multiple email types (confirmation, errors)
- Mock implementation that logs email content

## Schema Validation

The workflow validates incoming events against this schema:

- **orderId**: Required string
- **customerId**: Required string
- **customerEmail**: Required email format
- **items**: Required array with product details
- **totalAmount**: Required positive number
- **shippingAddress**: Required object with address fields

Invalid events are rejected and do not trigger the workflow.

## Monitoring

Monitor the workflow execution in:

1. **Terminal Output**: See tool execution and email notifications
2. **Temporal UI**: View workflow execution details at http://localhost:8080
3. **Logs**: Structured logging for debugging and monitoring

## Customization

### Adding New Tools

1. Create new TypeScript tool in `tools/` directory
2. Add tool definition to `workflow.yaml`
3. Reference tool in workflow tasks

### Modifying Email Templates

Edit templates in `tools/send_email.ts` to customize notification content.

### Extending Validation

Update the schema in `workflow.yaml` to add new validation rules.

### Adding New Event Types

Create additional trigger configurations for different event types (e.g., `order.cancelled`, `payment.refunded`).

## Production Considerations

This example uses mock implementations. For production:

1. **Replace Mock Tools**: Integrate with real inventory, payment, and shipping APIs
2. **Authentication**: Implement proper JWT-based authentication
3. **Email Service**: Connect to actual email providers (SendGrid, AWS SES, etc.)
4. **Error Handling**: Add retry logic and dead letter queues
5. **Monitoring**: Set up proper logging and alerting
6. **Rate Limiting**: Implement API rate limits
7. **Data Persistence**: Add database storage for order tracking

## API Documentation

See the main [Signals Documentation](../../docs/signals.md) for complete API reference and authentication details.
