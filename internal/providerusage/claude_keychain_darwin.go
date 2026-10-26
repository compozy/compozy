//go:build darwin

package providerusage

import (
	"context"
	"encoding/base64"
	"encoding/json/v2"
	"os"
	"os/exec"
	"strconv"
	"time"
)

type claudeExecKeychain struct{}

var _ KeychainReader = claudeExecKeychain{}

// The system JXA bridge supports Security.framework without requiring cgo in release binaries.
// Non-interactive reads disable both prompt paths: the LAContext flags cover data-protection items,
// while SecKeychainSetUserInteractionAllowed covers the legacy login keychain ACL dialog that
// Claude Code's item uses; without it every background poll asks for the keychain password.
const claudeKeychainScript = `
ObjC.import('Foundation');
ObjC.import('Security');
ObjC.import('LocalAuthentication');
function run(argv) {
  var c = ObjC.castRefToObject;
  var query = $.NSMutableDictionary.alloc.init;
  query.setObjectForKey(c($.kSecClassGenericPassword), c($.kSecClass));
  query.setObjectForKey($(argv[0]), c($.kSecAttrService));
  query.setObjectForKey($(argv[1]), c($.kSecAttrAccount));
  query.setObjectForKey($.NSNumber.numberWithBool(true), c($.kSecReturnData));
  if (argv[2] !== 'true') {
    $.SecKeychainSetUserInteractionAllowed(false);
    var context = $.LAContext.alloc.init;
    context.interactionNotAllowed = true;
    query.setObjectForKey(context, c($.kSecUseAuthenticationContext));
    query.setObjectForKey(c($.kSecUseAuthenticationUIFail), c($.kSecUseAuthenticationUI));
  }
  var result = Ref();
  var status = $.SecItemCopyMatching(query, result);
  var data = status === 0 ? ObjC.unwrap(c(result[0]).base64EncodedStringWithOptions(0)) : '';
  return JSON.stringify({status:status, data:data});
}`

func defaultClaudeKeychain() KeychainReader { return claudeExecKeychain{} }
func claudeKeychainCommand(ctx context.Context, service, account string, interactive bool) *exec.Cmd {
	return exec.CommandContext(ctx, "/usr/bin/osascript", "-l", "JavaScript", "-e", claudeKeychainScript,
		"--", service, account, strconv.FormatBool(interactive))
}
func (claudeExecKeychain) Read(ctx context.Context, service, account string, interactive bool) ([]byte, error) {
	timeout := 10 * time.Second
	if interactive {
		timeout = 60 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	command := claudeKeychainCommand(ctx, service, account, interactive)
	command.WaitDelay = time.Second
	output := &claudeSecretBuffer{}
	command.Stdout = output
	defer func() { clear(output.data) }()
	if err := command.Run(); err != nil {
		return nil, NewUsageError(ErrorCodeProviderUnavailable, "Claude")
	}
	return decodeClaudeKeychainResult(output.data)
}

func decodeClaudeKeychainResult(data []byte) ([]byte, error) {
	var result struct {
		Status *int   `json:"status"`
		Data   string `json:"data"`
	}
	if err := json.Unmarshal(data, &result); err != nil || result.Status == nil {
		return nil, NewUsageError(ErrorCodeParseError, "Claude")
	}
	switch *result.Status {
	case 0:
		token, err := base64.StdEncoding.DecodeString(result.Data)
		if err != nil {
			clear(token)
			return nil, NewUsageError(ErrorCodeParseError, "Claude")
		}
		return token, nil
	case -25300:
		return nil, os.ErrNotExist
	case -25308, -25293, -128:
		return nil, os.ErrPermission
	default:
		return nil, NewUsageError(ErrorCodeProviderUnavailable, "Claude")
	}
}

type claudeSecretBuffer struct{ data []byte }

func (b *claudeSecretBuffer) Write(data []byte) (int, error) {
	if len(b.data)+len(data) > MaxResponseBytes {
		return 0, NewUsageError(ErrorCodeParseError, "Claude")
	}
	b.data = append(b.data, data...)
	return len(data), nil
}
