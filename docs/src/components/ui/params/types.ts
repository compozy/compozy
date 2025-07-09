// JSON Schema TypeScript definitions for type safety

export type JSONSchemaType =
  | "string"
  | "number"
  | "integer"
  | "boolean"
  | "array"
  | "object"
  | "null";

export interface JSONSchemaBase {
  $id?: string;
  $ref?: string;
  $schema?: string;
  $comment?: string;
  title?: string;
  description?: string;
  default?: any;
  examples?: any[];
  deprecated?: boolean;
  readOnly?: boolean;
  writeOnly?: boolean;
  enum?: any[];
  const?: any;
  format?: string;
  type?: string;
  // Object-specific properties (for flexibility)
  properties?: Record<string, any>;
  required?: string[];
  additionalProperties?: boolean | any;
  patternProperties?: Record<string, any>;
  minProperties?: number;
  maxProperties?: number;
  // Array-specific properties (for flexibility)
  items?: any;
  minItems?: number;
  maxItems?: number;
  uniqueItems?: boolean;
  additionalItems?: boolean | any;
  // String-specific properties (for flexibility)
  minLength?: number;
  maxLength?: number;
  pattern?: string;
  // Number-specific properties (for flexibility)
  minimum?: number;
  maximum?: number;
  exclusiveMinimum?: number;
  exclusiveMaximum?: number;
  multipleOf?: number;
  // Conditional properties (for flexibility)
  if?: any;
  then?: any;
  else?: any;
  allOf?: any[];
  anyOf?: any[];
  oneOf?: any[];
  not?: any;
}

export interface JSONSchemaString extends JSONSchemaBase {
  type: "string";
}

export interface JSONSchemaNumber extends JSONSchemaBase {
  type: "number" | "integer";
}

export interface JSONSchemaBoolean extends JSONSchemaBase {
  type: "boolean";
}

export interface JSONSchemaArray extends JSONSchemaBase {
  type: "array";
}

export interface JSONSchemaObject extends JSONSchemaBase {
  type: "object";
}

export interface JSONSchemaConditional extends JSONSchemaBase {
  // No specific type required for conditional schemas
}

export type JSONSchema = JSONSchemaBase;

// Utility type guards
export const isObjectSchema = (schema: JSONSchema): schema is JSONSchemaObject => {
  return schema.type === "object" || (!!schema.properties && !schema.type);
};

export const isArraySchema = (schema: JSONSchema): schema is JSONSchemaArray => {
  return schema.type === "array" || (!!schema.items && !schema.type);
};

export const hasConditionals = (schema: JSONSchema): schema is JSONSchemaConditional => {
  return !!(schema.anyOf || schema.oneOf || schema.allOf);
};

export const isRefSchema = (schema: JSONSchema): schema is JSONSchema & { $ref: string } => {
  return !!schema.$ref;
};
