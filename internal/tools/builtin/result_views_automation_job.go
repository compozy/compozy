package builtin

const automationJobRecordCompactOutputSchema = `{
 "type":"object",
 "required":["job"],
 "properties":{
  "job":` + automationJobCompactRecordSchema + `
 },
 "additionalProperties":false
}`

const automationJobsListCompactOutputSchema = `{
 "type":"object",
 "required":["jobs","page"],
 "properties":{
  "jobs":{"type":["array","null"],"items":` + automationJobCompactRecordSchema + `},
  "page":` + automationCompactPageSchema + `
 },
 "additionalProperties":false
}`

const automationJobCompactRecordSchema = `{
 "type":"object",
 "required":["id","profile_id","profile_name","scope","name","target_kind","agent_name","enabled","source"],
 "properties":{
  "id":{"type":"string"},
  "profile_id":{"type":"string"},
  "profile_name":{"type":"string"},
  "scope":{"type":"string"},
  "name":{"type":"string"},
  "target_kind":{"type":"string"},
  "agent_name":{"type":"string"},
  "workspace_id":{"type":"string"},
  "schedule":{
   "type":"object",
   "required":["mode"],
   "properties":{
    "mode":{"type":"string"},
    "expr":{"type":"string"},
    "interval":{"type":"string"},
    "time":{"type":"string"},
    "catch_up_policy":{"type":"string"},
    "misfire_grace_seconds":{"type":"integer"}
   },
   "additionalProperties":false
  },
  "enabled":{"type":"boolean"},
  "source":{"type":"string"},
  "next_run":{"type":"string"},
  "last_run":` + automationCompactLastRunSchema + `,
  "loop_target":` + automationCompactLoopTargetSchema + `,
  "task":{
   "type":"object",
   "required":[],
   "properties":{
    "title":{"type":"string"}
   },
   "additionalProperties":false
  },
  "scheduler":{
   "type":"object",
   "required":["registered"],
   "properties":{
    "registered":{"type":"boolean"},
    "consecutive_resume_failures":{"type":"integer"},
    "misfire_count":{"type":"integer"},
    "last_misfire_at":{"type":"string"}
   },
   "additionalProperties":false
  }
 },
 "additionalProperties":false
}`

const automationCompactLastRunSchema = `{
 "type":"object",
 "required":["id","status"],
 "properties":{
  "id":{"type":"string"},
  "status":{"type":"string"},
  "started_at":{"type":"string"},
  "ended_at":{"type":"string"},
  "skip_reason":{"type":"string"}
 },
 "additionalProperties":false
}`

const automationCompactLoopTargetSchema = `{
 "type":"object",
 "required":["workspace_id","loop_name"],
 "properties":{
  "workspace_id":{"type":"string"},
  "loop_name":{"type":"string"}
 },
 "additionalProperties":false
}`

const automationCompactPageSchema = `{
 "type":"object",
 "required":["has_more","total","limit"],
 "properties":{
  "next_cursor":{"type":"string"},
  "has_more":{"type":"boolean"},
  "total":{"type":"integer"},
  "limit":{"type":"integer"}
 },
 "additionalProperties":false
}`
