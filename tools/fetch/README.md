# @compozy/tool-fetch

A Compozy tool for making HTTP/HTTPS requests with automatic JSON handling, custom headers, and comprehensive error handling.

## Overview

The fetch tool provides a simple interface for making HTTP requests from within Compozy workflows. It supports all standard HTTP methods, automatically handles JSON serialization/deserialization, and provides detailed error information.

## Installation

This tool is included in the Compozy runtime and can be referenced in your workflow configurations.

## Usage

### Basic Example

```yaml
# workflow.yaml
name: fetch-example
tasks:
  - id: get-data
    type: basic
    tool: fetch
    input:
      url: "https://api.example.com/data"
```

### Input Parameters

| Parameter | Type             | Required | Default | Description                                                |
| --------- | ---------------- | -------- | ------- | ---------------------------------------------------------- |
| `url`     | string           | Yes      | -       | The URL to fetch                                           |
| `method`  | string           | No       | GET     | HTTP method (GET, POST, PUT, DELETE, PATCH, HEAD, OPTIONS) |
| `headers` | object           | No       | {}      | Custom HTTP headers                                        |
| `body`    | string \| object | No       | -       | Request body (automatically JSON-serialized for objects)   |
| `timeout` | number           | No       | 30000   | Request timeout in milliseconds                            |

### Output Format

#### Success Response

```json
{
  "status": 200,
  "statusText": "OK",
  "headers": {
    "content-type": "application/json",
    "x-custom-header": "value"
  },
  "body": "Response body as string",
  "success": true
}
```

#### Error Response

```json
{
  "error": "Error description",
  "code": "ERROR_CODE"
}
```

### Error Codes

| Code              | Description                                            |
| ----------------- | ------------------------------------------------------ |
| `INVALID_URL`     | The provided URL is not valid                          |
| `INVALID_METHOD`  | The HTTP method is not supported                       |
| `INVALID_REQUEST` | Request configuration is invalid (e.g., body with GET) |
| `INVALID_BODY`    | Body parameter has invalid type                        |
| `TIMEOUT`         | Request exceeded the timeout limit                     |
| `ECONNREFUSED`    | Connection was refused by the server                   |
| `ENOTFOUND`       | DNS lookup failed                                      |
| `CERT_ERROR`      | SSL certificate validation failed                      |
| `NETWORK_ERROR`   | General network error                                  |
| `REQUEST_FAILED`  | Other request failures                                 |

## Examples

### GitHub API - Get Repository Info

```yaml
name: github-repo-info
tasks:
  - id: get-repo
    type: basic
    tool: fetch
    input:
      url: "https://api.github.com/repos/compozy/compozy"
      headers:
        Accept: "application/vnd.github.v3+json"
        User-Agent: "Compozy-App"
```

### OpenAI API - Chat Completion

```yaml
name: openai-chat
tasks:
  - id: chat-completion
    type: basic
    tool: fetch
    input:
      url: "https://api.openai.com/v1/chat/completions"
      method: "POST"
      headers:
        Authorization: "Bearer ${OPENAI_API_KEY}"
        Content-Type: "application/json"
      body:
        model: "gpt-3.5-turbo"
        messages:
          - role: "user"
            content: "Hello, how are you?"
        temperature: 0.7
```

### Slack Webhook - Send Message

```yaml
name: slack-notification
tasks:
  - id: send-message
    type: basic
    tool: fetch
    input:
      url: "${SLACK_WEBHOOK_URL}"
      method: "POST"
      body:
        text: "Deployment completed successfully! 🚀"
        attachments:
          - color: "good"
            title: "Deployment Details"
            fields:
              - title: "Environment"
                value: "Production"
                short: true
              - title: "Version"
                value: "v1.2.3"
                short: true
```

### REST API - CRUD Operations

```yaml
name: rest-crud-example
tasks:
  # Create
  - id: create-user
    type: basic
    tool: fetch
    input:
      url: "https://api.example.com/users"
      method: "POST"
      headers:
        Authorization: "Bearer ${API_TOKEN}"
      body:
        name: "John Doe"
        email: "john@example.com"
        role: "admin"

  # Read
  - id: get-user
    type: basic
    tool: fetch
    input:
      url: "https://api.example.com/users/123"
      headers:
        Authorization: "Bearer ${API_TOKEN}"

  # Update
  - id: update-user
    type: basic
    tool: fetch
    input:
      url: "https://api.example.com/users/123"
      method: "PUT"
      headers:
        Authorization: "Bearer ${API_TOKEN}"
      body:
        name: "John Smith"
        email: "john.smith@example.com"

  # Delete
  - id: delete-user
    type: basic
    tool: fetch
    input:
      url: "https://api.example.com/users/123"
      method: "DELETE"
      headers:
        Authorization: "Bearer ${API_TOKEN}"
```

### GraphQL Request

```yaml
name: graphql-query
tasks:
  - id: query-data
    type: basic
    tool: fetch
    input:
      url: "https://api.example.com/graphql"
      method: "POST"
      headers:
        Content-Type: "application/json"
        Authorization: "Bearer ${GRAPHQL_TOKEN}"
      body:
        query: |
          query GetUser($id: ID!) {
            user(id: $id) {
              id
              name
              email
              posts {
                title
                createdAt
              }
            }
          }
        variables:
          id: "user-123"
```

### Form Data Submission

```yaml
name: form-submission
tasks:
  - id: submit-form
    type: basic
    tool: fetch
    input:
      url: "https://api.example.com/submit"
      method: "POST"
      headers:
        Content-Type: "application/x-www-form-urlencoded"
      body: "name=John+Doe&email=john%40example.com&message=Hello+World"
```

### Handling Timeouts

```yaml
name: timeout-example
tasks:
  - id: quick-request
    type: basic
    tool: fetch
    input:
      url: "https://api.example.com/quick"
      timeout: 5000 # 5 seconds

  - id: slow-request
    type: basic
    tool: fetch
    input:
      url: "https://api.example.com/slow-endpoint"
      timeout: 60000 # 60 seconds for slow operations
```

### Error Handling

```yaml
name: error-handling-example
tasks:
  - id: fetch-with-retry
    type: basic
    tool: fetch
    input:
      url: "https://api.example.com/data"
    onError:
      - type: retry
        maxAttempts: 3
        backoff: exponential

  - id: check-response
    type: router
    routes:
      - when: "tasks['fetch-with-retry'].status >= 200 && tasks['fetch-with-retry'].status < 300"
        tasks:
          - id: process-success
            type: basic
            tool: your-tool
            input:
              data: "{{ tasks['fetch-with-retry'].body }}"

      - when: "tasks['fetch-with-retry'].status >= 400"
        tasks:
          - id: handle-error
            type: basic
            tool: your-error-handler
            input:
              error: "{{ tasks['fetch-with-retry'] }}"
```

### Working with APIs that Return Different Content Types

```yaml
name: content-type-handling
tasks:
  # JSON API
  - id: get-json
    type: basic
    tool: fetch
    input:
      url: "https://api.example.com/data.json"
    output:
      parsedData: "{{ fromJson(output.body) }}"

  # Plain text
  - id: get-text
    type: basic
    tool: fetch
    input:
      url: "https://api.example.com/readme.txt"
    output:
      textContent: "{{ output.body }}"

  # HTML
  - id: get-html
    type: basic
    tool: fetch
    input:
      url: "https://example.com/page.html"
    output:
      htmlContent: "{{ output.body }}"
```

### Authentication Examples

```yaml
name: auth-examples
tasks:
  # Bearer Token
  - id: bearer-auth
    type: basic
    tool: fetch
    input:
      url: "https://api.example.com/protected"
      headers:
        Authorization: "Bearer ${API_TOKEN}"

  # Basic Auth
  - id: basic-auth
    type: basic
    tool: fetch
    input:
      url: "https://api.example.com/secure"
      headers:
        Authorization: "Basic {{ base64('username:password') }}"

  # API Key in Header
  - id: api-key-header
    type: basic
    tool: fetch
    input:
      url: "https://api.example.com/data"
      headers:
        X-API-Key: "${API_KEY}"

  # API Key in Query
  - id: api-key-query
    type: basic
    tool: fetch
    input:
      url: "https://api.example.com/data?api_key=${API_KEY}"
```

### Paginated API Requests

```yaml
name: pagination-example
tasks:
  - id: fetch-page
    type: collection
    items: "{{ range(1, 5) }}" # Fetch 5 pages
    tasks:
      - id: get-page
        type: basic
        tool: fetch
        input:
          url: "https://api.example.com/items?page={{ item }}&limit=100"
          headers:
            Authorization: "Bearer ${API_TOKEN}"

  - id: combine-results
    type: aggregate
    input:
      allItems: |
        {{ tasks['fetch-page'].outputs 
           | map(output => fromJson(output.body).items) 
           | flatten }}
```

### Webhook with Signature Verification

```yaml
name: webhook-sender
tasks:
  - id: generate-signature
    type: basic
    tool: your-crypto-tool
    input:
      payload: "{{ toJson(webhookData) }}"
      secret: "${WEBHOOK_SECRET}"

  - id: send-webhook
    type: basic
    tool: fetch
    input:
      url: "https://webhook.site/your-endpoint"
      method: "POST"
      headers:
        Content-Type: "application/json"
        X-Webhook-Signature: "{{ tasks['generate-signature'].signature }}"
        X-Webhook-Timestamp: "{{ now() }}"
      body: "{{ webhookData }}"
```

## Best Practices

1. **Always Set Timeouts**: Configure appropriate timeouts based on expected response times
2. **Handle Errors**: Use error handling mechanisms in your workflows
3. **Secure Credentials**: Store API keys and tokens in environment variables
4. **Set User-Agent**: Include a User-Agent header to identify your application
5. **Respect Rate Limits**: Implement delays between requests when necessary
6. **Validate Responses**: Check status codes and validate response formats
7. **Use HTTPS**: Always use HTTPS for sensitive data transmission

## Limitations

- Maximum timeout is determined by the Compozy runtime configuration
- Large response bodies may impact performance
- Binary data is returned as string (base64 encoding recommended for binary APIs)
- Follows system proxy settings if configured

## Troubleshooting

### Common Issues

1. **TIMEOUT Error**: Increase the timeout parameter or check if the server is responding slowly
2. **CERT_ERROR**: The SSL certificate may be self-signed or expired. Contact the API provider
3. **ECONNREFUSED**: Ensure the server is running and the port is correct
4. **INVALID_URL**: Check the URL format includes protocol (http:// or https://)
5. **Network Errors**: Verify internet connectivity and DNS resolution

### Debug Tips

- Use the echo endpoint for testing: `https://httpbin.org/anything`
- Check response headers for debugging information
- Log the full response object to see all available data
- Test with curl or similar tools to verify the API is working
