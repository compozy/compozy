package spec

import (
	"net/http"

	"github.com/compozy/compozy/internal/api/contract"
)

func computerOperations() []OperationSpec {
	const path = contract.ComputersPath
	const id = path + "/{id}"
	stream := computerOperation(http.MethodGet, path+"/stream", "streamComputers",
		"Stream changes for the active account's computers", nil, ResponseSpec{
			Status: http.StatusOK, Description: "Computer changes and resets",
			ContentType: specContentTypeEventStream, Body: contract.ComputerEvent{},
		})
	stream.Parameters = []ParameterSpec{
		decimalUint64QueryParam(afterSequenceKey, "Resume from a computer event sequence", false),
		optionalLastEventIDHeaderParam("Resume from the last computer event received"),
	}
	remove := computerOperation(http.MethodDelete, id, "removeComputer", "Remove a computer from this controller",
		nil, computerSuccess(http.StatusOK, "Computer removed", contract.RemoveComputerResponse{}))
	remove.Parameters = append(remove.Parameters,
		boolQueryParam("unlink", "Also unlink the computer from the account"),
		boolQueryParam("uninstall", "Also uninstall the managed SSH service"),
	)
	forget := computerOperation(http.MethodDelete, id+"/routes/{route_id}", "removeComputerRoute",
		"Remove a learned route", nil, computerSuccess(http.StatusNoContent, "Route removed", nil))
	forget.Parameters = append(forget.Parameters, pathParam("route_id", "Learned route ID"))
	add := computerOperation(
		http.MethodPost,
		path,
		"addAccountComputer",
		"Add a linked account computer or start SSH setup",
		contract.AddComputerRequest{},
		computerSuccess(http.StatusCreated, "Computer added", contract.Computer{}),
	)
	add.Responses = append(add.Responses,
		computerSuccess(http.StatusAccepted, "SSH setup started", contract.ComputerSetupStarted{}),
		computerSuccess(http.StatusUnprocessableEntity, "SSH setup input failed", contract.ErrorPayload{}))
	return append([]OperationSpec{
		computerOperation(http.MethodGet, path, "listComputers", "List computers added to this controller", nil,
			computerSuccess(http.StatusOK, "Computer registry", contract.ComputerListResponse{})),
		computerOperation(http.MethodGet, path+"/available", "listAvailableComputers",
			"List linked computers available to add", nil,
			computerSuccess(http.StatusOK, "Available account computers", contract.AvailableComputersResponse{})),
		add,
		computerOperation(http.MethodGet, id, "getComputer", "Inspect a computer and its routes", nil,
			computerSuccess(http.StatusOK, "Computer status", contract.Computer{})),
		computerOperation(http.MethodPatch, id, "patchComputer", "Update a computer in this controller",
			contract.PatchComputerRequest{}, computerSuccess(http.StatusOK, "Updated computer", contract.Computer{})),
		remove, forget, stream,
	}, computerSetupOperations()...)
}

func computerSetupOperations() []OperationSpec {
	const setup = contract.ComputerSetupsPath + "/{setup_id}"
	status := computerOperation(http.MethodGet, setup, "getComputerSetup", "Read or wait for SSH setup progress",
		nil, computerSuccess(http.StatusOK, "Current SSH setup status", contract.ComputerSetupStatus{}))
	parameter := pathParam("setup_id", "SSH setup ID")
	status.Parameters = []ParameterSpec{parameter, intQueryParam("wait_ms", "Wait for up to 30,000 ms")}
	cancel := computerOperation(http.MethodDelete, setup, "cancelComputerSetup", "Cancel an unlinked SSH setup",
		nil, computerSuccess(http.StatusOK, "Setup stopped and slot released", contract.ComputerSetupCanceled{}))
	cancel.Parameters = []ParameterSpec{parameter}
	answer := computerOperation(http.MethodPost, setup+"/answer", "answerComputerSetup", "Answer an SSH setup prompt",
		contract.ComputerSetupAnswer{}, computerSuccess(http.StatusNoContent, "Answer accepted", nil))
	answer.Parameters = []ParameterSpec{parameter}
	return []OperationSpec{
		computerOperation(http.MethodGet, contract.ComputersPath+"/ssh-hosts", "listComputerSSHHosts",
			"Discover SSH hosts on the controller", nil,
			computerSuccess(http.StatusOK, "SSH hosts", contract.ComputerSSHHostsResponse{})),
		computerOperation(http.MethodPost, contract.ComputersPath+"/{id}/update", "startComputerUpdate",
			"Start an SSH computer update", nil,
			computerSuccess(http.StatusAccepted, "Update setup started", contract.ComputerUpdateStarted{})),
		status, cancel, answer,
	}
}

func computerSuccess(status int, description string, body any) ResponseSpec {
	return ResponseSpec{Status: status, Description: description, Body: body}
}

func computerOperation(method, path, id, summary string, input any, success ResponseSpec) OperationSpec {
	op := remoteHostOperation(method, path, id, summary, input, success)
	op.Tags = []string{"computers"}
	if len(op.Parameters) != 0 {
		op.Parameters[0].Description = "Computer ID"
	}
	return op
}
