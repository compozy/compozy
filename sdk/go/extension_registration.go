package aghsdk

import (
	"context"

	"slices"
	"strings"
)

// GetImplementedMethods returns the sorted method list advertised during initialize.
func (e *Extension) GetImplementedMethods() []string {
	if e == nil {
		return nil
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	methods := map[string]struct{}{
		healthCheckMethod: {},
		shutdownMethod:    {},
	}
	for method := range e.handlers {
		methods[method] = struct{}{}
	}
	if len(e.toolHandlers) > 0 {
		methods[ExtensionServiceMethodProvideTools] = struct{}{}
		methods[ExtensionServiceMethodToolsCall] = struct{}{}
	}
	if len(e.watchHandlers) > 0 {
		methods[ExtensionServiceMethodWatchPoll] = struct{}{}
	}
	out := make([]string, 0, len(methods))
	for method := range methods {
		out = append(out, method)
	}
	slices.Sort(out)
	return out
}

// GetToolDescriptors returns runtime descriptors registered by Tool.
func (e *Extension) GetToolDescriptors() []ExtensionToolRuntimeDescriptor {
	if e == nil {
		return nil
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	descriptors := make([]ExtensionToolRuntimeDescriptor, 0, len(e.toolHandlers))
	for _, tool := range e.toolHandlers {
		descriptor := tool.descriptor
		descriptor.Capabilities = slices.Clone(descriptor.Capabilities)
		descriptors = append(descriptors, descriptor)
	}
	slices.SortFunc(descriptors, func(a, b ExtensionToolRuntimeDescriptor) int {
		return strings.Compare(a.Handler, b.Handler)
	})
	return descriptors
}

func (e *Extension) registerTool(
	handler string,
	options ToolOptions,
	fn func(context.Context, rawToolRequest) (ToolResult, error),
) error {
	cleanHandler := strings.TrimSpace(handler)
	if cleanHandler == "" {
		return NewInvalidParamsError("tool handler is required", nil)
	}
	inputSchema, err := normalizeSchema(options.InputSchema, "input_schema", true)
	if err != nil {
		return err
	}
	outputSchema, err := normalizeSchema(options.OutputSchema, "output_schema", false)
	if err != nil {
		return err
	}
	toolID := options.ID
	if toolID == "" {
		toolID, err = canonicalExtensionToolID(e.definition.Name, cleanHandler)
		if err != nil {
			return err
		}
	}
	if err := toolID.Validate(); err != nil {
		return err
	}
	inputDigest, err := SchemaDigest(inputSchema)
	if err != nil {
		return err
	}
	outputDigest := ""
	if len(outputSchema) > 0 {
		outputDigest, err = SchemaDigest(outputSchema)
		if err != nil {
			return err
		}
	}
	risk := options.Risk
	if risk == "" {
		if options.ReadOnly {
			risk = RiskRead
		} else {
			risk = RiskMutating
		}
	}
	descriptor := ExtensionToolRuntimeDescriptor{
		ID:                 toolID,
		Handler:            cleanHandler,
		InputSchemaDigest:  inputDigest,
		OutputSchemaDigest: outputDigest,
		ReadOnly:           options.ReadOnly,
		Risk:               risk,
		Capabilities:       normalizeStrings(options.Capabilities),
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	if e.initialized {
		return NewInvalidParamsError("extension registration is closed after initialize", nil)
	}
	if err := e.ensureToolRegistrationAvailableLocked(cleanHandler, toolID); err != nil {
		return err
	}
	e.toolHandlers[cleanHandler] = registeredTool{
		descriptor:           descriptor,
		handler:              fn,
		sensitiveInputFields: normalizeStrings(options.SensitiveInputFields),
	}
	e.definition.Capabilities.Provides = normalizeStrings(
		append(e.definition.Capabilities.Provides, CapabilityToolProvider),
	)
	e.transport.Handle(ExtensionServiceMethodProvideTools, e.dispatch)
	e.transport.Handle(ExtensionServiceMethodToolsCall, e.dispatch)
	return nil
}

func (e *Extension) ensureToolRegistrationAvailableLocked(handler string, toolID ToolID) error {
	if _, ok := e.toolHandlers[handler]; ok {
		return NewInvalidParamsError("tool handler already registered", map[string]any{extensionHandlerKey: handler})
	}
	for existingHandler, existingTool := range e.toolHandlers {
		if existingTool.descriptor.ID == toolID {
			return NewInvalidParamsError("tool id already registered", map[string]any{
				extensionToolIDKey:  toolID,
				"existing_handler":  existingHandler,
				extensionHandlerKey: handler,
			})
		}
	}
	if _, ok := e.handlers[ExtensionServiceMethodProvideTools]; ok {
		return NewInvalidParamsError("provide_tools is reserved by Tool", nil)
	}
	if _, ok := e.handlers[ExtensionServiceMethodToolsCall]; ok {
		return NewInvalidParamsError("tools/call is reserved by Tool", nil)
	}
	return nil
}

func (e *Extension) bindBuiltins() {
	e.transport.Handle(initializeMethod, e.dispatch)
	e.transport.Handle(healthCheckMethod, e.dispatch)
	e.transport.Handle(shutdownMethod, e.dispatch)
}
