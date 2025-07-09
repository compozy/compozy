"use client";

import { FileJson } from "lucide-react";
import { tv } from "tailwind-variants";
import { ConditionalSchema, ParameterDescription } from "./components";
import { UI_CONSTANTS } from "./constants";
import {
  getDefaultValue,
  getOptionTitle,
  getRequiredFields,
  getSchemaProperties,
  getSchemaType,
  resolveRef,
} from "./helpers";
import { Param } from "./param";
import { Params } from "./params";
import { JSONSchema, hasConditionals, isArraySchema, isObjectSchema, isRefSchema } from "./types";

// Pre-import all schemas that might be referenced
import actionConfigSchema from "@/schemas/action-config.json";
import agentSchema from "@/schemas/agent.json";
import autoloadSchema from "@/schemas/autoload.json";
import cacheSchema from "@/schemas/cache.json";
import mcpSchema from "@/schemas/mcp.json";
import memorySchema from "@/schemas/memory.json";
import monitoringSchema from "@/schemas/monitoring.json";
import providerSchema from "@/schemas/provider.json";
import taskSchema from "@/schemas/task.json";
import toolSchema from "@/schemas/tool.json";
import workflowSchema from "@/schemas/workflow.json";

const schemaParamsVariants = tv({
  slots: {
    wrapper: "flex flex-col",
  },
});

// Map of external schema references to their imported schemas
const externalSchemas: Record<string, JSONSchema> = {
  "tool.json": toolSchema as JSONSchema,
  "mcp.json": mcpSchema as JSONSchema,
  "action-config.json": actionConfigSchema as JSONSchema,
  "agent.json": agentSchema as JSONSchema,
  "task.json": taskSchema as JSONSchema,
  "workflow.json": workflowSchema as JSONSchema,
  "memory.json": memorySchema as JSONSchema,
  "provider.json": providerSchema as JSONSchema,
  "cache.json": cacheSchema as JSONSchema,
  "autoload.json": autoloadSchema as JSONSchema,
  "monitoring.json": monitoringSchema as JSONSchema,
};

interface SchemaParamProps {
  path: string;
  schema: JSONSchema;
  rootSchema: JSONSchema;
  required?: boolean;
  paramType?: "query" | "path" | "body" | "header" | "response";
}

// Component to render a single schema parameter
function SchemaParam({
  path,
  schema,
  rootSchema,
  required = false,
  paramType = UI_CONSTANTS.DEFAULT_PARAM_TYPE,
}: SchemaParamProps) {
  // Resolve $ref if present
  let resolvedSchema: JSONSchema | null = null;

  if (isRefSchema(schema)) {
    // Try internal reference first
    resolvedSchema = resolveRef(schema.$ref, rootSchema);

    // If not found internally, try external schemas
    if (!resolvedSchema && externalSchemas[schema.$ref]) {
      resolvedSchema = externalSchemas[schema.$ref];
    }
  } else {
    resolvedSchema = schema;
  }

  if (!resolvedSchema) return null;

  // Merge the original schema's description with the resolved schema
  let schemaWithDescription: JSONSchema;
  if (isRefSchema(schema) && schema.description) {
    schemaWithDescription = { ...resolvedSchema, description: schema.description };
  } else {
    schemaWithDescription = resolvedSchema;
  }

  const type = getSchemaType(schemaWithDescription);
  const defaultValue = getDefaultValue(schemaWithDescription);
  const description = schemaWithDescription?.description;

  // Handle object type with properties
  if (isObjectSchema(schemaWithDescription) && schemaWithDescription.properties) {
    const requiredFields = getRequiredFields(schemaWithDescription);
    const properties = getSchemaProperties(schemaWithDescription);
    // Check if the description contains an external schema reference
    const hasExternalSchemaRef = description && description.includes("$ref:");

    return (
      <Param
        path={path}
        type={type}
        required={required}
        default={defaultValue}
        paramType={paramType}
      >
        <ParameterDescription description={description} />
        {properties.length > 0 && !hasExternalSchemaRef && (
          <Param.ExpandableRoot defaultValue={path === "" ? "properties" : undefined}>
            <Param.ExpandableItem value="properties" title={UI_CONSTANTS.PROPERTIES_TITLE}>
              {properties.map(([key, propSchema]) => (
                <SchemaParam
                  key={key}
                  path={path ? `${path}.${key}` : key}
                  schema={propSchema}
                  rootSchema={rootSchema}
                  required={requiredFields.includes(key)}
                  paramType={paramType}
                />
              ))}
            </Param.ExpandableItem>
          </Param.ExpandableRoot>
        )}
      </Param>
    );
  }

  // Handle array type with items
  if (isArraySchema(schemaWithDescription) && schemaWithDescription.items) {
    // Handle single item schema (most common case)
    const itemSchema = Array.isArray(schemaWithDescription.items)
      ? schemaWithDescription.items[0]
      : schemaWithDescription.items;

    // If the item is an external reference, we need to use the resolved schema as root
    let itemRootSchema = rootSchema;
    if (isRefSchema(itemSchema) && externalSchemas[itemSchema.$ref]) {
      itemRootSchema = externalSchemas[itemSchema.$ref];
    }

    return (
      <Param
        path={path}
        type={type}
        required={required}
        default={defaultValue}
        paramType={paramType}
      >
        <ParameterDescription description={description} />
        <Param.ExpandableRoot>
          <Param.ExpandableItem value="items" title={UI_CONSTANTS.ARRAY_ITEMS_TITLE}>
            <SchemaParam
              path={`${path}[0]`}
              schema={itemSchema}
              rootSchema={itemRootSchema}
              paramType={paramType}
            />
          </Param.ExpandableItem>
        </Param.ExpandableRoot>
      </Param>
    );
  }

  // Handle anyOf/oneOf/allOf
  if (hasConditionals(schemaWithDescription)) {
    const variants =
      schemaWithDescription.anyOf ||
      schemaWithDescription.oneOf ||
      schemaWithDescription.allOf ||
      [];
    return (
      <Param
        path={path}
        type={type}
        required={required}
        default={defaultValue}
        paramType={paramType}
      >
        <ParameterDescription description={description} />
        <ConditionalSchema
          variants={variants}
          path={path}
          rootSchema={rootSchema}
          required={required}
          paramType={paramType}
          getOptionTitle={getOptionTitle}
          renderSchema={(props: SchemaParamProps) => <SchemaParam {...props} />}
        />
      </Param>
    );
  }

  // Simple types
  return (
    <Param path={path} type={type} required={required} default={defaultValue} paramType={paramType}>
      <ParameterDescription description={description} />
    </Param>
  );
}

export interface SchemaParamsProps {
  /** JSON Schema object */
  schema: JSONSchema;
  /** Root path for the schema (optional) */
  rootPath?: string;
  /** Parameter type for styling */
  paramType?: "query" | "path" | "body" | "header" | "response";
  /** Additional CSS classes */
  className?: string;
  /** Title for the schema (used in accordion) */
  title?: string;
  /** Whether to wrap in an accordion (default: true for root schemas with title) */
  collapsible?: boolean;
  /** Whether the accordion is expanded by default */
  defaultExpanded?: boolean;
}

/**
 * SchemaParams component renders a JSON Schema as visual parameters using the Param component
 */
export function SchemaParams({
  schema,
  rootPath = "",
  paramType = UI_CONSTANTS.DEFAULT_PARAM_TYPE,
  className,
  title,
  collapsible,
  defaultExpanded = false,
}: SchemaParamsProps) {
  if (!schema) return null;

  // Handle root $ref
  const rootSchema = isRefSchema(schema) ? resolveRef(schema.$ref, schema) : schema;
  const finalSchema = rootSchema || schema;
  const shouldBeCollapsible = collapsible !== undefined ? collapsible : !!title;
  const renderContent = () => {
    // If the root is an object with properties and no rootPath specified, render properties directly
    if (isObjectSchema(finalSchema) && finalSchema.properties && !rootPath) {
      const requiredFields = getRequiredFields(finalSchema);
      const properties = getSchemaProperties(finalSchema);
      return (
        <>
          {properties.map(([key, propSchema]) => (
            <SchemaParam
              key={key}
              path={key}
              schema={propSchema}
              rootSchema={schema}
              required={requiredFields.includes(key)}
              paramType={paramType}
            />
          ))}
        </>
      );
    }

    // Otherwise render normally (for non-object roots or when rootPath is specified)
    return (
      <SchemaParam path={rootPath} schema={finalSchema} rootSchema={schema} paramType={paramType} />
    );
  };

  const styles = schemaParamsVariants();

  // If collapsible and has title, wrap in Params component
  if (shouldBeCollapsible && title) {
    return (
      <Params className={className} collapsible={true} defaultOpen={defaultExpanded}>
        <Params.Header className="py-4">
          <div className="flex flex-col gap-1">
            <div className="flex items-center gap-2">
              <FileJson className="size-4" />
              <span className="font-medium">{title}</span>
            </div>
            {schema.description && (
              <div className="text-sm text-muted-foreground">{schema.description}</div>
            )}
          </div>
        </Params.Header>
        <Params.Body>{renderContent()}</Params.Body>
      </Params>
    );
  }

  // Otherwise render without wrapper
  return <div className={styles.wrapper({ className })}>{renderContent()}</div>;
}
