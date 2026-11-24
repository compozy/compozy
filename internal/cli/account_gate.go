package cli

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/compozy/compozy/internal/account"
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/version"
	"github.com/spf13/cobra"
)

func configureAccountGate(root *cobra.Command, deps commandDeps) {
	if version.SignInRequired != "true" {
		return
	}
	for _, cmd := range root.Commands() {
		if !isRemovedGatewayCommand(cmd.Name()) {
			installAccountGate(cmd, deps)
		}
	}
}

func installAccountGate(cmd *cobra.Command, deps commandDeps) {
	for _, child := range cmd.Commands() {
		installAccountGate(child, deps)
	}
	if !cmd.Runnable() {
		return
	}
	previous := cmd.PreRunE
	if previous == nil && cmd.PreRun != nil {
		fn := cmd.PreRun
		previous = func(cmd *cobra.Command, args []string) error { fn(cmd, args); return nil }
	}
	cmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		if err := checkAccountGate(cmd, deps); err != nil {
			return err
		}
		if previous != nil {
			return previous(cmd, args)
		}
		return nil
	}
}

func checkAccountGate(cmd *cobra.Command, deps commandDeps) error {
	class := accountCommandClass(cmd, deps)
	client, err := accountClientFromDeps(deps)
	if err != nil {
		return cliAccountStatusUnavailable(class)
	}
	current, err := client.ReadAccount(cmd.Context())
	if err != nil {
		return cliAccountStatusUnavailable(class)
	}
	if current.Grace != nil && current.Gate == account.GateGrace {
		if _, err := fmt.Fprintf(cmd.ErrOrStderr(),
			"CompozyOS keeps working without sign-in until %s. Sign in: compozy account login\n",
			current.Grace.Until.Local().Format("Jan 2, 15:04"),
		); err != nil {
			return err
		}
		if class == accountCommandCloud {
			return cliSignInRequired()
		}
	} else if !current.SignedIn && current.Gate != account.GateOff &&
		(class == accountCommandLocal || class == accountCommandCloud) {
		return cliSignInRequired()
	}
	if current.Source != nil && *current.Source == "token" && class == accountCommandCloud {
		return &daemonAPIError{
			statusCode: http.StatusForbidden,
			payload: contract.ErrorPayload{
				Code:  account.ErrTokenLocalOnly.Code,
				Error: account.ErrTokenLocalOnly.Message,
			},
		}
	}
	return nil
}

func cliAccountStatusUnavailable(class cliAccountClass) error {
	if class == accountCommandBootstrap || class == accountCommandContinuation {
		return nil
	}
	return cliSignInRequired()
}

func handOffInstallToSignIn(cmd *cobra.Command, deps commandDeps) error {
	if version.SignInRequired != "true" {
		return nil
	}
	if !deps.inputIsTerminal(cmd.InOrStdin()) {
		_, err := fmt.Fprintln(cmd.ErrOrStderr(), "Next: compozy account login")
		return err
	}
	client, err := accountClientFromDeps(deps)
	if err != nil {
		return err
	}
	status, err := client.ReadAccount(cmd.Context())
	if err == nil && status.SignedIn {
		return nil
	}
	return runAccountLogin(cmd, deps, client, accountLoginOptions{})
}

type cliAccountClass uint8

const (
	accountCommandLocal cliAccountClass = iota
	accountCommandBootstrap
	accountCommandCloud
	accountCommandContinuation
)

func accountCommandClass(cmd *cobra.Command, deps commandDeps) cliAccountClass {
	path := strings.TrimPrefix(cmd.CommandPath(), cmd.Root().Name()+" ")
	switch path {
	case "account login", "account status", "account logout", "install", "version", "help",
		"service install", "service uninstall", "service status",
		"remote status", "daemon start", "daemon stop", "daemon restart", "daemon status", "daemon bootstrap":
		return accountCommandBootstrap
	case "me", "notify", "task start", "task heartbeat", "task complete", "task fail", "task release":
		if credentials := agentCredentialsFromEnv(deps); credentials.SessionID != "" &&
			credentials.AgentName != "" {
			return accountCommandContinuation
		}
	case "remote link":
		if stdin, err := cmd.Flags().GetBool("token-stdin"); err == nil && stdin {
			return accountCommandBootstrap
		}
		return accountCommandCloud
	}
	switch root, _, _ := strings.Cut(path, " "); root {
	case "update":
		return accountCommandBootstrap
	case "account", "computer", "remote":
		return accountCommandCloud
	case "daemon", "__internal":
		if cmd.Hidden {
			return accountCommandBootstrap
		}
	}
	return accountCommandLocal
}

func cliSignInRequired() error {
	return &daemonAPIError{statusCode: http.StatusUnauthorized,
		payload: contract.ErrorPayload{Code: "sign_in_required", Error: accountSignInHint}}
}
