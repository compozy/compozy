package sshsetup

import (
	"context"
	"encoding/json/v2"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/cloudclient"
	"github.com/compozy/compozy/internal/remoteproto"
	"github.com/compozy/compozy/internal/service"
)

func (m *Manager) run(job *setupRun, fingerprint string) (err error) {
	t, err := targetFromAddress(job.row.TargetAlias, job.row.HostKey)
	if err != nil {
		return err
	}
	if job.row.Kind == KindAdd && job.row.ComputerID == "" {
		if err := m.reserve(job); err != nil {
			return err
		}
	}
	if job.row.Step == StepAdmit {
		if err := m.advance(job, StepVerify); err != nil {
			return err
		}
	}
	manifest, err := m.releases.verify(job.ctx, job.row.ReleaseVersion)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, os.RemoveAll(manifest.dir)) }()
	if job.row.Step == StepVerify {
		if err := m.advance(job, StepConnect); err != nil {
			return err
		}
	}
	if err := m.ensureReservation(job); err != nil {
		return err
	}
	if err := m.ensurePins(); err != nil {
		return err
	}
	if err := m.hostKey(job.ctx, job, t, fingerprint); err != nil {
		return err
	}
	for job.row.Step != StepDone {
		if err := m.ensureReservation(job); err != nil {
			return err
		}
		if err := m.runStep(job, t, manifest); err != nil {
			return err
		}
	}
	return nil
}

func (m *Manager) runStep(job *setupRun, t target, manifest releaseManifest) error {
	switch job.row.Step {
	case StepConnect:
		if err := m.platform(job, t); err != nil {
			return err
		}
		return m.advance(job, StepInstall)
	case StepInstall:
		if err := m.install(job, t, manifest); err != nil {
			return err
		}
		return m.advance(job, StepService)
	case StepService:
		if err := m.installService(job, t); err != nil {
			return err
		}
		if job.row.Kind == KindUpdate {
			return m.advance(job, StepForward)
		}
		return m.advance(job, StepLink)
	case StepLink:
		return m.link(job, t)
	case StepForward:
		if err := m.connectComputer(job, t); err != nil {
			return err
		}
		return m.advance(job, StepDone)
	default:
		return errInterrupted
	}
}

func (m *Manager) advance(job *setupRun, step string) error {
	previous := job.row.Step
	job.row.Step = step
	job.row.ErrorCode, job.row.ErrorDetail = "", ""
	if err := m.persist(job.ctx, &job.row); err != nil {
		job.row.Step = previous
		return err
	}
	m.logger.InfoContext(job.ctx, "computers.setup_step", "setup_id", job.row.ID, "step", previous, "result", "done")
	m.mu.Lock()
	defer m.mu.Unlock()
	m.publish(job, runningStatus(job.row))
	return nil
}

func (m *Manager) remote(
	job *setupRun, t target, command string, stdin io.Reader, budget time.Duration,
) (commandResult, error) {
	ctx, cancel := context.WithTimeout(job.ctx, budget)
	defer cancel()
	env, stop, err := serveAskpass(ctx, m.ask(job))
	if err != nil {
		return commandResult{}, err
	}
	defer stop()
	return m.ssh.run(ctx, commandArgs(t, m.pins, command), stdin, env)
}

const findCompozy = "if test -x \"$HOME/.compozy/bin/compozy\"; then printf '%s\\n' \"$HOME/.compozy/bin/compozy\"; " +
	"else command -v compozy || :; fi"

const installedCompozy = "\"$HOME/.compozy/bin/compozy\""

func (m *Manager) platform(job *setupRun, t target) error {
	result, err := m.remote(job, t, "uname -sm", nil, commandBudget)
	if err != nil {
		return sshFailure(t, result, err)
	}
	job.row.Platform, err = parseUname(string(result.stdout))
	return err
}

func (m *Manager) install(job *setupRun, t target, manifest releaseManifest) error {
	if job.row.Kind == KindAdd {
		existing, err := m.compatibleBinary(job, t)
		if err != nil {
			return err
		}
		if existing != "" {
			job.row.BinaryPath = existing
			return nil
		}
	}
	command, err := manifest.installCommand(job.row.Platform)
	if err != nil {
		return err
	}
	result, err := m.remote(job, t, command, strings.NewReader(installScript), installBudget)
	if err := installFailure(t, result, err); err != nil {
		return err
	}
	job.row.BinaryPath = ""
	return nil
}

func (m *Manager) compatibleBinary(job *setupRun, t target) (string, error) {
	result, err := m.remote(job, t, findCompozy, nil, commandBudget)
	if err != nil {
		return "", sshFailure(t, result, err)
	}
	binary := strings.TrimSpace(string(result.stdout))
	if !strings.HasPrefix(binary, "/") || strings.ContainsAny(binary, "\x00\r\n") {
		return "", nil
	}
	result, err = m.remote(job, t, shellQuote(binary)+" version --json", nil, commandBudget)
	if err != nil {
		if result.code == 255 {
			return "", sshFailure(t, result, err)
		}
		return "", nil
	}
	var installed struct {
		Protocol remoteproto.ProtocolRange `json:"remote_protocol"`
	}
	if err := json.Unmarshal(result.stdout, &installed); err != nil {
		return "", nil
	}
	if _, ok := remoteproto.CurrentProtocol.Overlap(installed.Protocol); ok {
		return binary, nil
	}
	return "", nil
}

func (m *Manager) installService(job *setupRun, t target) error {
	result, err := m.remote(job, t, binaryCommand(job)+" service install --json", nil, commandBudget)
	if err != nil {
		return sshFailure(t, result, err)
	}
	var status service.Result
	if err := json.Unmarshal(result.stdout, &status); err != nil {
		return errInterrupted
	}
	if status.Warning == service.NoManager {
		return failure(http.StatusUnprocessableEntity, service.NoManager,
			"This computer has no supported user service manager.")
	}
	if !status.Running {
		return errInterrupted
	}
	job.row.ServiceWarning = status.Warning
	return nil
}

func binaryCommand(job *setupRun) string {
	if job.row.BinaryPath != "" {
		return shellQuote(job.row.BinaryPath)
	}
	return installedCompozy
}

func describeFailure(err error) *Failure {
	if fault, ok := errors.AsType[*Failure](err); ok {
		return fault
	}
	if fault, ok := errors.AsType[*cloudclient.Failure](err); ok {
		return &Failure{HTTPStatus: fault.Status, Code: fault.Code, Message: fault.Message}
	}
	return errInterrupted
}
