import { default as FumaLink } from "fumadocs-core/link";
import { ChevronRight, Link } from "lucide-react";
import React from "react";
import { Markdown } from "../markdown";
import { ParamCollapse, ParamCollapseItem } from "./param";
import { JSONSchema } from "./types";

interface ParameterDescriptionProps {
  description?: string;
}

/**
 * Simple parameter description component for individual schema parameters
 */
export function ParameterDescription({ description }: ParameterDescriptionProps) {
  if (!description) {
    return null;
  }

  const parts: React.ReactNode[] = [];

  // Check for different types of schema references in the description
  const hasExternalRef = description.includes("$ref: schema://");
  const hasInlineRef = description.includes("$ref: inline:#");

  const externalRefMatch = hasExternalRef ? description.match(/\$ref: schema:\/\/(\w+)/) : null;
  const inlineRefMatch = hasInlineRef ? description.match(/\$ref: inline:#([\w-]+)/) : null;

  const referencedSchema = externalRefMatch ? externalRefMatch[1] : null;
  const inlineReference = inlineRefMatch ? inlineRefMatch[1] : null;

  // Get main description without $ref references
  let mainDescription = description;
  if (hasExternalRef) {
    mainDescription = mainDescription.replace(/\$ref: schema:\/\/\w+/g, "").trim();
  }
  if (hasInlineRef) {
    mainDescription = mainDescription.replace(/\$ref: inline:#[\w-]+/g, "").trim();
  }
  mainDescription = mainDescription.trim();

  // Add main description
  if (mainDescription) {
    parts.push(<Markdown key="desc">{mainDescription}</Markdown>);
  }

  // Add external schema reference link if present
  if (referencedSchema) {
    parts.push(
      <div key="external-ref" className="flex items-center gap-1 mt-3">
        <Link className="size-3" /> <strong>Schema Reference:</strong>{" "}
        <FumaLink
          href={`/docs/schema/${referencedSchema}`}
          className="underline hover:no-underline"
        >
          {referencedSchema}.json
        </FumaLink>
      </div>
    );
  }

  // Add inline schema reference link if present
  if (inlineReference) {
    parts.push(
      <div key="inline-ref" className="flex items-center gap-1 mt-3">
        <Link className="size-3" /> <strong>See also:</strong>{" "}
        <a href={`#${inlineReference}`} className="underline hover:no-underline">
          {inlineReference.replace(/-/g, " ").replace(/\b\w/g, l => l.toUpperCase())}
        </a>
      </div>
    );
  }

  if (parts.length === 0) {
    return null;
  }

  return <>{parts}</>;
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
    <ParamCollapse>
      {variants.map((variant, index) => {
        const optionTitle = getOptionTitle(variant, index);

        return (
          <ParamCollapseItem
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
          </ParamCollapseItem>
        );
      })}
    </ParamCollapse>
  );
}
