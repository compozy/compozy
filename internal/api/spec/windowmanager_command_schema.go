package spec

import (
	"maps"
	"slices"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/getkin/kin-openapi/openapi3"
)

type windowManagerCommandSchemaSpec struct {
	commandID      contract.WindowManagerCommandID
	payload        windowManagerComponentSchema
	requiresClient bool
}

var windowManagerCommandSchemaSpecs = []windowManagerCommandSchemaSpec{
	newWindowManagerCommandSchemaSpec(
		contract.WindowManagerCommandDesktopCreate,
		"WindowManagerCreateDesktopPayload",
		contract.WindowManagerCreateDesktopPayload{},
		false,
	),
	newWindowManagerCommandSchemaSpec(
		contract.WindowManagerCommandDesktopUpdate,
		"WindowManagerUpdateDesktopPayload",
		contract.WindowManagerUpdateDesktopPayload{},
		false,
	),
	newWindowManagerCommandSchemaSpec(
		contract.WindowManagerCommandDesktopReorder,
		"WindowManagerReorderDesktopPayload",
		contract.WindowManagerReorderDesktopPayload{},
		false,
	),
	newWindowManagerCommandSchemaSpec(
		contract.WindowManagerCommandDesktopSwitch,
		"WindowManagerSwitchDesktopPayload",
		contract.WindowManagerSwitchDesktopPayload{},
		true,
	),
	newWindowManagerCommandSchemaSpec(
		contract.WindowManagerCommandDesktopDelete,
		"WindowManagerDeleteDesktopPayload",
		contract.WindowManagerDeleteDesktopPayload{},
		false,
	),
	newWindowManagerCommandSchemaSpec(
		contract.WindowManagerCommandWindowOpen,
		"WindowManagerOpenWindowPayload",
		contract.WindowManagerOpenWindowPayload{},
		false,
	),
	newWindowManagerCommandSchemaSpec(
		contract.WindowManagerCommandWindowNavigate,
		"WindowManagerNavigateWindowPayload",
		contract.WindowManagerNavigateWindowPayload{},
		false,
	),
	newWindowManagerCommandSchemaSpec(
		contract.WindowManagerCommandWindowClose,
		"WindowManagerCloseWindowPayload",
		contract.WindowManagerCloseWindowPayload{},
		false,
	),
	newWindowManagerCommandSchemaSpec(
		contract.WindowManagerCommandWindowFocus,
		"WindowManagerFocusWindowPayload",
		contract.WindowManagerFocusWindowPayload{},
		true,
	),
	newWindowManagerCommandSchemaSpec(
		contract.WindowManagerCommandWindowMove,
		"WindowManagerMoveWindowPayload",
		contract.WindowManagerMoveWindowPayload{},
		false,
	),
	newWindowManagerCommandSchemaSpec(
		contract.WindowManagerCommandWindowSwap,
		"WindowManagerSwapWindowsPayload",
		contract.WindowManagerSwapWindowsPayload{},
		false,
	),
	newWindowManagerCommandSchemaSpec(
		contract.WindowManagerCommandWindowToggleFloating,
		"WindowManagerToggleFloatingPayload",
		contract.WindowManagerToggleFloatingPayload{},
		false,
	),
	newWindowManagerCommandSchemaSpec(
		contract.WindowManagerCommandWindowZoom,
		"WindowManagerZoomWindowPayload",
		contract.WindowManagerZoomWindowPayload{},
		true,
	),
	newWindowManagerCommandSchemaSpec(
		contract.WindowManagerCommandLayoutArrange,
		"WindowManagerArrangeLayoutPayload",
		contract.WindowManagerArrangeLayoutPayload{},
		false,
	),
	newWindowManagerCommandSchemaSpec(
		contract.WindowManagerCommandLayoutResize,
		"WindowManagerResizeLayoutPayload",
		contract.WindowManagerResizeLayoutPayload{},
		false,
	),
	newWindowManagerCommandSchemaSpec(
		contract.WindowManagerCommandLayoutBalance,
		"WindowManagerBalanceLayoutPayload",
		contract.WindowManagerBalanceLayoutPayload{},
		false,
	),
	newWindowManagerCommandSchemaSpec(
		contract.WindowManagerCommandLayoutUndo,
		"WindowManagerUndoLayoutPayload",
		contract.WindowManagerUndoLayoutPayload{},
		false,
	),
	newWindowManagerCommandSchemaSpec(
		contract.WindowManagerCommandLayoutRedo,
		"WindowManagerRedoLayoutPayload",
		contract.WindowManagerRedoLayoutPayload{},
		false,
	),
	newWindowManagerCommandSchemaSpec(
		contract.WindowManagerCommandLayoutReplace,
		"WindowManagerReplaceLayoutPayload",
		contract.WindowManagerReplaceLayoutPayload{},
		false,
	),
}

func newWindowManagerCommandSchemaSpec(
	commandID contract.WindowManagerCommandID,
	payloadName string,
	payload any,
	requiresClient bool,
) windowManagerCommandSchemaSpec {
	return windowManagerCommandSchemaSpec{
		commandID:      commandID,
		payload:        windowManagerComponentSchema{name: payloadName, value: payload},
		requiresClient: requiresClient,
	}
}

func customizeWindowManagerCommandRequest(schemas openapi3.Schemas) {
	requestRef := schemas["WindowManagerCommandRequest"]
	if requestRef == nil || requestRef.Value == nil {
		return
	}

	base := requestRef.Value
	variants := make(openapi3.SchemaRefs, 0, len(windowManagerCommandSchemaSpecs))
	for _, command := range windowManagerCommandSchemaSpecs {
		properties := maps.Clone(base.Properties)
		properties["command_id"] = openapi3.NewStringSchema().
			WithEnum(string(command.commandID)).
			NewRef()
		properties["payload"] = openapi3.NewSchemaRef(
			"#/components/schemas/"+command.payload.name,
			schemas[command.payload.name].Value,
		)
		if command.requiresClient {
			properties["client_id"] = requiredWindowManagerClientIDSchema(base.Properties["client_id"])
		}

		required := append([]string(nil), base.Required...)
		if command.requiresClient && !slices.Contains(required, "client_id") {
			required = append(required, "client_id")
			slices.Sort(required)
		}
		variant := openapi3.NewObjectSchema().WithoutAdditionalProperties()
		variant.Properties = properties
		variant.Required = required
		variants = append(variants, variant.NewRef())
	}

	*requestRef.Value = openapi3.Schema{
		Description: "Semantic window-manager command selected by command_id",
		OneOf:       variants,
		Discriminator: &openapi3.Discriminator{
			PropertyName: "command_id",
		},
	}
}

func requiredWindowManagerClientIDSchema(optional *openapi3.SchemaRef) *openapi3.SchemaRef {
	if optional == nil || optional.Value == nil {
		return openapi3.NewStringSchema().NewRef()
	}
	required := *optional.Value
	required.Nullable = false
	return required.NewRef()
}
