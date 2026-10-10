package builtin

const sessionPromptOriginSchema = `{
 "type":"object",
 "required":["kind","session_id","workspace_id","hop"],
 "properties":{
 "kind":{"type":"string","enum":["session"]},
 "session_id":{"type":"string"},"workspace_id":{"type":"string"},
 "agent_name":{"type":"string"},"title_at_send":{"type":"string"},
 "hop":{"type":"integer","minimum":1,"maximum":8},
 "notify_on_complete":{"type":"boolean"},"reply_watch_id":{"type":"string"}
 },"additionalProperties":false
}`
