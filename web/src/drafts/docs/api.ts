/** A real snapshot of the Sessions resource from the OpenAPI document and the SDK samples. */
import raw from "./api.json";

export interface ApiField {
  name: string;
  type: string;
  required: boolean;
  description: string;
  in?: string;
}

export interface ApiOperation {
  id: string;
  method: string;
  path: string;
  summary: string;
  transports?: string[];
  params?: ApiField[];
  body?: ApiField[];
  bodyType?: string;
  responses?: { code: string; description: string }[];
  response?: string;
  samples?: { curl: string; typescript: string; go: string };
}

export const OPERATIONS = (raw as unknown as { operations: ApiOperation[] }).operations;
/** The operations the snapshot carries in full. */
export const DETAILED = OPERATIONS.filter(operation => operation.samples !== undefined);

export const METHOD_TONE: Record<string, string> = {
  GET: "text-brand-strong",
  POST: "text-success",
  PUT: "text-warning",
  PATCH: "text-warning",
  DELETE: "text-danger",
};
