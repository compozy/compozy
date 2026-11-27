# Remote MCP tool inputs

Generated from the schemas used by the shipped server. Each section is the JSON Schema for one tool's input.

## proto_shares_list

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "properties": {
    "cursor": {
      "type": "string"
    }
  }
}
```

## proto_share_show

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "properties": {
    "share": {
      "type": "string",
      "minLength": 1
    }
  },
  "required": ["share"]
}
```

## proto_inbox

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "properties": {
    "cursor": {
      "type": "string"
    },
    "status": {
      "default": "open",
      "type": "string",
      "enum": ["open", "resolved"]
    },
    "share": {
      "type": "string"
    },
    "involving": {
      "type": "string",
      "const": "me"
    },
    "limit": {
      "default": 50,
      "type": "integer",
      "minimum": 1,
      "maximum": 100
    }
  }
}
```

## proto_comments_show

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "properties": {
    "share": {
      "type": "string",
      "minLength": 1
    },
    "id": {
      "type": "string",
      "minLength": 1
    }
  },
  "required": ["share", "id"]
}
```

## proto_comments_reply

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "properties": {
    "share": {
      "type": "string",
      "minLength": 1
    },
    "id": {
      "type": "string",
      "minLength": 1
    },
    "body": {
      "type": "string",
      "minLength": 1,
      "maxLength": 10000
    }
  },
  "required": ["share", "id", "body"]
}
```

## proto_comments_resolve

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "properties": {
    "share": {
      "type": "string",
      "minLength": 1
    },
    "id": {
      "type": "string",
      "minLength": 1
    }
  },
  "required": ["share", "id"]
}
```

## proto_comments_reopen

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "properties": {
    "share": {
      "type": "string",
      "minLength": 1
    },
    "id": {
      "type": "string",
      "minLength": 1
    }
  },
  "required": ["share", "id"]
}
```

## proto_comments_attachments

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "properties": {
    "share": {
      "type": "string",
      "minLength": 1
    },
    "id": {
      "type": "string",
      "minLength": 1
    }
  },
  "required": ["share", "id"]
}
```

## proto_review_status

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "properties": {
    "share": {
      "type": "string",
      "minLength": 1
    }
  },
  "required": ["share"]
}
```
