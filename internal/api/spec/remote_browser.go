package spec

import (
	"net/http"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/remoteproto"
)

func remoteBrowserOperations() []OperationSpec {
	operations := []OperationSpec{
		browserOperation(http.MethodGet, remoteproto.BrowserStatusPath, "getBrowserStatus",
			"Read the hosted listener status", "", nil, http.StatusOK, contract.BrowserStatus{}),
		browserOperation(http.MethodPost, remoteproto.BrowserSessionPath, "startBrowserSession",
			"Redeem a hosted sign-in code", OperationAuthBrowserDPoP, contract.BrowserSessionRequest{},
			http.StatusCreated, contract.BrowserSessionResponse{}),
		browserOperation(http.MethodDelete, remoteproto.BrowserSessionPath, "endBrowserSession",
			"Revoke the current browser session", OperationAuthBrowserSession, nil, http.StatusNoContent, nil),
		browserOperation(http.MethodGet, remoteproto.BrowserApprovalPath, "getBrowserApproval",
			"Read the approval for the waiting browser key", OperationAuthBrowserDPoP, nil,
			http.StatusOK, contract.BrowserApprovalStatus{}),
		browserOperation(http.MethodGet, remoteproto.BrowserPeersPath, "listBrowserPeers",
			"List other hosted computers on this account", OperationAuthBrowserSession, nil,
			http.StatusOK, contract.BrowserPeersResponse{}),
		browserOperation(
			http.MethodPost,
			remoteproto.BrowserStreamTicketsPath,
			"mintBrowserStreamTicket",
			"Mint a single-use ticket for one stream path",
			OperationAuthBrowserSession,
			contract.BrowserStreamTicketRequest{},
			http.StatusCreated,
			contract.BrowserStreamTicketResponse{},
		),
	}
	return operations
}

func browserOperation(
	method, path, id, summary string, auth OperationAuth, request any, status int, response any,
) OperationSpec {
	op := remoteHostOperation(method, path, id, summary, request,
		ResponseSpec{Status: status, Description: http.StatusText(status), Body: response})
	op.Transports = []Transport{TransportHostedHTTP}
	if path == remoteproto.BrowserApprovalPath {
		op.Parameters = []ParameterSpec{pathParam("id", "Approval ID")}
	}
	if auth != "" {
		op.Auth = map[Transport]OperationAuth{TransportHostedHTTP: auth}
		op.Parameters = append(op.Parameters, ParameterSpec{
			Name: "DPoP", In: "header", Required: true, Kind: specStringKey, Description: "Proof from the browser key",
		})
	}
	return op
}
