package builtin

const sessionRecordCompactOutputSchema = `{"type":"object","required":["session"],"properties":{"session":` + sessionCompactRecordSchema + `},"additionalProperties":false}`

const sessionCompactRecordSchema = `{
 "type": "object",
 "properties": {
  "id": {"type":"string"},
  "profile_id": {"type":"string"},
  "name": {"type":"string"},
  "agent_name": {"type":"string"},
  "type": {"type":"string"},
  "state": {"type":"string"},
  "badge": {"type":"string"},
  "board_column": {"type":"string"},
  "attention": {"type":"string"},
  "waiting_on": {"type":"string"},
  "workspace_id": {"type":"string"},
  "worktree_id": {"type":"string"},
  "branch": {"type":"string"},
  "stop_reason": {"type":"string"},
  "stop_detail": {"type":"string"},
  "stop_cause": {"type":"string"},
  "created_at": {"type":"string"},
  "updated_at": {"type":"string"},
  "permission_mode_degraded": {"type":"boolean"},
  "attention_dismissible": {"type":"boolean"},
  "runtime": {
   "type": "object",
   "properties": {
    "status": {"type":"string"},
    "failure": {"type":"string"},
    "generation": {"type":"integer"},
    "selection_revision": {"type":"integer"},
    "effective": {
     "type": [
      "object",
      "null"
     ],
     "properties": {
      "provider": {"type":"string"},
      "model": {"type":"string"},
      "reasoning_effort": {"type":"string"},
      "speed": {"type":"string"},
      "acp_options": {
       "type": "array",
       "items": {
        "type": "object",
        "properties": {
         "id": {"type":"string"},
         "value_id": {"type":"string"},
         "bool_value": {"type":"boolean"}
        },
        "additionalProperties": false,
        "required": [
         "id"
        ]
       }
      },
      "speed_resolution": {
       "type": [
        "object",
        "null"
       ],
       "properties": {
        "requested": {"type":"string"},
        "status": {"type":"string"},
        "reason": {"type":"string"}
       },
       "additionalProperties": false,
       "required": [
        "requested",
        "status"
       ]
      }
     },
     "additionalProperties": false,
     "required": [
      "provider"
     ]
    }
   },
   "additionalProperties": false,
   "required": [
    "status",
    "selection_revision"
   ]
  },
  "transcript_epoch": {"type":"integer"},
  "lineage": {
   "type": "object",
   "properties": {
    "kind": {"type":"string"},
    "parent_session_id": {"type":"string"},
    "root_session_id": {"type":"string"},
    "ttl_expires_at": {"type":"string"},
    "spawn_budget": {
     "type": "object",
     "properties": {
      "max_children": {"type":"integer"},
      "max_depth": {"type":"integer"},
      "ttl_seconds": {"type":"integer"},
      "max_active_per_workspace": {"type":"integer"}
     },
     "additionalProperties": false,
     "required": [
      "max_children",
      "max_depth",
      "ttl_seconds"
     ]
    }
   },
   "additionalProperties": false
  },
  "pull_request": {
   "type": [
    "object",
    "null"
   ],
   "properties": {
    "title": {"type":"string"},
    "provider": {"type":"string"},
    "url": {"type":"string"},
    "state": {"type":"string"},
    "head_sha": {"type":"string"},
    "review_decision": {"type":"string"},
    "mergeable": {"type":"string"},
    "number": {"type":"integer"},
    "draft": {
     "type": [
      "boolean",
      "null"
     ]
    },
    "ready_to_merge": {"type":"boolean"},
    "checks": {
     "type": "object",
     "properties": {
      "state": {"type":"string"},
      "passed": {"type":"integer"},
      "failed": {"type":"integer"},
      "pending": {"type":"integer"},
      "failed_names": {
       "type": [
        "array",
        "null"
       ],
       "items": {"type":"string"}
      }
     },
     "additionalProperties": false,
     "required": [
      "state",
      "passed",
      "failed",
      "pending",
      "failed_names"
     ]
    }
   },
   "additionalProperties": false,
   "required": [
    "provider",
    "number",
    "url",
    "state",
    "draft",
    "head_sha",
    "checks",
    "review_decision",
    "mergeable",
    "ready_to_merge"
   ]
  },
  "archived_at": {
   "type": [
    "string",
    "null"
   ]
  },
  "pending_interactions": {
   "type": [
    "array",
    "null"
   ],
   "items": {
    "type": "object",
    "properties": {
     "session_id": {"type":"string"},
     "interaction_id": {"type":"string"},
     "kind": {"type":"string"},
     "provider_request_id": {"type":"string"},
     "turn_id": {"type":"string"},
     "title": {"type":"string"},
     "tool_id": {"type":"string"},
     "status": {"type":"string"},
     "created_at": {"type":"string"},
     "expires_at": {"type":"string"},
     "resolved_at": {"type":"string"},
     "resolution": {"type":"string"},
     "resolved_by": {"type":"string"},
     "choices": {
      "type": "array",
      "items": {"type":"string"}
     },
     "decisions": {
      "type": "array",
      "items": {"type":"string"}
     }
    },
    "additionalProperties": false,
    "required": [
     "session_id",
     "interaction_id",
     "kind",
     "provider_request_id",
     "status",
     "created_at"
    ]
   }
  },
  "queue": {
   "type": "object",
   "properties": {
    "entries": {"type":"integer"},
    "cap": {"type":"integer"}
   },
   "additionalProperties": false,
   "required": [
    "entries",
    "cap"
   ]
  },
  "children": {
   "type": "object",
   "properties": {
    "total": {"type":"integer"},
    "needs_you": {"type":"integer"},
    "working": {"type":"integer"},
    "waiting": {"type":"integer"},
    "in_review": {"type":"integer"},
    "done": {"type":"integer"}
   },
   "additionalProperties": false,
   "required": [
    "total",
    "needs_you",
    "working",
    "waiting",
    "in_review",
    "done"
   ]
  },
  "subagent_summary": {
   "type": "object",
   "properties": {
    "live": {"type":"integer"},
    "total": {"type":"integer"},
    "failed": {"type":"integer"},
    "attention": {"type":"integer"},
    "most_urgent": {"type":"string"}
   },
   "additionalProperties": false,
   "required": [
    "live",
    "total",
    "failed",
    "attention",
    "most_urgent"
   ]
  },
  "later": {
   "type": "object",
   "properties": {
    "parked_at": {"type":"string"},
    "reason": {"type":"string"},
    "return_at": {"type":"string"},
    "pending_messages": {"type":"integer"}
   },
   "additionalProperties": false,
   "required": [
    "parked_at",
    "pending_messages"
   ]
  },
  "failure": {
   "type": "object",
   "properties": {
    "kind": {"type":"string"},
    "reason_code": {"type":"string"},
    "summary": {"type":"string"},
    "crash_bundle_path": {"type":"string"}
   },
   "additionalProperties": false,
   "required": [
    "kind"
   ]
  }
 },
 "additionalProperties": false,
 "required": [
  "id",
  "profile_id",
  "agent_name",
  "state",
  "badge",
  "board_column",
  "permission_mode_degraded",
  "runtime",
  "pull_request",
  "pending_interactions",
  "archived_at",
  "created_at",
  "updated_at"
 ]
}`

const sessionSpawnCompactOutputSchema = `{"type":"object","required":["session_id","spawn_role","spawn_depth","ttl_expires_at"],"properties":{"session_id":{"type":"string"},"spawn_role":{"type":"string"},"spawn_depth":{"type":"integer","minimum":1},"ttl_expires_at":{"type":"string","format":"date-time"},"worktree":{"type":"object","required":["id","name","branch","path","created"],"properties":{"id":{"type":"string"},"name":{"type":"string"},"branch":{"type":"string"},"path":{"type":"string"},"base_sha":{"type":"string"},"created":{"type":"boolean"}},"additionalProperties":false},"prompt":{"type":"object","required":["message_id","status"],"properties":{"message_id":{"type":"string"},"status":{"type":"string"},"reply_watch_id":{"type":"string"},"reply_watch_state":{"type":"string"},"error":{"type":"string"}},"additionalProperties":false}},"additionalProperties":false}`
