package testutil

import (
	"bufio"
	"encoding/json"
	"strings"
	"testing"
)

type SSERecord struct {
	ID    string
	Event string
	Data  []byte
}

const maxSSEScannerTokenBytes = 8 * 1024 * 1024

func DecodeSSEData(t *testing.T, record SSERecord, dest any) {
	t.Helper()

	if err := json.Unmarshal(record.Data, dest); err != nil {
		t.Fatalf("json.Unmarshal(sse data) error = %v; data_len=%d", err, len(record.Data))
	}
}

func ParseSSE(t *testing.T, body string) []SSERecord {
	t.Helper()

	scanner := bufio.NewScanner(strings.NewReader(body))
	scanner.Buffer(make([]byte, 0, 64*1024), maxSSEScannerTokenBytes)
	records := make([]SSERecord, 0)
	current := SSERecord{}

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			if !isEmptySSERecord(current) {
				records = append(records, current)
			}
			current = SSERecord{}
			continue
		}

		field, value, ok := parseSSEFieldLine(line)
		if !ok {
			continue
		}

		switch field {
		case "id":
			current.ID = value
		case "event":
			current.Event = value
		case "data":
			if len(current.Data) > 0 {
				current.Data = append(current.Data, '\n')
			}
			current.Data = append(current.Data, []byte(value)...)
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scanner.Err() = %v", err)
	}
	if !isEmptySSERecord(current) {
		records = append(records, current)
	}

	return records
}

func parseSSEFieldLine(line string) (string, string, bool) {
	index := strings.IndexByte(line, ':')
	if index <= 0 {
		return "", "", false
	}
	value := strings.TrimPrefix(line[index+1:], " ")
	return line[:index], value, true
}

func isEmptySSERecord(record SSERecord) bool {
	return record.ID == "" && record.Event == "" && len(record.Data) == 0
}
