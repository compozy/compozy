package builtin

const subagentDelegateInputSchema = `{
  "type": "object",
  "properties": {
    "task": {
      "type": "string",
      "minLength": 1,
      "maxLength": 120000,
      "description": "Self-contained brief. The task and formatted handoff must fit within 120000 characters."
    },
    "prior_subagent_id": {
      "type": "string",
      "maxLength": 64,
      "description": "Optional ID of a prior subagent from the same parent session and workspace."
    },
    "handoff": {
      "type": "object",
      "properties": {
        "objections": {
          "type": "string",
          "maxLength": 40000,
          "description": "Unresolved objections or verification failures from the prior round."
        },
        "findings": {
          "type": "string",
          "maxLength": 40000,
          "description": "Key findings or conclusions to carry forward."
        },
        "files": {
          "type": "array",
          "maxItems": 64,
          "items": {
            "type": "string",
            "maxLength": 1024
          },
          "description": "Relevant file paths already inspected or modified."
        }
      },
      "additionalProperties": false
    },
    "title": {
      "type": "string",
      "maxLength": 512
    },
    "role": {
      "type": "string",
      "enum": [
        "general",
        "implementation",
        "research",
        "review",
        "design",
        "test"
      ],
      "default": "general"
    },
    "target": {
      "type": "object",
      "properties": {
        "agent": {
          "type": "string"
        },
        "provider": {
          "type": "string"
        },
        "model": {
          "type": "string"
        },
        "reasoning_effort": {
          "type": "string"
        },
        "speed": {
          "type": "string",
          "enum": [
            "normal",
            "fast"
          ]
        },
        "acp_options": {
          "type": "array",
          "items": {
            "type": "object",
            "properties": {
              "id": {
                "type": "string",
                "minLength": 1
              },
              "value": {
                "type": "string"
              }
            },
            "required": [
              "id",
              "value"
            ],
            "additionalProperties": false
          }
        }
      },
      "required": [],
      "additionalProperties": false
    },
    "mode": {
      "type": "string",
      "enum": [
        "async",
        "wait"
      ],
      "default": "async"
    },
    "timeout_ms": {
      "type": "integer",
      "default": 600000,
      "description": "Wait only; clamped to 1000\u20133600000 ms. Timeout never cancels the child."
    },
    "idempotency_key": {
      "type": "string",
      "maxLength": 256,
      "description": "Defaults to the invoking tool call id."
    },
    "permission_mode": {
      "type": "string",
      "enum": [
        "inherit",
        "deny-all",
        "approve-reads",
        "approve-edits",
        "auto",
        "approve-all"
      ],
      "default": "inherit"
    },
    "tools": {
      "type": "array",
      "items": {
        "type": "string"
      },
      "description": "Omitted inherits the caller budget; [] grants none. Narrowing only."
    },
    "skills": {
      "type": "array",
      "items": {
        "type": "string"
      },
      "description": "Omitted inherits the caller budget; [] grants none. Narrowing only."
    },
    "mcp_servers": {
      "type": "array",
      "items": {
        "type": "string"
      },
      "description": "Omitted inherits the caller budget; [] grants none. Narrowing only."
    },
    "workspace_paths": {
      "type": "array",
      "items": {
        "type": "string"
      },
      "description": "Omitted inherits the caller budget; [] grants none. Narrowing only."
    },
    "isolation": {
      "type": "string",
      "enum": [
        "shared",
        "worktree"
      ],
      "description": "Shared uses the caller checkout (default); worktree creates a separate branch and checkout."
    },
    "base_ref": {
      "type": "string",
      "description": "Worktree only. Defaults to the linked prior subagent's worktree branch or the caller HEAD."
    }
  },
  "required": [
    "task"
  ],
  "additionalProperties": false
}`

const subagentStatusInputSchema = `{
  "type": "object",
  "properties": {
    "subagent_id": {
      "type": "string",
      "minLength": 1
    }
  },
  "required": [
    "subagent_id"
  ],
  "additionalProperties": false
}`

const subagentCancelInputSchema = `{
  "type": "object",
  "properties": {
    "subagent_id": {
      "type": "string",
      "minLength": 1
    },
    "reason": {
      "type": "string"
    }
  },
  "required": [
    "subagent_id"
  ],
  "additionalProperties": false
}`
