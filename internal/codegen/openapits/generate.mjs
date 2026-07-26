import { writeFile } from "node:fs/promises";
import { resolve } from "node:path";
import { pathToFileURL } from "node:url";

import openapiTS, {
  astToString,
  COMMENT_HEADER,
  transformSchemaObject,
  tsUnion,
} from "openapi-typescript";
import ts from "typescript";

const widenAdditionalPropertiesExtension = "x-agh-typescript-widen-additional-properties";

const runtimeArgs = process.argv.slice(1);
const args = runtimeArgs[0]?.endsWith("generate.mjs") ? runtimeArgs.slice(1) : runtimeArgs;
const input = args[0];
const outputFlag = args.indexOf("-o");
const output = outputFlag >= 0 ? args[outputFlag + 1] : undefined;

if (!input || !output) {
  throw new Error("usage: generate.mjs <openapi-path> -o <output-path>");
}

const ast = await openapiTS(pathToFileURL(resolve(input)), {
  transform(schemaObject, options) {
    if (schemaObject[widenAdditionalPropertiesExtension] !== true) {
      return undefined;
    }

    const baseType = transformSchemaObject(schemaObject, {
      ...options,
      schema: schemaObject,
      ctx: { ...options.ctx, transform: undefined },
    });
    return widenAdditionalPropertyIndexes(baseType);
  },
});

await writeFile(resolve(output), `${COMMENT_HEADER}${astToString(ast)}`, "utf8");

function widenAdditionalPropertyIndexes(type) {
  if (ts.isUnionTypeNode(type)) {
    return ts.factory.updateUnionTypeNode(type, type.types.map(widenAdditionalPropertyIndexes));
  }
  if (ts.isParenthesizedTypeNode(type)) {
    return ts.factory.updateParenthesizedType(type, widenAdditionalPropertyIndexes(type.type));
  }
  if (!ts.isIntersectionTypeNode(type)) {
    return type;
  }

  const propertyTypes = [];
  let hasOptionalProperty = false;
  for (const memberType of type.types) {
    if (!ts.isTypeLiteralNode(memberType)) {
      continue;
    }
    for (const member of memberType.members) {
      if (ts.isPropertySignature(member) && member.type) {
        propertyTypes.push(member.type);
        hasOptionalProperty ||= member.questionToken !== undefined;
      }
    }
  }
  if (propertyTypes.length === 0) {
    return type;
  }

  let changed = false;
  const widenedMembers = type.types.map(memberType => {
    if (!ts.isTypeLiteralNode(memberType)) {
      return memberType;
    }
    const members = memberType.members.map(member => {
      if (!ts.isIndexSignatureDeclaration(member) || !member.type) {
        return member;
      }
      changed = true;
      const indexTypes = [member.type, ...propertyTypes];
      // Optional request properties may be present as undefined before JSON serialization.
      if (hasOptionalProperty) {
        indexTypes.push(ts.factory.createKeywordTypeNode(ts.SyntaxKind.UndefinedKeyword));
      }
      return ts.factory.updateIndexSignature(
        member,
        member.modifiers,
        member.parameters,
        tsUnion(indexTypes)
      );
    });
    return ts.factory.updateTypeLiteralNode(memberType, members);
  });

  return changed ? ts.factory.updateIntersectionTypeNode(type, widenedMembers) : type;
}
