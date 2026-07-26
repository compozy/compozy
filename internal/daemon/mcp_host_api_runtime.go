package daemon

import (
	"context"
	"encoding/json"
	"errors"

	bridgepkg "github.com/compozy/agh/internal/bridges"
	extensionpkg "github.com/compozy/agh/internal/extension"
	watchpkg "github.com/compozy/agh/internal/loop/watch"
	mcppkg "github.com/compozy/agh/internal/mcp"
)

type mcpHostAPIRuntimeInvoker struct {
	current func() extensionRuntime
}

func newMCPHostAPIRuntimeInvoker(current func() extensionRuntime) mcppkg.HostAPIInvoker {
	return mcpHostAPIRuntimeInvoker{current: current}
}

func (i mcpHostAPIRuntimeInvoker) InvokeHostAPI(
	ctx context.Context,
	serveSessionID string,
	workspace string,
	method string,
	params json.RawMessage,
) (json.RawMessage, error) {
	if i.current == nil {
		return nil, errors.New("daemon: extension runtime accessor is not configured")
	}
	runtime := i.current()
	invoker, ok := runtime.(mcppkg.HostAPIInvoker)
	if !ok || invoker == nil {
		return nil, errors.New("daemon: mcp host api façade is unavailable")
	}
	return invoker.InvokeHostAPI(ctx, serveSessionID, workspace, method, params)
}

func (i mcpHostAPIRuntimeInvoker) CloseHostAPISession(
	ctx context.Context,
	serveSessionID string,
) error {
	if i.current == nil {
		return errors.New("daemon: extension runtime accessor is not configured")
	}
	runtime := i.current()
	invoker, ok := runtime.(mcppkg.HostAPIInvoker)
	if !ok || invoker == nil {
		return errors.New("daemon: mcp host api façade is unavailable")
	}
	return invoker.CloseHostAPISession(ctx, serveSessionID)
}

type extensionRuntimeWithMCPHostAPI struct {
	*extensionpkg.Manager
	hostAPI *mcppkg.HostAPIFacade
}

var (
	_ extensionRuntime                  = (*extensionRuntimeWithMCPHostAPI)(nil)
	_ extensionpkg.ExtensionToolRuntime = (*extensionRuntimeWithMCPHostAPI)(nil)
	_ extensionpkg.ModelSourceRuntime   = (*extensionRuntimeWithMCPHostAPI)(nil)
	_ watchpkg.Poller                   = (*extensionRuntimeWithMCPHostAPI)(nil)
	_ bridgepkg.DeliveryTransport       = (*extensionRuntimeWithMCPHostAPI)(nil)
	_ bridgepkg.TargetSnapshotTransport = (*extensionRuntimeWithMCPHostAPI)(nil)
	_ bridgepkg.BridgeControlTransport  = (*extensionRuntimeWithMCPHostAPI)(nil)
	_ mcppkg.HostAPIInvoker             = (*extensionRuntimeWithMCPHostAPI)(nil)
)

func newExtensionRuntimeWithMCPHostAPI(
	manager *extensionpkg.Manager,
	hostAPI *mcppkg.HostAPIFacade,
) extensionRuntime {
	if manager == nil {
		return nil
	}
	return &extensionRuntimeWithMCPHostAPI{
		Manager: manager,
		hostAPI: hostAPI,
	}
}

func (r *extensionRuntimeWithMCPHostAPI) InvokeHostAPI(
	ctx context.Context,
	serveSessionID string,
	workspace string,
	method string,
	params json.RawMessage,
) (json.RawMessage, error) {
	if r == nil || r.hostAPI == nil {
		return nil, errors.New("daemon: mcp host api façade is unavailable")
	}
	return r.hostAPI.InvokeHostAPI(ctx, serveSessionID, workspace, method, params)
}

func (r *extensionRuntimeWithMCPHostAPI) CloseHostAPISession(
	ctx context.Context,
	serveSessionID string,
) error {
	if r == nil || r.hostAPI == nil {
		return errors.New("daemon: mcp host api façade is unavailable")
	}
	return r.hostAPI.CloseHostAPISession(ctx, serveSessionID)
}

func (r *extensionRuntimeWithMCPHostAPI) Stop(ctx context.Context) error {
	if r == nil || r.Manager == nil {
		return nil
	}
	err := r.Manager.Stop(ctx)
	if r.hostAPI != nil {
		err = errors.Join(err, r.hostAPI.Close(ctx))
	}
	return err
}
