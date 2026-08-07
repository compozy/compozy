package cli

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/fileutil"
	"golang.org/x/crypto/scrypt"
)

const (
	gatewayProfileBundleVersion   = 1
	gatewayProfileBundlePrefix    = "compozy-gateway-profile-v1:"
	gatewayProfileBundleSaltBytes = 32
	gatewayProfileBundleKeyBytes  = 32
	gatewayProfileBundleScryptN   = 32768
	gatewayProfileBundleScryptR   = 8
	gatewayProfileBundleScryptP   = 1
	gatewayProfileBundleMaxBytes  = 64 * 1024
	gatewayProfilePassphraseMax   = 4 * 1024
	gatewayProfilePassphraseMin   = 16
)

type gatewayProfileBundle struct {
	Profile    compozyconfig.GatewayConnectionConfig `json:"profile"`
	Credential string                                `json:"credential"`
}

type gatewayProfileBundleEnvelope struct {
	Version    int    `json:"version"`
	Profile    string `json:"profile"`
	ScryptN    int    `json:"scrypt_n"`
	ScryptR    int    `json:"scrypt_r"`
	ScryptP    int    `json:"scrypt_p"`
	Salt       string `json:"salt"`
	Nonce      string `json:"nonce"`
	Ciphertext string `json:"ciphertext"`
}

func encryptGatewayProfileBundle(
	bundle gatewayProfileBundle,
	passphrase []byte,
	entropy io.Reader,
) ([]byte, error) {
	if err := validateGatewayProfileBundle(bundle); err != nil {
		return nil, err
	}
	if err := validateGatewayProfilePassphrase(passphrase); err != nil {
		return nil, err
	}
	if entropy == nil {
		entropy = rand.Reader
	}
	salt := make([]byte, gatewayProfileBundleSaltBytes)
	if _, err := io.ReadFull(entropy, salt); err != nil {
		return nil, fmt.Errorf("cli: generate gateway profile bundle salt: %w", err)
	}
	key, err := scrypt.Key(
		passphrase,
		salt,
		gatewayProfileBundleScryptN,
		gatewayProfileBundleScryptR,
		gatewayProfileBundleScryptP,
		gatewayProfileBundleKeyBytes,
	)
	if err != nil {
		return nil, fmt.Errorf("cli: derive gateway profile bundle key: %w", err)
	}
	aead, err := gatewayProfileBundleAEAD(key)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(entropy, nonce); err != nil {
		return nil, fmt.Errorf("cli: generate gateway profile bundle nonce: %w", err)
	}
	plaintext, err := json.Marshal(bundle)
	if err != nil {
		return nil, fmt.Errorf("cli: encode gateway profile bundle: %w", err)
	}
	ciphertext := aead.Seal(nil, nonce, plaintext, gatewayProfileBundleAAD(bundle.Profile.Name))
	envelope := gatewayProfileBundleEnvelope{
		Version: gatewayProfileBundleVersion,
		Profile: bundle.Profile.Name,
		ScryptN: gatewayProfileBundleScryptN,
		ScryptR: gatewayProfileBundleScryptR,
		ScryptP: gatewayProfileBundleScryptP,
		Salt:    base64.RawURLEncoding.EncodeToString(salt), Nonce: base64.RawURLEncoding.EncodeToString(nonce),
		Ciphertext: base64.RawURLEncoding.EncodeToString(ciphertext),
	}
	encoded, err := json.Marshal(envelope)
	if err != nil {
		return nil, fmt.Errorf("cli: encode gateway profile bundle envelope: %w", err)
	}
	return []byte(gatewayProfileBundlePrefix + base64.RawURLEncoding.EncodeToString(encoded) + "\n"), nil
}

func decryptGatewayProfileBundle(payload, passphrase []byte) (gatewayProfileBundle, error) {
	if err := validateGatewayProfilePassphrase(passphrase); err != nil {
		return gatewayProfileBundle{}, err
	}
	encoded := strings.TrimSpace(string(payload))
	if !strings.HasPrefix(encoded, gatewayProfileBundlePrefix) {
		return gatewayProfileBundle{}, errors.New("cli: gateway profile bundle version is invalid")
	}
	envelopeJSON, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(encoded, gatewayProfileBundlePrefix))
	if err != nil {
		return gatewayProfileBundle{}, fmt.Errorf("cli: decode gateway profile bundle envelope: %w", err)
	}
	var envelope gatewayProfileBundleEnvelope
	if err := json.Unmarshal(envelopeJSON, &envelope); err != nil {
		return gatewayProfileBundle{}, fmt.Errorf("cli: parse gateway profile bundle envelope: %w", err)
	}
	if envelope.Version != gatewayProfileBundleVersion || envelope.ScryptN != gatewayProfileBundleScryptN ||
		envelope.ScryptR != gatewayProfileBundleScryptR || envelope.ScryptP != gatewayProfileBundleScryptP {
		return gatewayProfileBundle{}, errors.New("cli: gateway profile bundle parameters are invalid")
	}
	salt, err := decodeGatewayProfileBundlePart("salt", envelope.Salt, gatewayProfileBundleSaltBytes)
	if err != nil {
		return gatewayProfileBundle{}, err
	}
	nonce, err := decodeGatewayProfileBundlePart("nonce", envelope.Nonce, 0)
	if err != nil {
		return gatewayProfileBundle{}, err
	}
	ciphertext, err := decodeGatewayProfileBundlePart("ciphertext", envelope.Ciphertext, 0)
	if err != nil {
		return gatewayProfileBundle{}, err
	}
	key, err := scrypt.Key(
		passphrase, salt, envelope.ScryptN, envelope.ScryptR, envelope.ScryptP, gatewayProfileBundleKeyBytes,
	)
	if err != nil {
		return gatewayProfileBundle{}, fmt.Errorf("cli: derive gateway profile bundle key: %w", err)
	}
	aead, err := gatewayProfileBundleAEAD(key)
	if err != nil {
		return gatewayProfileBundle{}, err
	}
	if len(nonce) != aead.NonceSize() {
		return gatewayProfileBundle{}, errors.New("cli: gateway profile bundle nonce is invalid")
	}
	if !validGatewayProfileName(strings.TrimSpace(envelope.Profile)) {
		return gatewayProfileBundle{}, errors.New("cli: gateway profile bundle name is invalid")
	}
	plaintext, err := aead.Open(nil, nonce, ciphertext, gatewayProfileBundleAAD(envelope.Profile))
	if err != nil {
		return gatewayProfileBundle{}, errors.New("cli: decrypt gateway profile bundle; check the passphrase")
	}
	var bundle gatewayProfileBundle
	if err := json.Unmarshal(plaintext, &bundle); err != nil {
		return gatewayProfileBundle{}, fmt.Errorf("cli: parse gateway profile bundle: %w", err)
	}
	if err := validateGatewayProfileBundle(bundle); err != nil {
		return gatewayProfileBundle{}, err
	}
	if bundle.Profile.Name != envelope.Profile {
		return gatewayProfileBundle{}, errors.New("cli: gateway profile bundle name does not match its envelope")
	}
	return bundle, nil
}

func gatewayProfileBundleAAD(profile string) []byte {
	return fmt.Appendf(nil, "compozy:gateway-profile:%d:%s", gatewayProfileBundleVersion, strings.TrimSpace(profile))
}

func gatewayProfileBundleAEAD(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("cli: initialize gateway profile bundle cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("cli: initialize gateway profile bundle AEAD: %w", err)
	}
	return aead, nil
}

func validateGatewayProfileBundle(bundle gatewayProfileBundle) error {
	if bundle.Profile.Scheme != gatewaySchemeHTTPS {
		return errors.New("cli: only HTTPS gateway profiles can be transferred")
	}
	if err := bundle.Profile.Validate(); err != nil {
		return fmt.Errorf("cli: validate gateway profile bundle metadata: %w", err)
	}
	if !validGatewayDeviceCredential(strings.TrimSpace(bundle.Credential)) {
		return errors.New("cli: gateway profile bundle credential is invalid")
	}
	return nil
}

func validateGatewayProfilePassphrase(passphrase []byte) error {
	length := len(passphrase)
	if length < gatewayProfilePassphraseMin || length > gatewayProfilePassphraseMax {
		return fmt.Errorf(
			"cli: gateway profile passphrase must be between %d and %d bytes",
			gatewayProfilePassphraseMin,
			gatewayProfilePassphraseMax,
		)
	}
	return nil
}

func decodeGatewayProfileBundlePart(name, encoded string, exactBytes int) ([]byte, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(encoded))
	if err != nil || len(decoded) == 0 || exactBytes > 0 && len(decoded) != exactBytes {
		return nil, fmt.Errorf("cli: gateway profile bundle %s is invalid", name)
	}
	return decoded, nil
}

func readPrivateBoundedFile(path string, maximum int64, label string) (payload []byte, err error) {
	file, err := fileutil.OpenRegularFile(strings.TrimSpace(path))
	if err != nil {
		return nil, fmt.Errorf("cli: open %s: %w", label, err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("cli: close %s: %w", label, closeErr))
		}
	}()
	info, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("cli: inspect %s: %w", label, err)
	}
	if info.Mode().Perm()&0o077 != 0 {
		return nil, fmt.Errorf("cli: %s permissions must not grant group or other access", label)
	}
	reader := io.LimitReader(file, maximum+1)
	payload, err = io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("cli: read %s: %w", label, err)
	}
	if int64(len(payload)) > maximum {
		return nil, fmt.Errorf("cli: %s exceeds %d bytes", label, maximum)
	}
	return payload, nil
}

func writePrivateNewFile(path string, payload []byte, overwrite bool) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return errors.New("cli: gateway profile bundle output path is required")
	}
	if _, err := os.Lstat(path); err == nil {
		if !overwrite {
			return fmt.Errorf("cli: gateway profile bundle %q already exists; pass --overwrite to replace it", path)
		}
		if _, _, err := fileutil.ReadRegularFile(path); err != nil {
			return fmt.Errorf("cli: inspect existing gateway profile bundle: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("cli: inspect gateway profile bundle output: %w", err)
	}
	if err := fileutil.AtomicWriteFile(path, payload, 0o600); err != nil {
		return fmt.Errorf("cli: write gateway profile bundle: %w", err)
	}
	return nil
}
