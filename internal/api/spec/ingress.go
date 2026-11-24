package spec

import (
	"net/http"

	"github.com/compozy/compozy/internal/api/contract"
)

func ingressOperations() []OperationSpec {
	bind := gatewayIngressBindOperation()
	bind.Path = contract.IngressBindingsPath
	bind.OperationID = "bindIngress"
	bind.Summary = "Confirm a webhook trigger against its advertised HTTPS endpoint"
	bind.RequestBody = contract.IngressBindRequest{}
	bind.Tags = []string{"ingress"}
	for i := range bind.Responses {
		if bind.Responses[i].Status == http.StatusConflict {
			bind.Responses[i] = ResponseSpec{Status: http.StatusServiceUnavailable,
				Description: "Webhook ingress unavailable", Body: contract.ErrorPayload{}}
		}
	}

	unbind := gatewayIngressUnbindOperation()
	unbind.Path = contract.IngressBindingsPath + "/{subject_kind}/{subject_id}"
	unbind.OperationID = "unbindIngress"
	unbind.Tags = []string{"ingress"}
	for i := range unbind.Responses {
		if unbind.Responses[i].Status == http.StatusOK {
			unbind.Responses[i] = ResponseSpec{Status: http.StatusNoContent, Description: "Unbound"}
		}
	}
	return []OperationSpec{bind, unbind}
}
