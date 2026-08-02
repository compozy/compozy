package providers

import (
	"crypto/hmac"
	cryptorand "crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"hash"
	"strconv"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/subprocess"
)

type preStartCacheHMACKey [sha256.Size]byte

type preStartCacheFingerprint [sha256.Size]byte

func newPreStartCacheHMACKey() (preStartCacheHMACKey, bool) {
	var key preStartCacheHMACKey
	if _, err := cryptorand.Read(key[:]); err != nil {
		return preStartCacheHMACKey{}, false
	}
	return key, true
}

func preStartSemanticFingerprint(
	cacheKey preStartCacheHMACKey,
	provider compozyconfig.ProviderConfig,
	env ProbeEnv,
) (preStartCacheFingerprint, bool) {
	mac := hmac.New(sha256.New, cacheKey[:])
	slots := provider.EffectiveCredentialSlots()
	launchResolution := env.launchResolution()
	for _, field := range [][2]string{
		{"provider_command", strings.TrimSpace(provider.Command)},
		{"auth_status_command", strings.TrimSpace(provider.AuthStatusCmd)},
		{"auth_login_command", strings.TrimSpace(provider.AuthLoginCmd)},
		{"credential_slot_count", strconv.Itoa(len(slots))},
		{"command_env_count", strconv.Itoa(len(env.CommandEnv))},
		{"command_dir", env.CommandDir},
		{"launch_executable_resolution_state", string(launchResolution.State())},
		{"resolved_launch_executable", launchResolution.ResolvedExecutable()},
		{"auth_status_executable", authStatusCommandExecutable(env)},
	} {
		if !writePreStartFingerprintField(mac, field[0], field[1]) {
			return preStartCacheFingerprint{}, false
		}
	}
	for _, slot := range slots {
		for _, field := range [][2]string{
			{"credential_slot_name", strings.TrimSpace(slot.Name)},
			{"credential_slot_target_env", strings.TrimSpace(slot.TargetEnv)},
			{"credential_slot_secret_ref", strings.TrimSpace(slot.SecretRef)},
			{"credential_slot_kind", strings.TrimSpace(slot.Kind)},
			{"credential_slot_required", preStartFingerprintBool(slot.Required)},
		} {
			if !writePreStartFingerprintField(mac, field[0], field[1]) {
				return preStartCacheFingerprint{}, false
			}
		}
	}
	for _, entry := range env.CommandEnv {
		if !writePreStartFingerprintField(mac, "command_env", entry) {
			return preStartCacheFingerprint{}, false
		}
	}
	for _, key := range []string{"PATH", "PATHEXT"} {
		value, present := subprocess.LookupEnv(env.CommandEnv, key)
		if !writePreStartFingerprintField(mac, "command_env_"+key+"_present", preStartFingerprintBool(present)) ||
			!writePreStartFingerprintField(mac, "command_env_"+key, value) {
			return preStartCacheFingerprint{}, false
		}
	}
	var fingerprint preStartCacheFingerprint
	copy(fingerprint[:], mac.Sum(nil))
	return fingerprint, true
}

func authStatusCommandExecutable(env ProbeEnv) string {
	resolution := env.authStatusCommandResolution
	if resolution == nil || resolution.state != authStatusCommandResolutionResolved {
		return ""
	}
	return resolution.spec.Executable
}

func writePreStartFingerprintField(mac hash.Hash, name string, value string) bool {
	return writePreStartFingerprintPart(mac, name) && writePreStartFingerprintPart(mac, value)
}

func writePreStartFingerprintPart(mac hash.Hash, value string) bool {
	var length [8]byte
	binary.BigEndian.PutUint64(length[:], uint64(len(value)))
	if !writePreStartFingerprintBytes(mac, length[:]) {
		return false
	}
	return writePreStartFingerprintBytes(mac, []byte(value))
}

func writePreStartFingerprintBytes(mac hash.Hash, value []byte) bool {
	written, err := mac.Write(value)
	return err == nil && written == len(value)
}

func preStartFingerprintBool(value bool) string {
	if value {
		return "true"
	}
	return "false"
}
