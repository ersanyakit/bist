package audit

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type Event struct {
	ID        string         `json:"id"`
	Time      time.Time      `json:"time"`
	Actor     string         `json:"actor"`
	Action    string         `json:"action"`
	Entity    string         `json:"entity"`
	EntityID  string         `json:"entity_id,omitempty"`
	Details   map[string]any `json:"details,omitempty"`
	PrevHash  string         `json:"prev_hash,omitempty"`
	EventHash string         `json:"event_hash"`
}

type VerificationReport struct {
	Status    string    `json:"status"`
	Events    int       `json:"events"`
	LastHash  string    `json:"last_hash,omitempty"`
	Errors    []string  `json:"errors,omitempty"`
	CheckedAt time.Time `json:"checked_at"`
}

func Append(path string, event Event) (Event, error) {
	if path == "" {
		return Event{}, errors.New("audit log path is required")
	}
	if event.Time.IsZero() {
		event.Time = time.Now().UTC()
	}
	if event.Actor == "" {
		event.Actor = "system"
	}
	if event.Action == "" || event.Entity == "" {
		return Event{}, errors.New("audit event action and entity are required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return Event{}, err
	}
	event.PrevHash = lastHash(path)
	event.ID = eventID(event)
	event.EventHash = hashEvent(event)
	line, err := json.Marshal(event)
	if err != nil {
		return Event{}, err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o640)
	if err != nil {
		return Event{}, err
	}
	defer file.Close()
	if _, err := file.Write(append(line, '\n')); err != nil {
		return Event{}, err
	}
	if err := file.Sync(); err != nil {
		return Event{}, err
	}
	return event, nil
}

func Verify(path string) (VerificationReport, error) {
	report := VerificationReport{Status: "pass", CheckedAt: time.Now().UTC()}
	if path == "" {
		return report, errors.New("audit log path is required")
	}
	file, err := os.Open(path)
	if err != nil {
		return report, err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	lineNo := 0
	prevHash := ""
	for scanner.Scan() {
		lineNo++
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var event Event
		if err := json.Unmarshal(line, &event); err != nil {
			report.Errors = append(report.Errors, fmt.Sprintf("line_%d_json_invalid", lineNo))
			continue
		}
		if event.PrevHash != prevHash {
			report.Errors = append(report.Errors, fmt.Sprintf("line_%d_prev_hash_mismatch", lineNo))
		}
		if event.EventHash == "" {
			report.Errors = append(report.Errors, fmt.Sprintf("line_%d_event_hash_missing", lineNo))
		} else if got := hashEvent(event); got != event.EventHash {
			report.Errors = append(report.Errors, fmt.Sprintf("line_%d_event_hash_mismatch", lineNo))
		}
		prevHash = event.EventHash
		report.LastHash = event.EventHash
		report.Events++
	}
	if err := scanner.Err(); err != nil {
		return report, err
	}
	if report.Events == 0 {
		report.Errors = append(report.Errors, "audit_log_empty")
	}
	if len(report.Errors) > 0 {
		report.Status = "fail"
	}
	return report, nil
}

func eventID(event Event) string {
	raw := event.Action + "|" + event.Entity + "|" + event.EntityID + "|" + event.Time.Format(time.RFC3339Nano)
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])[:24]
}

func hashEvent(event Event) string {
	cp := event
	cp.EventHash = ""
	raw, _ := json.Marshal(cp)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func lastHash(path string) string {
	raw, err := os.ReadFile(path)
	if err != nil || len(raw) == 0 {
		return ""
	}
	lines := splitLines(raw)
	for i := len(lines) - 1; i >= 0; i-- {
		if len(lines[i]) == 0 {
			continue
		}
		var event Event
		if json.Unmarshal(lines[i], &event) == nil {
			return event.EventHash
		}
	}
	return ""
}

func splitLines(raw []byte) [][]byte {
	lines := [][]byte{}
	start := 0
	for i, b := range raw {
		if b == '\n' {
			lines = append(lines, raw[start:i])
			start = i + 1
		}
	}
	if start < len(raw) {
		lines = append(lines, raw[start:])
	}
	return lines
}
