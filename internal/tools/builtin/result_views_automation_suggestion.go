package builtin

const automationSuggestionsListCompactOutputSchema = `{
 "type":"object",
 "required":["suggestions"],
 "properties":{
  "suggestions":{"type":["array","null"],"items":` + automationSuggestionCompactRecordSchema + `}
 },
 "additionalProperties":false
}`

const automationSuggestionCompactOutputSchema = `{
 "type":"object",
 "required":["suggestion"],
 "properties":{
  "suggestion":` + automationSuggestionCompactRecordSchema + `
 },
 "additionalProperties":false
}`

const automationSuggestionAcceptCompactOutputSchema = `{
 "type":"object",
 "required":["suggestion","job"],
 "properties":{
  "suggestion":` + automationSuggestionCompactRecordSchema + `,
  "job":` + automationJobCompactRecordSchema + `
 },
 "additionalProperties":false
}`

const automationSuggestionCompactRecordSchema = `{
 "type":"object",
 "required":["id","profile_id","workspace_id","source","dedup_key","status","payload","created_at"],
 "properties":{
  "id":{"type":"string"},
  "profile_id":{"type":"string"},
  "workspace_id":{"type":"string"},
  "source":{"type":"string"},
  "dedup_key":{"type":"string"},
  "status":{"type":"string"},
  "payload":` + automationJobCompactRecordSchema + `,
  "created_at":{"type":"string"},
  "resolved_at":{"type":"string"}
 },
 "additionalProperties":false
}`
