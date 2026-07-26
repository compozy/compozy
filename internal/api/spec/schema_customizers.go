package spec

import (
	"reflect"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/network/participation"
	"github.com/getkin/kin-openapi/openapi3"
)

var putNetworkCoordinationInvitationRequestType = reflect.TypeFor[contract.PutNetworkCoordinationInvitationRequest]()

var schemaCustomizers = map[reflect.Type]func(*openapi3.Schema){
	reflect.TypeFor[binaryResponse](): func(schema *openapi3.Schema) {
		*schema = *openapi3.NewStringSchema()
		schema.Format = "binary"
	},
	reflect.TypeFor[contract.LoopGraph](): customizeLoopGraphSchema,
	reflect.TypeFor[contract.WindowManagerRevision](): func(schema *openapi3.Schema) {
		*schema = *openapi3.NewIntegerSchema().
			WithMin(0).
			WithMax(float64(contract.WindowManagerMaxSafeRevision))
	},
	reflect.TypeFor[contract.WindowManagerSnapshotFrame](): func(schema *openapi3.Schema) {
		customizeWindowManagerFrameSchema(schema, contract.WindowManagerFrameSnapshot)
	},
	reflect.TypeFor[contract.WindowManagerEventFrame](): func(schema *openapi3.Schema) {
		customizeWindowManagerFrameSchema(schema, contract.WindowManagerFrameEvent)
	},
	reflect.TypeFor[contract.WindowManagerClientFrame](): func(schema *openapi3.Schema) {
		customizeWindowManagerFrameSchema(schema, contract.WindowManagerFrameClient)
	},
	reflect.TypeFor[contract.WindowManagerErrorFrame](): func(schema *openapi3.Schema) {
		customizeWindowManagerFrameSchema(schema, contract.WindowManagerFrameError)
	},
	reflect.TypeFor[contract.WindowManagerLayoutNode]():             customizeClosedObjectSchema,
	reflect.TypeFor[contract.UpdateSettingsWindowManagerRequest]():  customizeClosedObjectSchema,
	reflect.TypeFor[contract.SettingsWindowManagerResponse]():       customizeClosedObjectSchema,
	reflect.TypeFor[contract.SettingsWindowManagerConfigPayload]():  customizeClosedObjectSchema,
	reflect.TypeFor[contract.SettingsWindowManagerGapsPayload]():    customizeClosedObjectSchema,
	reflect.TypeFor[contract.SettingsWindowManagerSnapPayload]():    customizeClosedObjectSchema,
	reflect.TypeFor[contract.SettingsWindowManagerBindingPayload](): customizeClosedObjectSchema,
	reflect.TypeFor[contract.SettingsMCPSecretInputPayload]():       customizeSettingsMCPSecretInputSchema,
	reflect.TypeFor[contract.SettingsMCPAuthExchangeRequest]():      customizeSettingsMCPAuthExchangeRequestSchema,
	rawMessageType: func(schema *openapi3.Schema) {
		*schema = *openapi3.NewSchema()
	},
	reflect.TypeFor[contract.BridgeProviderConfigPayload](): func(schema *openapi3.Schema) {
		*schema = *bridgeProviderConfigSchema()
	},
	reflect.TypeFor[contract.BridgeDeliveryDefaultsPayload](): func(schema *openapi3.Schema) {
		*schema = *bridgeDeliveryDefaultsSchema()
	},
	reflect.TypeFor[contract.NetworkSendRequest]():              customizeNetworkSendRequestSchema,
	reflect.TypeFor[contract.NetworkSubscriptionRequest]():      customizeClosedObjectSchema,
	reflect.TypeFor[contract.PromoteNetworkThreadTaskRequest](): customizeClosedObjectSchema,
	reflect.TypeFor[contract.PutNetworkCoordinationRequest]():   customizePutNetworkCoordinationRequestSchema,
	putNetworkCoordinationInvitationRequestType:                 customizePutNetworkCoordinationInvitationRequestSchema,
	reflect.TypeFor[contract.TaskPayload]():                     describeTaskBlockedReasonsProperty,
	reflect.TypeFor[contract.TaskSummaryPayload]():              describeTaskBlockedReasonsProperty,
	reflect.TypeFor[participation.Request]():                    customizeParticipationRequestSchema,
	reflect.TypeFor[participation.Spec]():                       customizeParticipationSpecSchema,
}

func customizeClosedObjectSchema(schema *openapi3.Schema) {
	if schema != nil {
		schema.WithoutAdditionalProperties()
	}
}
