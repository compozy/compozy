export type IPCMessageType = "Error" | "Log" | "AgentResponse" | "ToolResponse";

export type IPCMessage<T> = {
  type: IPCMessageType;
  payload: T;
};

export enum LogLevel {
  Debug = "Debug",
  Info = "Info",
  Warn = "Warn",
  Error = "Error",
}

export type LogContext = Record<string, any>;

export type LogMessage = {
  level: LogLevel;
  message: string;
  context?: LogContext;
  timestamp: string;
};

export type Input = Record<string, any>;
export type Output = Record<string, any>;

export type ProviderConfig = {
  api_key: string;
  provider: string;
  model: string;
};

export type ActionRequest = {
  id: string;
  prompt: string;
  output_schema?: string | null;
};

export type AgentRequest = {
  type: "AgentRequest";
  payload: {
    id: string;
    agent_id: string;
    instructions: string;
    config: ProviderConfig;
    action: ActionRequest;
    tools: ToolRequest[];
  };
};

export type ErrorMessage = {
  request_id: string | null;
  message: string;
  stack: any;
  data: Input;
};

export type AgentResponse<T> = {
  id: string;
  agent_id: string;
  output: T;
  status: "Success" | "Error";
};

export type ToolRequest = {
  id: string;
  tool_id: string;
  description: string;
  input_schema?: string | null;
  output_schema?: string | null;
  input?: Record<string, any> | null;
};

export type ToolResponse<T = unknown> = {
  id: string;
  tool_id: string;
  output: T;
  status: "Success" | "Error";
};

export type RequestType = "agent" | "tool";
