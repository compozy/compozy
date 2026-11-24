package account

import (
	"cmp"
	"errors"
	"os"
)

const TokenEnv = "COMPOZY_ACCOUNT_TOKEN" // #nosec G101 -- environment variable name.

// TakeLauncherToken consumes the one-shot credential before the daemon can
// start providers, terminals, hooks, or extensions.
func TakeLauncherToken(internal string) (string, error) {
	token := cmp.Or(os.Getenv(internal), os.Getenv(TokenEnv))
	if err := errors.Join(os.Unsetenv(internal), os.Unsetenv(TokenEnv)); err != nil {
		return "", err
	}
	return token, nil
}
