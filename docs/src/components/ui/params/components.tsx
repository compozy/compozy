import { ChevronRight } from "lucide-react";
import React from "react";
import {
  buildNumericConstraints,
  buildStringConstraints,
  formatValue,
  hasEnumValues,
} from "./helpers";
import { Param } from "./param";
import { JSONSchema } from "./types";

interface SchemaDescriptionProps {
  schema: JSONSchema;
}

/**
 * Renders the description content for a schema
 */
export function SchemaDescription({ schema }: SchemaDescriptionProps) {
  const parts: React.ReactNode[] = [];

  // Add main description
  if (schema.description) {
    parts.push(<p key="desc">{schema.description}</p>);
  }

  // Add enum values
  if (hasEnumValues(schema)) {
    parts.push(
      <div key="enum">
        <p>Allowed values:</p>
        <ul>
          {schema.enum.map((value, index) => (
            <li key={index}>
              <code>{formatValue(value)}</code>
            </li>
          ))}
        </ul>
      </div>
    );
  }

  // Add numeric constraints
  const numericConstraints = buildNumericConstraints(schema);
  if (numericConstraints.length > 0) {
    parts.push(<p key="numeric-constraints">Constraints: {numericConstraints.join(", ")}</p>);
  }

  // Add string constraints
  const stringConstraints = buildStringConstraints(schema);
  if (stringConstraints.length > 0) {
    parts.push(<p key="string-constraints">Length: {stringConstraints.join(", ")}</p>);
  }

  // Add pattern
  if (typeof schema === "object" && schema !== null && schema.pattern) {
    parts.push(
      <p key="pattern">
        Pattern: <code>{schema.pattern}</code>
      </p>
    );
  }

  // Add format
  if (typeof schema === "object" && schema !== null && schema.format) {
    parts.push(
      <p key="format">
        Format: <code>{schema.format}</code>
      </p>
    );
  }

  if (parts.length === 0) return null;

  return <Param.Body>{parts}</Param.Body>;
}

interface ConditionalSchemaProps {
  variants: JSONSchema[];
  path: string;
  rootSchema: JSONSchema;
  required: boolean;
  paramType: "query" | "path" | "body" | "header" | "response";
  getOptionTitle: (variant: JSONSchema, index: number) => string;
  renderSchema: (props: any) => React.ReactNode;
}

/**
 * Renders conditional schemas (anyOf, oneOf, allOf)
 */
export function ConditionalSchema({
  variants,
  path,
  rootSchema,
  required,
  paramType,
  getOptionTitle,
  renderSchema,
}: ConditionalSchemaProps) {
  return (
    <Param.ExpandableRoot>
      {variants.map((variant, index) => {
        const optionTitle = getOptionTitle(variant, index);

        return (
          <Param.ExpandableItem
            key={index}
            value={`option-${index}`}
            title={optionTitle}
            icon={<ChevronRight className="size-3" />}
          >
            {renderSchema({
              path,
              schema: variant,
              rootSchema,
              required,
              paramType,
            })}
          </Param.ExpandableItem>
        );
      })}
    </Param.ExpandableRoot>
  );
}
