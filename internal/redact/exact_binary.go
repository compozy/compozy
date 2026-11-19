package redact

import (
	"bytes"
	"cmp"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/url"
	"slices"
	"strings"
)

// ExactBytes removes registered secrets from an opaque byte payload.
func ExactBytes(value []byte) []byte {
	if value == nil {
		return nil
	}
	redacted := replaceRegisteredSecrets(string(value), dynamicSecrets.valuesSnapshot())
	if redacted == string(value) {
		return bytes.Clone(value)
	}
	return []byte(redacted)
}

// ExactBinaryJSON removes literal secrets and whole JSON strings whose decoded bytes contain a registered secret.
func ExactBinaryJSON(raw json.RawMessage) (json.RawMessage, error) {
	return RegisteredSecretSnapshot().ExactBinaryJSON(raw)
}

// ExactBinaryString removes literal and reversibly encoded registered secrets from text.
func ExactBinaryString(value string) string {
	return RegisteredSecretSnapshot().ExactBinaryString(value)
}

// ExactBinaryJSON redacts keys and string values against this fixed secret set.
func (s *SecretSnapshot) ExactBinaryJSON(raw json.RawMessage) (json.RawMessage, error) {
	return exactJSON(raw, s.ExactBinaryString)
}

// NeedsBinaryJSONRedaction reports whether any JSON string requires redaction under this snapshot.
func (s *SecretSnapshot) NeedsBinaryJSONRedaction(raw json.RawMessage) (bool, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		if value, ok := token.(string); ok && s.ExactBinaryString(value) != value {
			return true, nil
		}
	}
}

// ExactBinaryString redacts text against this fixed secret set.
func (s *SecretSnapshot) ExactBinaryString(value string) string {
	changed := func(decoded []byte) bool {
		text := string(decoded)
		return replaceRegisteredSecrets(text, s.values) != text
	}
	if exactDataURIMatches(value, changed) ||
		exactURLEncodedMatches(value, changed) ||
		exactEncodedBytesMatch(value, changed) {
		return Marker
	}
	containsMarker := func(decoded []byte) bool {
		return bytes.Contains(decoded, []byte(Marker))
	}
	if exactDataURIMatches(value, containsMarker) ||
		exactURLEncodedMatches(value, containsMarker) ||
		exactEncodedBytesMatch(value, containsMarker) {
		return value
	}
	redacted := replaceRegisteredSecrets(value, s.encodings)
	return exactRedactStringWithSecrets(redacted, s.values)
}

func secretEncodings(secrets []string) []string {
	unique := make(map[string]struct{})
	for _, secret := range secrets {
		secretBytes := []byte(secret)
		for _, encoding := range []*base64.Encoding{
			base64.StdEncoding,
			base64.RawStdEncoding,
			base64.URLEncoding,
			base64.RawURLEncoding,
		} {
			unique[encoding.EncodeToString(secretBytes)] = struct{}{}
		}
		hexSecret := hex.EncodeToString(secretBytes)
		unique[hexSecret] = struct{}{}
		unique[strings.ToUpper(hexSecret)] = struct{}{}
	}
	encoded := make([]string, 0, len(unique))
	for value := range unique {
		if value != "" {
			encoded = append(encoded, value)
		}
	}
	slices.SortFunc(encoded, func(a, b string) int {
		return cmp.Or(cmp.Compare(len(b), len(a)), cmp.Compare(a, b))
	})
	return encoded
}

func exactDataURIMatches(value string, match func([]byte) bool) bool {
	const prefix = "data:"
	if len(value) < len(prefix) || !strings.EqualFold(value[:len(prefix)], prefix) {
		return false
	}
	header, payload, found := strings.Cut(value, ",")
	if !found {
		return false
	}
	if strings.Contains(strings.ToLower(header), ";base64") {
		return exactEncodedBytesMatch(payload, match)
	}
	decoded, err := url.PathUnescape(payload)
	return err == nil && match([]byte(decoded))
}

func exactURLEncodedMatches(value string, match func([]byte) bool) bool {
	for _, plusAsSpace := range []bool{false, true} {
		decoded, changed := decodeURLEncodedBytes(value, plusAsSpace)
		if changed && match(decoded) {
			return true
		}
	}
	return false
}

func decodeURLEncodedBytes(value string, plusAsSpace bool) ([]byte, bool) {
	decoded := make([]byte, 0, len(value))
	changed := false
	for index := 0; index < len(value); {
		if value[index] == '%' && index+2 < len(value) {
			high, highOK := hexNibble(value[index+1])
			low, lowOK := hexNibble(value[index+2])
			if highOK && lowOK {
				decoded = append(decoded, high<<4|low)
				index += 3
				changed = true
				continue
			}
		}
		if plusAsSpace && value[index] == '+' {
			decoded = append(decoded, ' ')
			index++
			changed = true
			continue
		}
		decoded = append(decoded, value[index])
		index++
	}
	return decoded, changed
}

func hexNibble(value byte) (byte, bool) {
	switch {
	case value >= '0' && value <= '9':
		return value - '0', true
	case value >= 'a' && value <= 'f':
		return value - 'a' + 10, true
	case value >= 'A' && value <= 'F':
		return value - 'A' + 10, true
	default:
		return 0, false
	}
}

func exactEncodedBytesMatch(value string, match func([]byte) bool) bool {
	for _, encoding := range []*base64.Encoding{
		base64.StdEncoding,
		base64.RawStdEncoding,
		base64.URLEncoding,
		base64.RawURLEncoding,
	} {
		decoded, err := encoding.DecodeString(value)
		if err == nil && match(decoded) {
			return true
		}
	}
	decoded, err := hex.DecodeString(value)
	return err == nil && match(decoded)
}
