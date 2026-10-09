package mcp

import (
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

const hostedRequestIDMeta = "compozyHostedRequestId"

type hostedIdentityTransport struct{ sdkmcp.Transport }

var _ sdkmcp.Transport = hostedIdentityTransport{}

func (t hostedIdentityTransport) Connect(ctx context.Context) (sdkmcp.Connection, error) {
	conn, err := t.Transport.Connect(ctx)
	if err != nil {
		return nil, err
	}
	return hostedIdentityConnection{Connection: conn}, nil
}

type hostedIdentityConnection struct{ sdkmcp.Connection }

var _ sdkmcp.Connection = hostedIdentityConnection{}

func (c hostedIdentityConnection) Read(ctx context.Context) (jsonrpc.Message, error) {
	message, err := c.Connection.Read(ctx)
	if err != nil {
		return nil, err
	}
	request, ok := message.(*jsonrpc.Request)
	if !ok || request.Method != "tools/call" || !request.ID.IsValid() {
		return message, nil
	}
	var params map[string]jsontext.Value
	if err := json.Unmarshal(request.Params, &params); err != nil || params == nil {
		return message, nil
	}
	meta := make(map[string]any)
	if raw := params["_meta"]; len(raw) > 0 {
		if err := json.Unmarshal(raw, &meta); err != nil {
			return message, nil
		}
	}
	if meta == nil {
		meta = make(map[string]any)
	}
	id, err := json.Marshal(request.ID.Raw())
	if err != nil {
		return nil, err
	}
	meta[hostedRequestIDMeta] = string(id)
	params["_meta"], err = json.Marshal(meta)
	if err != nil {
		return nil, err
	}
	copied := *request
	copied.Params, err = json.Marshal(params)
	return &copied, err
}
