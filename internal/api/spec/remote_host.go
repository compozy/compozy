package spec

import (
	"net/http"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
)

const specRemoteHostKey = "remote"

func remoteHostOperations() []OperationSpec {
	const device = contract.RemoteDevicesPath + "/{id}"
	ok := func(description string, body any) ResponseSpec {
		return ResponseSpec{Status: http.StatusOK, Description: description, Body: body}
	}
	return []OperationSpec{
		remoteHostOperation(
			http.MethodGet,
			contract.RemotePath,
			"getRemoteStatus",
			"Read this computer's remote access status",
			nil,
			ok("Remote access status", contract.RemoteStatusResponse{}),
		),
		remoteHostOperation(
			http.MethodPatch,
			contract.RemotePath,
			"patchRemoteStatus",
			"Update this computer's remote access",
			contract.RemotePatchRequest{},
			ok("Updated remote access", contract.RemoteStatusResponse{}),
		),
		remoteHostOperation(
			http.MethodPost,
			contract.RemoteLinkPath,
			"linkRemote",
			"Link this computer from the local daemon",
			contract.RemoteLinkRequest{},
			ok("Computer link or device activation", contract.RemoteLinkResponse{}),
		),
		remoteHostOperation(
			http.MethodDelete,
			contract.RemoteLinkPath,
			"unlinkRemote",
			"Unlink this computer from the local daemon",
			nil,
			ResponseSpec{Status: http.StatusNoContent, Description: "Computer unlinked"},
		),
		remoteHostOperation(
			http.MethodGet,
			contract.RemoteDevicesPath,
			"listRemoteDevices",
			"List devices allowed on this computer",
			nil,
			ok("Remote devices and pending approvals", contract.RemoteDevicesResponse{}),
		),
		remoteHostOperation(
			http.MethodPatch,
			device,
			"patchRemoteDevice",
			"Change device scopes or decide a local approval",
			contract.RemoteDevicePatchRequest{},
			ok("Updated device", contract.RemoteDeviceResponse{}),
		),
		remoteHostOperation(http.MethodDelete, device, "revokeRemoteDevice", "Revoke a device on this account",
			nil, ResponseSpec{Status: http.StatusNoContent, Description: "Device revoked"}),
	}
}

func remoteHostOperation(method, path, id, summary string, request any, success ResponseSpec) OperationSpec {
	op := OperationSpec{
		Method:      method,
		Path:        path,
		OperationID: id,
		Summary:     summary,
		RequestBody: request,
		AppAccess:   AppAccessDenied,
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Tags:        []string{specRemoteHostKey},
		Responses:   []ResponseSpec{success},
	}
	if strings.Contains(path, "{id}") {
		op.Parameters = []ParameterSpec{pathParam("id", "Device ID")}
	}
	for _, status := range []int{
		http.StatusBadRequest, http.StatusUnauthorized, http.StatusPaymentRequired, http.StatusForbidden,
		http.StatusNotFound, http.StatusConflict, http.StatusTooManyRequests, http.StatusServiceUnavailable,
	} {
		op.Responses = append(
			op.Responses,
			ResponseSpec{Status: status, Description: http.StatusText(status), Body: contract.ErrorPayload{}},
		)
	}
	return op
}
