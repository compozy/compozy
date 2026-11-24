package core

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/computers/sshsetup"
	"github.com/compozy/compozy/internal/remotehost"
)

func (h *BaseHandlers) sshSetups(c *gin.Context) *sshsetup.Manager {
	service, ok := h.Computers.(interface{ SSHSetups() *sshsetup.Manager })
	if ok && service.SSHSetups() != nil {
		c.Header("Cache-Control", "no-store")
		return service.SSHSetups()
	}
	c.AbortWithStatusJSON(http.StatusServiceUnavailable, contract.ErrorPayload{
		Code: "computer_ssh_setup_unavailable", Error: "SSH computer setup is unavailable.",
	})
	return nil
}

func (h *BaseHandlers) addSSHComputer(c *gin.Context, input contract.AddComputerRequest) {
	if manager := h.sshSetups(c); manager != nil {
		id, err := manager.Start(c.Request.Context(), sshsetup.Request{
			Host: input.Host, User: input.User, Port: input.Port, Name: input.Name,
			TrustFingerprint: input.TrustHostFingerprint,
		})
		h.computerResult(c, http.StatusAccepted, contract.ComputerSetupStarted{SetupID: id}, err)
	}
}

func (h *BaseHandlers) ComputerSSHHosts(c *gin.Context) {
	manager := h.sshSetups(c)
	if manager == nil {
		return
	}
	found, err := manager.Hosts(c.Request.Context())
	hosts := make([]contract.ComputerSSHHost, 0, len(found))
	for _, host := range found {
		hosts = append(hosts, contract.ComputerSSHHost{
			Alias: host.Alias, Hostname: host.Hostname, User: host.User, Port: host.Port, Source: host.Source,
		})
	}
	h.computerResult(c, http.StatusOK, contract.ComputerSSHHostsResponse{Hosts: hosts}, err)
}

func (h *BaseHandlers) StartComputerUpdate(c *gin.Context) {
	if manager := h.sshSetups(c); manager != nil {
		id, err := manager.StartUpdate(c.Request.Context(), computerID(c))
		h.computerResult(c, http.StatusAccepted, contract.ComputerUpdateStarted{SetupID: id}, err)
	}
}

func (h *BaseHandlers) ComputerSetupStatus(c *gin.Context) {
	manager := h.sshSetups(c)
	if manager == nil {
		return
	}
	var wait time.Duration
	if raw := c.Query("wait_ms"); raw != "" {
		millis, err := strconv.Atoi(raw)
		if err != nil || millis < 0 {
			invalidComputerBody(c)
			return
		}
		wait = time.Duration(min(millis, int(sshsetup.MaxStatusWait.Milliseconds()))) * time.Millisecond
	}
	status, err := manager.Status(c.Request.Context(), c.Param("setup_id"), wait)
	h.computerResult(c, http.StatusOK, setupDTO(status), err)
}

func (h *BaseHandlers) CancelComputerSetup(c *gin.Context) {
	if manager := h.sshSetups(c); manager != nil {
		id := c.Param("setup_id")
		err := manager.Cancel(c.Request.Context(), id)
		h.computerResult(c, http.StatusOK, contract.ComputerSetupCanceled{SetupID: id, State: sshsetup.Canceled}, err)
	}
}

func (h *BaseHandlers) AnswerComputerSetup(c *gin.Context) {
	manager := h.sshSetups(c)
	if manager == nil {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 24<<10)
	var input contract.ComputerSetupAnswer
	if err := decodeStrictJSONBody(c, &input); err != nil || !validSetupAnswer(input) {
		invalidComputerBody(c)
		return
	}
	ctx := c.Request.Context()
	_, app := AppTokenFromContext(ctx)
	_, remote := remotehost.PrincipalFromContext(ctx)
	operator := !app && !remote && !hasAgentCallerIdentityCredentials(agentCallerCredentialsFromRequest(c))
	answer := sshsetup.Answer{Kind: input.Kind, Accept: input.Accept != nil && *input.Accept, Value: input.Value}
	err := manager.Answer(ctx, c.Param("setup_id"), answer, operator)
	h.computerResult(c, http.StatusNoContent, nil, err)
}

func validSetupAnswer(input contract.ComputerSetupAnswer) bool {
	switch input.Kind {
	case sshsetup.TrustHost:
		return input.Accept != nil && input.Value == ""
	case sshsetup.Secret:
		return input.Accept == nil
	default:
		return false
	}
}

func setupDTO(status sshsetup.Status) contract.ComputerSetupStatus {
	result := contract.ComputerSetupStatus{SetupID: status.ID, Step: status.Step, State: status.State}
	if status.ComputerID != "" {
		result.ComputerID = new(status.ComputerID)
	}
	if status.Prompt != nil {
		result.Prompt = &contract.ComputerSetupPrompt{
			Kind: status.Prompt.Kind, Host: status.Prompt.Host, Fingerprint: status.Prompt.Fingerprint,
		}
		if status.State == sshsetup.NeedsOperatorSecret {
			result.PromptKind = status.Prompt.Kind
		}
	}
	if status.Error != nil {
		result.Error = new(setupErrorDTO(status.Error))
	}
	return result
}

func setupErrorDTO(fault *sshsetup.Failure) contract.ErrorPayload {
	return contract.ErrorPayload{Code: fault.Code, Error: fault.Message, Details: fault.Details}
}
