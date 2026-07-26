package sdkts

import (
	"reflect"

	"github.com/compozy/agh/internal/network/participation"
)

var specializedTypeAliases = map[reflect.Type]string{
	reflect.TypeFor[participation.Request](): `
  | {
      mode?: "local";
      channel_strategy?: never;
      channel_id?: never;
      bounds?: never;
    }
  | {
      mode: "live";
      channel_strategy: "named";
      channel_id: string;
      bounds?: NetworkParticipationBoundsRequest;
    }
  | {
      mode: "live";
      channel_strategy: "run";
      channel_id?: never;
      bounds?: NetworkParticipationBoundsRequest;
    }
  | {
      mode: "live";
      channel_strategy: "loop_run";
      channel_id?: never;
      bounds?: NetworkParticipationBoundsRequest;
    }`,
	reflect.TypeFor[participation.Spec](): `
  | {
      version: "network-participation/v1";
      mode: "local";
      workspace_id?: never;
      channel_strategy?: never;
      channel_id?: never;
      source: NetworkParticipationSource;
      bounds?: never;
    }
  | {
      version: "network-participation/v1";
      mode: "live";
      workspace_id: string;
      channel_strategy: "named" | "run" | "loop_run";
      channel_id: string;
      source: NetworkParticipationSource;
      bounds: NetworkParticipationBounds;
    }`,
}

func (g *generator) prepareSpecializedTypeDependencies() {
	for _, typ := range []reflect.Type{
		reflect.TypeFor[participation.BoundsRequest](),
		reflect.TypeFor[participation.Bounds](),
		reflect.TypeFor[participation.Mode](),
		reflect.TypeFor[participation.ChannelStrategy](),
		reflect.TypeFor[participation.Source](),
		reflect.TypeFor[participation.OwnerKind](),
	} {
		if _, exists := g.typeNames[typ]; exists {
			continue
		}
		name := generatedTypeName(typ)
		g.typeNames[typ] = name
		g.queued[name] = typ
	}
}

func isSpecializedTypeAlias(typ reflect.Type) bool {
	_, ok := specializedTypeAliases[typ]
	return ok
}

func specializedTypeAlias(typ reflect.Type) string {
	return specializedTypeAliases[typ]
}

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
