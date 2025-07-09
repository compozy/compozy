// Constants for param components

export const PARAM_TYPES = {
  STRING: "string",
  NUMBER: "number",
  INTEGER: "integer",
  BOOLEAN: "boolean",
  ARRAY: "array",
  OBJECT: "object",
  NULL: "null",
  MIXED: "mixed",
  ANY: "any",
} as const;

export const PARAM_FORMATS = {
  EMAIL: "email",
  URI: "uri",
  UUID: "uuid",
  DATE_TIME: "date-time",
  DATE: "date",
  TIME: "time",
} as const;

export const CONDITIONAL_TYPES = {
  ANY_OF: "anyOf",
  ONE_OF: "oneOf",
  ALL_OF: "allOf",
} as const;

export const UI_CONSTANTS = {
  DEFAULT_PARAM_TYPE: "body",
  PROPERTIES_TITLE: "Properties",
  ARRAY_ITEMS_TITLE: "Array items",
  OPTION_PREFIX: "Option",
  REF_PREFIX: "#/",
  ACCORDION_VALUE: "schema",
} as const;

export const ICONS = {
  TABLE: "table-properties",
  CHEVRON_RIGHT: "chevron-right",
  FILE_JSON: "file-json",
} as const;
