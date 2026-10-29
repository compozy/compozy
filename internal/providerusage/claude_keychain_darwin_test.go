//go:build darwin

package providerusage

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/procutil"
)

func TestClaudeKeychainCommand(t *testing.T) {
	t.Run("Should fail closed unless metadata explicitly admits the read", func(t *testing.T) {
		t.Parallel()
		for _, data := range []string{`{`, `{}`, `{"status":0}`, `{"status":0,"safe":false}`, `{"status":-25293,"safe":true}`} {
			if !errors.Is(decodeClaudeKeychainPreflight([]byte(data)), os.ErrPermission) {
				t.Fatalf("preflight admitted %s", data)
			}
		}
		requireNoError(t, decodeClaudeKeychainPreflight([]byte(`{"status":0,"safe":true}`)))
		if !errors.Is(decodeClaudeKeychainPreflight([]byte(`{"status":-25300}`)), os.ErrNotExist) {
			t.Fatal("missing item was not classified")
		}
	})
	t.Run("Should gate native metadata before any secret read", func(t *testing.T) {
		t.Parallel()
		for _, tc := range []struct {
			name      string
			flags     int
			partition bool
			wantReads int
		}{
			{"trusted metadata", 0, true, 1},
			{"confirmation required", 1, true, 0},
			{"missing apple-tool partition", 0, false, 0},
		} {
			t.Run("Should evaluate "+tc.name, func(t *testing.T) {
				t.Parallel()
				reads := 0
				reader := claudeExecKeychain{run: func(ctx context.Context, cmd *exec.Cmd) ([]byte, error) {
					if cmd.Path == "/usr/bin/security" {
						reads++
						return []byte("synthetic"), nil
					}
					// Substitute only Security/Foundation I/O, executing the production gate unchanged.
					script, err := json.Marshal(claudeKeychainPreflightScript)
					requireNoError(t, err)
					fixture := fmt.Sprintf(claudePreflightMetadataFixture, script, tc.flags, tc.partition)
					probe := exec.CommandContext(ctx, "/usr/bin/osascript", "-l", "JavaScript", "-e", fixture)
					return runClaudeKeychainCommand(ctx, probe)
				}}
				data, err := reader.Read(t.Context(), "service", "account", false)
				defer clear(data)
				if tc.wantReads == 0 {
					if !errors.Is(err, os.ErrPermission) {
						t.Fatalf("got %v, want denied", err)
					}
				} else {
					requireNoError(t, err)
					requireEqual(t, "synthetic", string(data))
				}
				requireEqual(t, tc.wantReads, reads)
			})
		}
	})

	t.Run("Should preserve lookup arguments without interpreting shell syntax", func(t *testing.T) {
		t.Parallel()
		command := claudeKeychainCommand(t.Context(), "service' ; injected", "account")
		requireEqual(t, "/usr/bin/security", command.Path)
		requireEqual(t, "service' ; injected", command.Args[3])
		requireEqual(t, "account", command.Args[5])
		requireEqual(t, "-w", command.Args[6])
	})
	t.Run("Should never launch a secret read after a rejected or failed preflight", func(t *testing.T) {
		t.Parallel()
		for _, failed := range []bool{false, true} {
			calls := 0
			reader := claudeExecKeychain{run: func(_ context.Context, cmd *exec.Cmd) ([]byte, error) {
				calls++
				requireEqual(t, "/usr/bin/osascript", cmd.Path)
				if failed {
					return nil, errors.New("sensitive upstream diagnostic")
				}
				return []byte(`{"status":0,"safe":false}`), nil
			}}
			_, err := reader.Read(t.Context(), "service", "account", false)
			if !errors.Is(err, os.ErrPermission) {
				t.Fatalf("got %v", err)
			}
			requireEqual(t, 1, calls)
		}
	})
	t.Run("Should read with security after approval and bypass preflight only for explicit intent", func(t *testing.T) {
		t.Parallel()
		for _, interactive := range []bool{false, true} {
			calls := 0
			reader := claudeExecKeychain{run: func(ctx context.Context, cmd *exec.Cmd) ([]byte, error) {
				calls++
				if cmd.Path == "/usr/bin/osascript" {
					return []byte(`{"status":0,"safe":true}`), nil
				}
				requireEqual(t, "/usr/bin/security", cmd.Path)
				if _, ok := ctx.Deadline(); !ok {
					t.Fatal("missing subprocess deadline")
				}
				return []byte("synthetic\n"), nil
			}}
			data, err := reader.Read(t.Context(), "service", "account", interactive)
			requireNoError(t, err)
			requireEqual(t, "synthetic", string(data))
			want := 2
			if interactive {
				want = 1
			}
			requireEqual(t, want, calls)
		}
	})
	t.Run("Should sanitize subprocess errors and distinguish OS denial from missing credentials", func(t *testing.T) {
		t.Parallel()
		for _, tc := range []struct {
			code int
			want error
		}{{44, os.ErrNotExist}, {36, os.ErrPermission}, {51, os.ErrPermission}, {128, os.ErrPermission}} {
			cmd := exec.CommandContext(t.Context(), "/bin/sh", "-c", "exit \"$1\"", "sh", strconv.Itoa(tc.code))
			exitErr := cmd.Run()
			reader := claudeExecKeychain{
				run: func(context.Context, *exec.Cmd) ([]byte, error) { return []byte("secret"), exitErr },
			}
			_, err := reader.Read(t.Context(), "service", "account", true)
			if !errors.Is(err, tc.want) {
				t.Fatalf("code %d: got %v", tc.code, err)
			}
		}
		reader := claudeExecKeychain{
			run: func(context.Context, *exec.Cmd) ([]byte, error) { return nil, errors.New("secret") },
		}
		_, err := reader.Read(t.Context(), "service", "account", true)
		requireEqual(t, ErrorCodeProviderUnavailable, AsUsageError(err, "Claude").Code)
		if strings.Contains(err.Error(), "secret") {
			t.Fatal("upstream output escaped")
		}
	})
	t.Run("Should terminate a timed out command and return no partial secret", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
		defer cancel()
		cmd := exec.CommandContext(ctx, "/bin/sh", "-c", "printf synthetic; sleep 20 & wait")
		data, err := runClaudeKeychainCommand(ctx, cmd)
		if err == nil || len(data) != 0 {
			t.Fatal("timed out command returned data")
		}
		if !errors.Is(ctx.Err(), context.DeadlineExceeded) {
			t.Fatalf("deadline: %v", ctx.Err())
		}
		if cmd.ProcessState == nil {
			t.Fatal("process was not reaped")
		}
		requireNoError(t, procutil.WaitForCommandProcessGroupExit(cmd, time.Second))
	})
	t.Run("Should classify a background timeout as consent recovery without exposing output", func(t *testing.T) {
		t.Parallel()
		reader := claudeExecKeychain{run: func(ctx context.Context, cmd *exec.Cmd) ([]byte, error) {
			if cmd.Path == "/usr/bin/osascript" {
				return []byte(`{"status":0,"safe":true}`), nil
			}
			<-ctx.Done()
			return []byte("secret"), ctx.Err()
		}}
		_, err := reader.Read(t.Context(), "service", "account", false)
		usage := AsUsageError(err, "Claude")
		requireEqual(t, ErrorCodeKeychainReadTimeout, usage.Code)
		requireEqual(t, StatusNeedsConsent, usage.Status())
		requireEqual(t, string(ActionRefresh), usage.Action)
		requireEqual(t, true, explicitRecovery("claude", usage))
	})
	t.Run("Should bound captured secret output", func(t *testing.T) {
		t.Parallel()
		output := &claudeSecretBuffer{}
		_, err := output.Write(make([]byte, MaxResponseBytes+1))
		requireEqual(t, ErrorCodeParseError, AsUsageError(err, "Claude").Code)
		requireEqual(t, 0, len(output.data))
	})
}

// This fixture supplies native API responses, not the final safe/unsafe decision.
// The positive control ensures a broken bridge fixture cannot make rejection cases pass.
const claudePreflightMetadataFixture = `function run() {
  var script = %s, confirmationFlags = %d, hasPartition = %t;
  function array(values) { return {count:values.length, objectAtIndex:function(i) { return values[i]; }}; }
  function identity(value) { return value; }
  var objc = {import:function(){}, bindFunction:function(){}, castRefToObject:identity, deepUnwrap:identity, unwrap:identity};
  var api = function(value) { return {dataUsingEncoding:function() { return value; }}; };
  api.NSMutableDictionary = {alloc:{init:{setObjectForKey:function(){}}}};
  api.NSNumber = {numberWithBool:identity};
  var interactionAllowed = true;
  api.SecKeychainGetUserInteractionAllowed = function(result) { result[0] = interactionAllowed; return 0; };
  api.SecKeychainSetUserInteractionAllowed = function(allowed) { interactionAllowed = allowed; return 0; };
  api.SecItemCopyMatching = function(query, result) {
    if (interactionAllowed) throw Error('metadata lookup allowed interaction');
    result[0] = {}; return 0;
  };
  api.SecKeychainItemCopyKeychain = function(item, result) { result[0] = {}; return 0; };
  api.SecKeychainGetStatus = function(keychain, result) { result[0] = 1; return 0; };
  api.SecKeychainItemCopyAccess = function(item, result) { result[0] = {}; return 0; };
  api.SecAccessCopyACLList = function(access, result) { result[0] = array(['decrypt', 'partition']); return 0; };
  api.SecACLCopyAuthorizations = function(acl) { return [acl === 'decrypt' ? 'ACLAuthorizationDecrypt' : 'ACLAuthorizationPartitionID']; };
  api.SecACLCopyContents = function(acl, apps, description, flags) {
    apps[0] = array([{}]); description[0] = '00'; flags[0] = acl === 'decrypt' ? confirmationFlags : 0; return 0;
  };
  api.SecTrustedApplicationCopyData = function(app, result) { result[0] = {}; return 0; };
  api.NSString = {alloc:{initWithDataEncoding:function() { return '/usr/bin/security'; }}};
  api.SecTrustedApplicationValidateWithPath = function() { return 0; };
  api.NSPropertyListSerialization = {propertyListWithDataOptionsFormatError:function() {
    return {Partitions:hasPartition ? ['apple-tool:'] : ['apple:']};
  }};
  var result = Function('ObjC', '$', 'Ref', script + '; return run(["service", "account"]);')(objc, api, function() { return []; });
  if (!interactionAllowed) throw Error('interaction state was not restored');
  return result;
}`
