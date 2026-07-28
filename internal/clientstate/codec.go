package clientstate

import (
	"encoding/binary"
	"errors"
	"fmt"
	"time"
)

const (
	envelopeHeaderSize = 25
	tombstoneFlag      = byte(1)
	maxSafeJSONInteger = uint64(1<<53 - 1)
)

func encodeEntry(entry Entry) ([]byte, error) {
	if entry.Rev > maxSafeJSONInteger || entry.Seq > maxSafeJSONInteger {
		return nil, errors.New("clientstate: revision or sequence exceeds the safe JSON integer range")
	}
	encoded := make([]byte, envelopeHeaderSize+len(entry.Value))
	binary.BigEndian.PutUint64(encoded[0:8], entry.Rev)
	binary.BigEndian.PutUint64(encoded[8:16], entry.Seq)
	if entry.Deleted {
		encoded[16] = tombstoneFlag
	}
	if _, err := binary.Encode(encoded[17:25], binary.BigEndian, entry.UpdatedAt.UTC().UnixNano()); err != nil {
		return nil, fmt.Errorf("clientstate: encode updated_at: %w", err)
	}
	copy(encoded[envelopeHeaderSize:], entry.Value)
	return encoded, nil
}

func decodeEntry(domain, key string, encoded []byte) (Entry, error) {
	if len(encoded) < envelopeHeaderSize {
		return Entry{}, fmt.Errorf(
			"clientstate: corrupt envelope for %s/%s: header is %d bytes",
			domain,
			key,
			len(encoded),
		)
	}
	flags := encoded[16]
	if flags & ^tombstoneFlag != 0 {
		return Entry{}, fmt.Errorf("clientstate: corrupt envelope for %s/%s: flags=%d", domain, key, flags)
	}
	deleted := flags&tombstoneFlag != 0
	value := append([]byte(nil), encoded[envelopeHeaderSize:]...)
	if deleted && len(value) != 0 {
		return Entry{}, fmt.Errorf("clientstate: corrupt tombstone for %s/%s: payload is present", domain, key)
	}
	var updatedAtNanos int64
	if _, err := binary.Decode(encoded[17:25], binary.BigEndian, &updatedAtNanos); err != nil {
		return Entry{}, fmt.Errorf("clientstate: decode updated_at for %s/%s: %w", domain, key, err)
	}
	return Entry{
		Domain:    domain,
		Key:       key,
		Value:     value,
		Rev:       binary.BigEndian.Uint64(encoded[0:8]),
		Seq:       binary.BigEndian.Uint64(encoded[8:16]),
		Deleted:   deleted,
		UpdatedAt: time.Unix(0, updatedAtNanos).UTC(),
	}, nil
}

func encodeSequence(sequence uint64) ([]byte, error) {
	if sequence > maxSafeJSONInteger {
		return nil, errors.New("clientstate: sequence exceeds the safe JSON integer range")
	}
	encoded := make([]byte, 8)
	binary.BigEndian.PutUint64(encoded, sequence)
	return encoded, nil
}

func decodeSequence(encoded []byte) (uint64, error) {
	if len(encoded) == 0 {
		return 0, nil
	}
	if len(encoded) != 8 {
		return 0, fmt.Errorf("clientstate: corrupt workspace sequence: %d bytes", len(encoded))
	}
	sequence := binary.BigEndian.Uint64(encoded)
	if sequence > maxSafeJSONInteger {
		return 0, errors.New("clientstate: workspace sequence exceeds the safe JSON integer range")
	}
	return sequence, nil
}
