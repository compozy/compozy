"use client";

import { FileJson } from "lucide-react";
import { tv } from "tailwind-variants";
import { ConditionalSchema, SchemaDescription } from "./components";
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

const schemaParamsVariants = tv({
  slots: {
    wrapper: "flex flex-col",
  },
});

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
  const resolvedSchema = isRefSchema(schema) ? resolveRef(schema.$ref, rootSchema) : schema;
  if (!resolvedSchema) return null;
  const type = getSchemaType(resolvedSchema);
  const defaultValue = getDefaultValue(resolvedSchema);

  // Handle object type with properties
  if (isObjectSchema(resolvedSchema) && resolvedSchema.properties) {
    const requiredFields = getRequiredFields(resolvedSchema);
    const properties = getSchemaProperties(resolvedSchema);

    return (
      <Param
        path={path}
        type={type}
        required={required}
        default={defaultValue}
        paramType={paramType}
      >
        <SchemaDescription schema={resolvedSchema} />
        {properties.length > 0 && (
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
  if (isArraySchema(resolvedSchema) && resolvedSchema.items) {
    // Handle single item schema (most common case)
    const itemSchema = Array.isArray(resolvedSchema.items)
      ? resolvedSchema.items[0]
      : resolvedSchema.items;

    return (
      <Param
        path={path}
        type={type}
        required={required}
        default={defaultValue}
        paramType={paramType}
      >
        <SchemaDescription schema={resolvedSchema} />
        <Param.ExpandableRoot>
          <Param.ExpandableItem value="items" title={UI_CONSTANTS.ARRAY_ITEMS_TITLE}>
            <SchemaParam
              path={`${path}[0]`}
              schema={itemSchema}
              rootSchema={rootSchema}
              paramType={paramType}
            />
          </Param.ExpandableItem>
        </Param.ExpandableRoot>
      </Param>
    );
  }

  // Handle anyOf/oneOf/allOf
  if (hasConditionals(resolvedSchema)) {
    const variants = resolvedSchema.anyOf || resolvedSchema.oneOf || resolvedSchema.allOf || [];
    return (
      <Param
        path={path}
        type={type}
        required={required}
        default={defaultValue}
        paramType={paramType}
      >
        <SchemaDescription schema={resolvedSchema} />
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
      <SchemaDescription schema={resolvedSchema} />
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
        <Params.Header>
          <FileJson className="size-4" />
          <span className="font-medium">{title}</span>
        </Params.Header>
        <Params.Body>{renderContent()}</Params.Body>
      </Params>
    );
  }

  // Otherwise render without wrapper
  return <div className={styles.wrapper({ className })}>{renderContent()}</div>;
}
