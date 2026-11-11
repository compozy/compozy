package builtin

const agentCompactRecordSchema = `{
 "type":"object",
 "required":["name","label","origin","definition_digest"],
 "properties":{
  "name":{"type":"string"},
  "label":{"type":"string"},
  "description":{"type":"string"},
  "kind":{"type":"string"},
  "category_path":{"type":["array","null"],"items":{"type":"string"}},
  "origin":{"type":"string"},
  "workspace_id":{"type":"string"},
  "layer":{"type":"string"},
  "package_owned":{"type":"boolean"},
  "diagnostics":{"type":["array","null"],"items":{
    "type":"object",
    "required":["path","error_kind","message"],
    "properties":{
     "path":{"type":"string"},
     "error_kind":{"type":"string"},
     "message":{"type":"string"}
    },
    "additionalProperties":false
  }},
  "definition_digest":{"type":"string"},
  "effective_runtime":{
   "type":"object",
   "required":["provider"],
   "properties":{
    "provider":{"type":"string"},
    "model":{"type":"string"},
    "reasoning_effort":{"type":"string"},
    "speed":{"type":"string"},
    "permissions":{"type":"string"}
   },
   "additionalProperties":false
  }
 },
 "additionalProperties":false
}`

const agentRecordCompactOutputSchema = `{
 "type":"object",
 "required":["agent"],
 "properties":{
  "agent":` + agentCompactRecordSchema + `
 },
 "additionalProperties":false
}`

const agentListCompactOutputSchema = `{
 "type":"object",
 "required":["agents"],
 "properties":{
  "agents":{"type":["array","null"],"items":` + agentCompactRecordSchema + `}
 },
 "additionalProperties":false
}`
