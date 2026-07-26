package spec

import (
	"github.com/compozy/agh/internal/network/participation"
	"github.com/getkin/kin-openapi/openapi3"
)

const (
	participationChannelPattern      = `^[a-z0-9][a-z0-9_-]{0,63}$`
	participationModeProperty        = "mode"
	participationWorkspaceIDProperty = "workspace_id"
)

func participationModeValues() []string {
	return []string{string(participation.ModeLocal), string(participation.ModeLive)}
}

func participationChannelStrategyValues() []string {
	return []string{
		string(participation.StrategyNamed),
		string(participation.StrategyRun),
		string(participation.StrategyLoopRun),
	}
}

func participationSourceValues() []string {
	return []string{
		string(participation.SourceExplicitRequest),
		string(participation.SourceTaskProfile),
		string(participation.SourceWorkspaceCoordination),
		string(participation.SourceLoopDefinition),
		string(participation.SourceAutomationJob),
		string(participation.SourceBuiltInLocal),
	}
}

func participationOwnerKindValues() []string {
	return []string{
		string(participation.OwnerKindSession),
		string(participation.OwnerKindTaskRun),
		string(participation.OwnerKindLoopRun),
		string(participation.OwnerKindAutomationRun),
	}
}

func customizeParticipationRequestSchema(schema *openapi3.Schema) {
	nullable := schema.Nullable
	*schema = openapi3.Schema{
		OneOf: []*openapi3.SchemaRef{
			{Value: participationLocalRequestSchema()},
			{Value: participationLiveRequestSchema(participation.StrategyNamed)},
			{Value: participationLiveRequestSchema(participation.StrategyRun)},
			{Value: participationLiveRequestSchema(participation.StrategyLoopRun)},
		},
		Nullable: nullable,
	}
}

func customizeParticipationSpecSchema(schema *openapi3.Schema) {
	nullable := schema.Nullable
	*schema = openapi3.Schema{
		OneOf: []*openapi3.SchemaRef{
			{Value: participationLocalSpecSchema()},
			{Value: participationLiveSpecSchema()},
		},
		Nullable: nullable,
	}
}

func participationLocalRequestSchema() *openapi3.Schema {
	return openapi3.NewObjectSchema().
		WithProperty(participationModeProperty, stringEnumSchema(string(participation.ModeLocal))).
		WithoutAdditionalProperties()
}

func participationLiveRequestSchema(strategy participation.ChannelStrategy) *openapi3.Schema {
	schema := openapi3.NewObjectSchema().
		WithProperty(participationModeProperty, stringEnumSchema(string(participation.ModeLive))).
		WithProperty("channel_strategy", stringEnumSchema(string(strategy))).
		WithProperty("bounds", participationBoundsRequestSchema()).
		WithoutAdditionalProperties()
	schema.Required = []string{"channel_strategy", participationModeProperty}
	if strategy == participation.StrategyNamed {
		schema.WithProperty("channel_id", participationChannelSchema())
		schema.Required = append(schema.Required, "channel_id")
	}
	return schema
}

func participationLocalSpecSchema() *openapi3.Schema {
	schema := openapi3.NewObjectSchema().
		WithProperty("version", stringEnumSchema(participation.SpecVersion)).
		WithProperty(participationModeProperty, stringEnumSchema(string(participation.ModeLocal))).
		WithProperty("source", stringEnumSchema(participationSourceValues()...)).
		WithoutAdditionalProperties()
	schema.Required = []string{participationModeProperty, "source", "version"}
	return schema
}

func participationLiveSpecSchema() *openapi3.Schema {
	schema := openapi3.NewObjectSchema().
		WithProperty("version", stringEnumSchema(participation.SpecVersion)).
		WithProperty(participationModeProperty, stringEnumSchema(string(participation.ModeLive))).
		WithProperty(participationWorkspaceIDProperty, openapi3.NewStringSchema().WithMinLength(1)).
		WithProperty("channel_strategy", stringEnumSchema(participationChannelStrategyValues()...)).
		WithProperty("channel_id", participationChannelSchema()).
		WithProperty("source", stringEnumSchema(participationSourceValues()...)).
		WithProperty("bounds", participationBoundsSchema()).
		WithoutAdditionalProperties()
	schema.Required = []string{
		"bounds", "channel_id", "channel_strategy", participationModeProperty, "source", "version",
		participationWorkspaceIDProperty,
	}
	return schema
}

func participationBoundsRequestSchema() *openapi3.Schema {
	return participationBoundsProperties(openapi3.NewObjectSchema()).WithoutAdditionalProperties()
}

func participationBoundsSchema() *openapi3.Schema {
	schema := participationBoundsProperties(openapi3.NewObjectSchema()).WithoutAdditionalProperties()
	schema.Required = []string{
		"coalesce_window",
		"max_input_tokens",
		"max_output_tokens",
		"max_total_wall_time",
		"max_wake_depth",
		"max_wake_wall_time",
		"max_wakes",
	}
	return schema
}

func participationBoundsProperties(schema *openapi3.Schema) *openapi3.Schema {
	return schema.
		WithProperty("max_wakes", openapi3.NewIntegerSchema().WithMin(1)).
		WithProperty("max_wake_wall_time", openapi3.NewStringSchema().WithMinLength(1)).
		WithProperty("max_total_wall_time", openapi3.NewStringSchema().WithMinLength(1)).
		WithProperty("max_input_tokens", openapi3.NewInt64Schema().WithMin(1).WithMax(9_007_199_254_740_991)).
		WithProperty("max_output_tokens", openapi3.NewInt64Schema().WithMin(1).WithMax(9_007_199_254_740_991)).
		WithProperty("max_wake_depth", openapi3.NewIntegerSchema().WithMin(1)).
		WithProperty("coalesce_window", openapi3.NewStringSchema().WithMinLength(1))
}

func participationChannelSchema() *openapi3.Schema {
	return openapi3.NewStringSchema().WithPattern(participationChannelPattern)
}

func stringEnumSchema(values ...string) *openapi3.Schema {
	return openapi3.NewStringSchema().WithEnum(enumAsAny(values)...)
}
