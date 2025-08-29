# `swagger` – _OpenAPI Response Types and Utilities_

> **Standardized response types and utilities for consistent API documentation and responses in Compozy's REST API.**

---

## 📑 Table of Contents

- [🎯 Overview](#-overview)
- [💡 Motivation](#-motivation)
- [⚡ Design Highlights](#-design-highlights)
- [🚀 Getting Started](#-getting-started)
- [📖 Usage](#-usage)
- [🎨 Examples](#-examples)
- [📚 API Reference](#-api-reference)
- [🧪 Testing](#-testing)
- [📦 Contributing](#-contributing)
- [📄 License](#-license)

---

## 🎯 Overview

The `swagger` package provides standardized response types and utilities for Compozy's REST API. It defines consistent structures for API responses, error handling, and pagination, ensuring uniform API documentation and client experience across all endpoints.

This package is designed to work seamlessly with Swagger/OpenAPI code generation tools and provides rich examples for API documentation.

---

## 💡 Motivation

- **API Consistency**: Ensure all API endpoints return responses in a standardized format
- **Documentation**: Provide rich examples and type information for Swagger/OpenAPI generation
- **Error Handling**: Standardize error response formats across the entire API
- **Pagination**: Consistent pagination metadata for list endpoints

---

## ⚡ Design Highlights

- **Standardized Responses**: Consistent response structure across all API endpoints
- **Rich Examples**: Comprehensive example values for API documentation
- **Type Safety**: Strong typing for all response components
- **Error Mapping**: Automatic HTTP status code mapping from error types
- **Pagination Support**: Built-in pagination metadata structure
- **Execution Context**: Specialized types for workflow execution responses

---

## 🚀 Getting Started

### Prerequisites

- Go 1.25+
- Familiarity with Compozy's core types (`engine/core`)

### Installation

```go
import "github.com/compozy/compozy/pkg/swagger"
```

### Quick Start

```go
// Create a successful response
response := swagger.StandardResponse{
    Status:  200,
    Message: "Success",
    Data:    userData,
}

// Create an error response
errorResp := swagger.ErrorResponse("VALIDATION_ERROR", "Invalid input", "Name is required")

// Create a paginated response
listResp := swagger.ListResponse{
    StandardResponse: swagger.StandardResponse{
        Status:  200,
        Message: "Success",
        Data:    users,
    },
    Meta: &swagger.PaginationMeta{
        Page:       1,
        PerPage:    20,
        Total:      100,
        TotalPages: 5,
    },
}
```

---

## 📖 Usage

### Standard Response Format

All API endpoints should return responses using the `StandardResponse` structure:

```go
type StandardResponse struct {
    Status  int    `json:"status"          example:"200"`
    Message string `json:"message"         example:"Success"`
    Data    any    `json:"data,omitempty"`
    Error   *Error `json:"error,omitempty"`
}
```

### Error Handling

```go
// Create error responses with automatic status code mapping
errorResp := swagger.ErrorResponse("NOT_FOUND", "User not found", "User ID: 123")

// Error codes automatically map to HTTP status codes:
// VALIDATION_ERROR, INVALID_INPUT -> 400
// NOT_FOUND -> 404
// CONFLICT -> 409
// Default -> 500
```

### Pagination

```go
// For list endpoints that support pagination
listResponse := swagger.ListResponse{
    StandardResponse: swagger.StandardResponse{
        Status:  200,
        Message: "Users retrieved successfully",
        Data:    users,
    },
    Meta: &swagger.PaginationMeta{
        Page:       1,
        PerPage:    20,
        Total:      100,
        TotalPages: 5,
    },
}
```

---

## 🎨 Examples

### Success Response

```json
{
  "status": 200,
  "message": "Success",
  "data": {
    "id": "2Z4PVTL6K27XVT4A3NPKMDD5BG",
    "name": "data-processor",
    "status": "active"
  }
}
```

### Error Response

```json
{
  "status": 400,
  "message": "Validation failed",
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Invalid input provided",
    "details": "Field 'name' is required"
  }
}
```

### Paginated List Response

```json
{
  "status": 200,
  "message": "Success",
  "data": [
    {
      "id": "2Z4PVTL6K27XVT4A3NPKMDD5BG",
      "name": "user-1"
    },
    {
      "id": "2Z4PVTL6K27XVT4A3NPKMDD5BH",
      "name": "user-2"
    }
  ],
  "meta": {
    "page": 1,
    "per_page": 20,
    "total": 100,
    "total_pages": 5
  }
}
```

### Workflow Execution Response

```json
{
  "id": "2Z4PVTL6K27XVT4A3NPKMDD5BG",
  "status": "completed",
  "component": "workflow",
  "workflow_id": "data-processing",
  "workflow_exec_id": "2Z4PVTL6K27XVT4A3NPKMDD5BG",
  "start_time": "2024-01-15T10:30:00Z",
  "end_time": "2024-01-15T10:35:00Z",
  "duration": "5m0s",
  "tasks": [
    {
      "id": "2Z4PVTL6K27XVT4A3NPKMDD5BH",
      "status": "completed",
      "component": "task"
    }
  ]
}
```

---

## 📚 API Reference

### Core Types

#### `StandardResponse`

Standard response structure for all API endpoints.

```go
type StandardResponse struct {
    Status  int    `json:"status"          example:"200"`
    Message string `json:"message"         example:"Success"`
    Data    any    `json:"data,omitempty"`
    Error   *Error `json:"error,omitempty"`
}
```

**Fields:**

- `Status`: HTTP status code
- `Message`: Human-readable message
- `Data`: Response payload (omitted if nil)
- `Error`: Error information (omitted if nil)

#### `Error`

Error response structure.

```go
type Error struct {
    Code    string `json:"code"              example:"VALIDATION_ERROR"`
    Message string `json:"message"           example:"Invalid input provided"`
    Details string `json:"details,omitempty" example:"Field 'name' is required"`
}
```

#### `PaginationMeta`

Pagination metadata for list responses.

```go
type PaginationMeta struct {
    Page       int `json:"page"        example:"1"`
    PerPage    int `json:"per_page"    example:"20"`
    Total      int `json:"total"       example:"100"`
    TotalPages int `json:"total_pages" example:"5"`
}
```

#### `ListResponse`

Extended response structure for paginated lists.

```go
type ListResponse struct {
    StandardResponse
    Meta *PaginationMeta `json:"meta,omitempty"`
}
```

#### `ExecutionResponse`

Response structure for execution-related endpoints.

```go
type ExecutionResponse struct {
    ID             core.ID         `json:"id"`
    Status         core.StatusType `json:"status"`
    Component      string          `json:"component"`
    WorkflowID     string          `json:"workflow_id"`
    WorkflowExecID core.ID         `json:"workflow_exec_id"`
    Input          core.Input      `json:"input,omitempty"`
    Output         core.Output     `json:"output,omitempty"`
    Error          *core.Error     `json:"error,omitempty"`
    StartTime      string          `json:"start_time"`
    EndTime        string          `json:"end_time"`
    Duration       string          `json:"duration"`
}
```

#### `WorkflowExecutionResponse`

Extended execution response with child executions.

```go
type WorkflowExecutionResponse struct {
    ExecutionResponse
    Tasks  []ExecutionResponse `json:"tasks,omitempty"`
    Agents []ExecutionResponse `json:"agents,omitempty"`
    Tools  []ExecutionResponse `json:"tools,omitempty"`
}
```

### Utility Functions

#### `ErrorResponse`

```go
func ErrorResponse(code, message, details string) router.Response
```

Creates a standardized error response with automatic HTTP status code mapping.

**Parameters:**

- `code`: Error code (e.g., "VALIDATION_ERROR", "NOT_FOUND")
- `message`: Human-readable error message
- `details`: Additional error details

**Returns:**

- `router.Response`: Router response with proper status code

**Status Code Mapping:**

- `VALIDATION_ERROR`, `INVALID_INPUT` → 400
- `NOT_FOUND` → 404
- `CONFLICT` → 409
- Default → 500

**Example:**

```go
// Returns 400 Bad Request
return swagger.ErrorResponse("VALIDATION_ERROR", "Invalid input", "Name is required")

// Returns 404 Not Found
return swagger.ErrorResponse("NOT_FOUND", "User not found", "User ID: 123")

// Returns 500 Internal Server Error
return swagger.ErrorResponse("INTERNAL_ERROR", "Something went wrong", "Database connection failed")
```

---

## 🧪 Testing

### Unit Tests

```go
func TestErrorResponse(t *testing.T) {
    tests := []struct {
        code           string
        expectedStatus int
    }{
        {"VALIDATION_ERROR", 400},
        {"INVALID_INPUT", 400},
        {"NOT_FOUND", 404},
        {"CONFLICT", 409},
        {"UNKNOWN_ERROR", 500},
    }

    for _, tt := range tests {
        t.Run(tt.code, func(t *testing.T) {
            resp := swagger.ErrorResponse(tt.code, "Test message", "Test details")
            assert.Equal(t, tt.expectedStatus, resp.Status)
            assert.NotNil(t, resp.Error)
            assert.Equal(t, tt.code, resp.Error.Code)
        })
    }
}
```

### Integration Tests

```go
func TestAPIResponseFormat(t *testing.T) {
    // Test that API responses match expected format
    response := swagger.StandardResponse{
        Status:  200,
        Message: "Success",
        Data:    map[string]any{"test": "data"},
    }

    jsonBytes, err := json.Marshal(response)
    assert.NoError(t, err)

    var parsed map[string]any
    err = json.Unmarshal(jsonBytes, &parsed)
    assert.NoError(t, err)
    assert.Equal(t, float64(200), parsed["status"])
    assert.Equal(t, "Success", parsed["message"])
    assert.NotNil(t, parsed["data"])
}
```

### Manual Testing

```bash
# Test error response generation
curl -X POST http://localhost:5001/api/v1/workflows \
  -H "Content-Type: application/json" \
  -d '{}'

# Expected response:
# {
#   "status": 400,
#   "message": "Validation failed",
#   "error": {
#     "code": "VALIDATION_ERROR",
#     "message": "Invalid input provided",
#     "details": "Field 'name' is required"
#   }
# }
```

---

## 📦 Contributing

See [CONTRIBUTING.md](../../CONTRIBUTING.md)

---

## 📄 License

BSL-1.1 License - see [LICENSE](../../LICENSE)
