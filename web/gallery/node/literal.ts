import ts from "typescript";

export type Literal = string | boolean | Literal[] | { [key: string]: Literal };

export class LiteralError extends Error {
  constructor(
    readonly reason: "not_literal" | "duplicate_key",
    readonly key?: string,
    readonly path: readonly string[] = []
  ) {
    super(reason);
  }
}

/** Reads data without executing expressions or silently replacing repeated object properties. */
export function readLiteral(node: ts.Node, path: readonly string[] = []): Literal {
  if (ts.isStringLiteral(node) || ts.isNoSubstitutionTemplateLiteral(node)) return node.text;
  if (node.kind === ts.SyntaxKind.TrueKeyword) return true;
  if (node.kind === ts.SyntaxKind.FalseKeyword) return false;
  if (
    ts.isAsExpression(node) ||
    ts.isSatisfiesExpression(node) ||
    ts.isParenthesizedExpression(node)
  ) {
    return readLiteral(node.expression, path);
  }
  if (ts.isArrayLiteralExpression(node)) {
    return node.elements.map((element, index) => readLiteral(element, [...path, String(index)]));
  }
  if (ts.isObjectLiteralExpression(node)) {
    const keys = new Set<string>();
    return Object.fromEntries(
      node.properties.map(property => {
        if (
          !ts.isPropertyAssignment(property) ||
          !(ts.isIdentifier(property.name) || ts.isStringLiteral(property.name))
        ) {
          throw new LiteralError("not_literal", undefined, path);
        }
        const key = property.name.text;
        if (keys.has(key)) throw new LiteralError("duplicate_key", key, path);
        keys.add(key);
        return [key, readLiteral(property.initializer, [...path, key])];
      })
    );
  }
  throw new LiteralError("not_literal", undefined, path);
}
