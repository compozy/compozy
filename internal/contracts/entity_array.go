package contracts

import "context"

func walkArrayEntities(
	ctx context.Context,
	schema map[string]any,
	items []any,
	path string,
	catalog EntityCatalog,
	issues *[]ValidationIssue,
) error {
	if itemSchema, ok := schemaObject(schema[schemaItems]); ok {
		for index, item := range items {
			if err := walkEntities(
				ctx,
				itemSchema,
				item,
				indexedPath(path, index),
				catalog,
				issues,
			); err != nil {
				return err
			}
		}
	}
	if err := walkContainedEntities(ctx, schema, items, path, catalog, issues); err != nil {
		return err
	}
	return walkPrefixEntities(ctx, schema, items, path, catalog, issues)
}

func walkContainedEntities(
	ctx context.Context,
	schema map[string]any,
	items []any,
	path string,
	catalog EntityCatalog,
	issues *[]ValidationIssue,
) error {
	contains, ok := schemaObject(schema[schemaContains])
	if !ok {
		return nil
	}
	applies := compileSchemaApplicability(contains)
	for index, item := range items {
		if !applies(item) {
			continue
		}
		if err := walkEntities(
			ctx,
			contains,
			item,
			indexedPath(path, index),
			catalog,
			issues,
		); err != nil {
			return err
		}
	}
	return nil
}

func walkPrefixEntities(
	ctx context.Context,
	schema map[string]any,
	items []any,
	path string,
	catalog EntityCatalog,
	issues *[]ValidationIssue,
) error {
	prefixes, ok := schemaList(schema[schemaPrefixItems])
	if !ok {
		return nil
	}
	for index, raw := range prefixes {
		if index >= len(items) {
			break
		}
		child, ok := schemaObject(raw)
		if !ok {
			continue
		}
		if err := walkEntities(
			ctx,
			child,
			items[index],
			indexedPath(path, index),
			catalog,
			issues,
		); err != nil {
			return err
		}
	}
	return nil
}
