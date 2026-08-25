/**
 * Adapter facade. The query layer and the barrel import this one module while
 * the implementation stays split by concern (calls / messages / counts / drain).
 */
export {
  AgentCommsApiError,
  AGENT_COMMS_ERROR_CODES,
  agentCommsErrorCode,
  isAgentCommsApiError,
  isAgentCommsErrorCode,
  type AgentCommsErrorCode,
} from "./agent-comms-api-error";

export {
  cancelCall,
  createCall,
  fetchCall,
  fetchCallPrompt,
  fetchCallResult,
  fetchCallSuperseded,
  listCalls,
  type CallsListFilter,
} from "./agent-comms-calls-api";

export { countCalls, type CallCount, type CallCountFilter } from "./agent-comms-summary-api";

export { drainCallSubtree } from "./agent-comms-drain-api";

export {
  listCallMessages,
  sendCallMessage,
  type CallMessagesFilter,
} from "./agent-comms-messages-api";
