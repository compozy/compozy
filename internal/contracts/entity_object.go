package contracts

import (
	"context"
	"regexp"
)

func walkObjectEntities(
	ctx context.Context,
	schema map[string]any,
	object map[string]any,
	path string,
	catalog EntityCatalog,
	issues *[]ValidationIssue,
) error {
	visited := make(map[string]bool, len(object))
	if err := walkDeclaredObjectEntities(
		ctx,
		schema,
		object,
		path,
		catalog,
		issues,
		visited,
	); err != nil {
		return err
	}
	if err := walkPatternObjectEntities(
		ctx,
		schema,
		object,
		path,
		catalog,
		issues,
		visited,
	); err != nil {
		return err
	}
	if err := walkAdditionalObjectEntities(
		ctx,
		schema,
		object,
		path,
		catalog,
		issues,
		visited,
	); err != nil {
		return err
	}
	if err := walkPropertyNameEntities(ctx, schema, object, path, catalog, issues); err != nil {
		return err
	}
	return walkDependentObjectEntities(ctx, schema, object, path, catalog, issues)
}

func walkDeclaredObjectEntities(
	ctx context.Context,
	schema map[string]any,
	object map[string]any,
	path string,
	catalog EntityCatalog,
	issues *[]ValidationIssue,
	visited map[string]bool,
) error {
	properties, _ := schemaObject(schema[schemaProperties])
	for name, childSchemaValue := range properties {
		childValue, present := object[name]
		childSchema, ok := schemaObject(childSchemaValue)
		if !present || !ok {
			continue
		}
		visited[name] = true
		if err := walkEntities(
			ctx,
			childSchema,
			childValue,
			fieldPath(path, name),
			catalog,
			issues,
		); err != nil {
			return err
		}
	}
	return nil
}

func walkPatternObjectEntities(
	ctx context.Context,
	schema map[string]any,
	object map[string]any,
	path string,
	catalog EntityCatalog,
	issues *[]ValidationIssue,
	visited map[string]bool,
) error {
	patterns, _ := schemaObject(schema[schemaPatternProperties])
	for expression, childSchemaValue := range patterns {
		compiled, err := regexp.Compile(expression)
		if err != nil {
			continue
		}
		childSchema, ok := schemaObject(childSchemaValue)
		if !ok {
			continue
		}
		for name, childValue := range object {
			if !compiled.MatchString(name) {
				continue
			}
			visited[name] = true
			if err := walkEntities(
				ctx,
				childSchema,
				childValue,
				fieldPath(path, name),
				catalog,
				issues,
			); err != nil {
				return err
			}
		}
	}
	return nil
}

func walkAdditionalObjectEntities(
	ctx context.Context,
	schema map[string]any,
	object map[string]any,
	path string,
	catalog EntityCatalog,
	issues *[]ValidationIssue,
	visited map[string]bool,
) error {
	childSchema, ok := schemaObject(schema[schemaAdditionalProperties])
	if !ok {
		childSchema, ok = schemaObject(schema[schemaUnevaluatedProperties])
	}
	if !ok {
		return nil
	}
	for name, childValue := range object {
		if visited[name] {
			continue
		}
		if err := walkEntities(
			ctx,
			childSchema,
			childValue,
			fieldPath(path, name),
			catalog,
			issues,
		); err != nil {
			return err
		}
	}
	return nil
}

func walkPropertyNameEntities(
	ctx context.Context,
	schema map[string]any,
	object map[string]any,
	path string,
	catalog EntityCatalog,
	issues *[]ValidationIssue,
) error {
	names, ok := schemaObject(schema[schemaPropertyNames])
	if !ok {
		return nil
	}
	for name := range object {
		if err := walkEntities(
			ctx,
			names,
			name,
			fieldPath(path, name),
			catalog,
			issues,
		); err != nil {
			return err
		}
	}
	return nil
}

func walkDependentObjectEntities(
	ctx context.Context,
	schema map[string]any,
	object map[string]any,
	path string,
	catalog EntityCatalog,
	issues *[]ValidationIssue,
) error {
	dependent, _ := schemaObject(schema[schemaDependentSchemas])
	for trigger, childSchemaValue := range dependent {
		if _, present := object[trigger]; !present {
			continue
		}
		childSchema, ok := schemaObject(childSchemaValue)
		if !ok {
			continue
		}
		if err := walkEntities(ctx, childSchema, object, path, catalog, issues); err != nil {
			return err
		}
	}
	return nil
}
