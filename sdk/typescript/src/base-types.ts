export type JSONPrimitive = string | number | boolean | null;
export type JSONValue = JSONPrimitive | JSONValue[] | { [key: string]: JSONValue };
export type ISODateTime = string;
export type ProtocolVersion = "1";
export type JSONRPCID = string | number;

export interface JSONRPCRequestEnvelope<Params = unknown> {
  jsonrpc: "2.0";
  id?: JSONRPCID;
  method: string;
  params?: Params;
}

export interface JSONRPCResponseEnvelope<Result = unknown, Data = unknown> {
  jsonrpc: "2.0";
  id: JSONRPCID;
  result?: Result;
  error?: JSONRPCErrorObject<Data>;
}

export interface JSONRPCErrorObject<Data = unknown> {
  code: number;
  message: string;
  data?: Data;
}
