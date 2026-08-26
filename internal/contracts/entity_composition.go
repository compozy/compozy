package contracts

import "context"

func walkCompositionEntities(
	ctx context.Context,
	schema map[string]any,
	value any,
	path string,
	catalog EntityCatalog,
	issues *[]ValidationIssue,
) error {
	for _, keyword := range []string{schemaAllOf, schemaAnyOf, schemaOneOf} {
		branches, _ := schemaList(schema[keyword])
		for _, raw := range branches {
			branch, ok := schemaObject(raw)
			if !ok {
				continue
			}
			applies := compileSchemaApplicability(branch)
			if !applies(value) {
				continue
			}
			if err := walkEntities(ctx, branch, value, path, catalog, issues); err != nil {
				return err
			}
		}
	}
	return nil
}

func walkConditionalEntities(
	ctx context.Context,
	schema map[string]any,
	value any,
	path string,
	catalog EntityCatalog,
	issues *[]ValidationIssue,
) error {
	condition, ok := schemaObject(schema[schemaIf])
	if !ok {
		return nil
	}
	keyword := schemaElse
	applies := compileSchemaApplicability(condition)
	if applies(value) {
		keyword = schemaThen
	}
	branch, ok := schemaObject(schema[keyword])
	if !ok {
		return nil
	}
	return walkEntities(ctx, branch, value, path, catalog, issues)
}
