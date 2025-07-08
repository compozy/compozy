import { PARAM_TYPES, UI_CONSTANTS } from "./constants";
import { JSONSchema, hasConditionals, isArraySchema, isObjectSchema, isRefSchema } from "./types";

/**
 * Determines the type of a JSON Schema
 */
export function getSchemaType(schema: JSONSchema): string {
  if (schema.type) return schema.type;
  if (isRefSchema(schema)) return PARAM_TYPES.OBJECT;
  if (isObjectSchema(schema)) return PARAM_TYPES.OBJECT;
  if (isArraySchema(schema)) return PARAM_TYPES.ARRAY;
  if (schema.enum) return PARAM_TYPES.STRING;
  if (hasConditionals(schema)) return PARAM_TYPES.MIXED;
  return PARAM_TYPES.ANY;
}

/**
 * Resolves a $ref reference in a JSON Schema
 */
export function resolveRef(ref: string, rootSchema: JSONSchema): JSONSchema | null {
  // Handle internal references (#/...)
  if (ref.startsWith(UI_CONSTANTS.REF_PREFIX)) {
    const path = ref.slice(UI_CONSTANTS.REF_PREFIX.length).split("/");
    let current: any = rootSchema;

    for (const segment of path) {
      current = current?.[segment];
      if (!current) return null;
    }

    return current as JSONSchema;
  }

  // Handle external references (e.g., tool.json, mcp.json)
  // For external refs, we need to load the schema dynamically
  // This will be handled by the component that uses this function
  return null;
}

/**
 * Gets the default value from a schema as a string
 */
export function getDefaultValue(schema: JSONSchema): string | undefined {
  if (schema.default === undefined) return undefined;

  if (typeof schema.default === "string") {
    return schema.default;
  }

  return JSON.stringify(schema.default);
}

/**
 * Builds constraints description for numeric types
 */
export function buildNumericConstraints(schema: JSONSchema): string[] {
  const constraints: string[] = [];

  // Only check for numeric constraints if schema is an object
  if (typeof schema !== "object" || schema === null) {
    return constraints;
  }

  if (schema.minimum !== undefined) {
    constraints.push(`minimum: ${schema.minimum}`);
  }
  if (schema.maximum !== undefined) {
    constraints.push(`maximum: ${schema.maximum}`);
  }
  if (schema.exclusiveMinimum !== undefined) {
    constraints.push(`exclusive minimum: ${schema.exclusiveMinimum}`);
  }
  if (schema.exclusiveMaximum !== undefined) {
    constraints.push(`exclusive maximum: ${schema.exclusiveMaximum}`);
  }
  if (schema.multipleOf !== undefined) {
    constraints.push(`multiple of: ${schema.multipleOf}`);
  }

  return constraints;
}

/**
 * Builds constraints description for string types
 */
export function buildStringConstraints(schema: JSONSchema): string[] {
  const constraints: string[] = [];

  // Only check for string constraints if schema is an object
  if (typeof schema !== "object" || schema === null) {
    return constraints;
  }

  if (schema.minLength !== undefined) {
    constraints.push(`minLength: ${schema.minLength}`);
  }
  if (schema.maxLength !== undefined) {
    constraints.push(`maxLength: ${schema.maxLength}`);
  }

  return constraints;
}

/**
 * Gets a better title for conditional schema options
 */
export function getOptionTitle(variant: JSONSchema, index: number): string {
  // If the variant has a title, use it
  if (variant.title) return variant.title;

  // If it's an object with a type constant property, use that (most specific)
  if (isObjectSchema(variant) && variant.properties?.type) {
    const typeSchema = variant.properties.type;
    if (typeSchema && "const" in typeSchema && typeSchema.const) {
      return String(typeSchema.const);
    }
  }

  // If it's an object with discriminator properties, try to find a meaningful name
  if (isObjectSchema(variant) && variant.properties) {
    const props = Object.keys(variant.properties);

    // Look for common discriminator properties
    const discriminators = ["kind", "type", "variant", "method", "format", "mode"];
    for (const disc of discriminators) {
      if (props.includes(disc)) {
        const discSchema = variant.properties[disc];
        if (discSchema && "const" in discSchema && discSchema.const) {
          return String(discSchema.const);
        }
        if (discSchema && "enum" in discSchema && discSchema.enum && discSchema.enum.length === 1) {
          return String(discSchema.enum[0]);
        }
      }
    }

    // Look for a name or title property
    if (props.includes("name")) {
      const nameSchema = variant.properties.name;
      if (nameSchema && "const" in nameSchema && nameSchema.const) {
        return String(nameSchema.const);
      }
    }

    // If object has few properties, show the main ones
    if (props.length <= 3) {
      const mainProps = props.filter(p => !["id", "createdAt", "updatedAt", "version"].includes(p));
      if (mainProps.length > 0) {
        return `{${mainProps.join(", ")}}`;
      }
    }
  }

  // If it's an array, describe the items
  if (isArraySchema(variant) && variant.items) {
    const itemType = getSchemaType(variant.items);
    return `Array<${itemType}>`;
  }

  // If it has a specific type, use that with more context
  if (variant.type) {
    if (variant.type === "object" && variant.description) {
      // Try to extract a meaningful name from description
      const desc = variant.description.toLowerCase();
      if (desc.includes("payment")) return "Payment";
      if (desc.includes("user")) return "User";
      if (desc.includes("product")) return "Product";
      if (desc.includes("order")) return "Order";
      if (desc.includes("config")) return "Configuration";
      if (desc.includes("setting")) return "Settings";
      if (desc.includes("response")) return "Response";
      if (desc.includes("request")) return "Request";
      if (desc.includes("error")) return "Error";
    }
    return variant.type;
  }

  // If it has enum values, show first few
  if (variant.enum && variant.enum.length > 0) {
    if (variant.enum.length === 1) {
      return String(variant.enum[0]);
    }
    if (variant.enum.length <= 3) {
      return variant.enum.map(String).join(" | ");
    }
    return `${variant.enum.slice(0, 2).map(String).join(" | ")}...`;
  }

  // If it has a description, try to extract meaning
  if (variant.description) {
    const desc = variant.description;
    // Look for patterns like "Card payment", "Bank transfer", etc.
    const match = desc.match(/^([A-Z][a-z]+(?:\s+[a-z]+)*)/);
    if (match) {
      return match[1];
    }
  }

  // Default to generic option title
  return `${UI_CONSTANTS.OPTION_PREFIX} ${index + 1}`;
}

/**
 * Extracts required fields from an object schema
 */
export function getRequiredFields(schema: JSONSchema): string[] {
  if (isObjectSchema(schema) && schema.required) {
    return schema.required;
  }
  return [];
}

/**
 * Gets properties from an object schema
 */
export function getSchemaProperties(schema: JSONSchema): [string, JSONSchema][] {
  if (isObjectSchema(schema) && schema.properties) {
    return Object.entries(schema.properties);
  }
  return [];
}

/**
 * Checks if schema has enum values
 */
export function hasEnumValues(schema: JSONSchema): schema is JSONSchema & { enum: any[] } {
  return Array.isArray(schema.enum) && schema.enum.length > 0;
}

/**
 * Formats a value for display (handles different types)
 */
export function formatValue(value: any): string {
  if (typeof value === "string") return value;
  return JSON.stringify(value);
}
