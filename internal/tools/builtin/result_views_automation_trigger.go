package builtin

const automationTriggerRecordCompactOutputSchema = `{
 "type":"object",
 "required":["trigger"],
 "properties":{
  "trigger":` + automationTriggerCompactRecordSchema + `
 },
 "additionalProperties":false
}`

const automationTriggersListCompactOutputSchema = `{
 "type":"object",
 "required":["triggers","page"],
 "properties":{
  "triggers":{"type":["array","null"],"items":` + automationTriggerCompactRecordSchema + `},
  "page":` + automationCompactPageSchema + `
 },
 "additionalProperties":false
}`

const automationTriggerCompactRecordSchema = `{
 "type":"object",
 "required":[
  "id",
  "profile_id",
  "profile_name",
  "scope",
  "name",
  "target_kind",
  "agent_name",
  "event",
  "enabled",
  "source",
  "webhook_secret_present"
 ],
 "properties":{
  "id":{"type":"string"},
  "profile_id":{"type":"string"},
  "profile_name":{"type":"string"},
  "scope":{"type":"string"},
  "name":{"type":"string"},
  "target_kind":{"type":"string"},
  "target_instance_scope":{"type":"string"},
  "agent_name":{"type":"string"},
  "workspace_id":{"type":"string"},
  "event":{"type":"string"},
  "enabled":{"type":"boolean"},
  "source":{"type":"string"},
  "webhook_id":{"type":"string"},
  "endpoint_slug":{"type":"string"},
  "webhook_secret_present":{"type":"boolean"},
  "last_run":` + automationCompactLastRunSchema + `,
  "ingress":{
   "type":"object",
   "required":["subject_kind","subject_id","scope_kind","reachability","endpoint_generation"],
   "properties":{
    "subject_kind":{"type":"string"},
    "subject_id":{"type":"string"},
    "scope_kind":{"type":"string"},
    "workspace_id":{"type":"string"},
    "url":{"type":"string"},
    "reachability":{"type":"string"},
    "endpoint_generation":{"type":"integer"},
    "confirmed_endpoint_generation":{"type":"integer"},
    "confirmed_at":{"type":"string"},
    "enable_path":{"type":"string"}
   },
   "additionalProperties":false
  },
  "loop_target":` + automationCompactLoopTargetSchema + `
 },
 "additionalProperties":false
}`
