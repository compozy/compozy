package opendesign

const lintInputSchema = `{
  "type": "object",
  "additionalProperties": false,
  "required": [
    "paths"
  ],
  "properties": {
    "paths": {
      "type": "array",
      "minItems": 1,
      "maxItems": 32,
      "uniqueItems": true,
      "items": {
        "type": "string",
        "minLength": 1
      }
    }
  }
}`

const lintOutputSchema = `{
  "type": "object",
  "additionalProperties": false,
  "required": [
    "artifacts",
    "passed"
  ],
  "properties": {
    "passed": {
      "type": "boolean"
    },
    "artifacts": {
      "type": "array",
      "items": {
        "type": "object",
        "additionalProperties": false,
        "required": [
          "path",
          "sha256",
          "findings",
          "counts",
          "feedback",
          "passed"
        ],
        "properties": {
          "path": {
            "type": "string"
          },
          "sha256": {
            "type": "string",
            "pattern": "^[0-9a-f]{64}$"
          },
          "passed": {
            "type": "boolean"
          },
          "feedback": {
            "type": "string"
          },
          "counts": {
            "type": "object",
            "additionalProperties": false,
            "required": [
              "P0",
              "P1",
              "P2"
            ],
            "properties": {
              "P0": {
                "type": "integer",
                "minimum": 0
              },
              "P1": {
                "type": "integer",
                "minimum": 0
              },
              "P2": {
                "type": "integer",
                "minimum": 0
              }
            }
          },
          "findings": {
            "type": "array",
            "items": {
              "type": "object",
              "additionalProperties": false,
              "required": [
                "id",
                "severity",
                "message",
                "fix"
              ],
              "properties": {
                "id": {
                  "type": "string"
                },
                "severity": {
                  "type": "string",
                  "enum": [
                    "P0",
                    "P1",
                    "P2"
                  ]
                },
                "message": {
                  "type": "string"
                },
                "fix": {
                  "type": "string"
                },
                "snippet": {
                  "type": "string"
                }
              }
            }
          }
        }
      }
    }
  }
}`
