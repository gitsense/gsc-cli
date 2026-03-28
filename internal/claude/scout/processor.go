/**
 * Component: Scout Stream Event Processor
 * Block-UUID: f930b1b1-82ca-459c-844c-ba6786ea85d3
 * Parent-UUID: 9df5633b-6e78-4202-9700-040837001480
 * Version: 1.0.4
 * Description: JSONL event streaming, parsing, and file I/O for Scout sessions
 * Language: Go
 * Created-at: 2026-03-28T00:29:07.060Z
 * Authors: claude-haiku-4-5-20251001 (v1.0.0), GLM-4.7 (v1.0.1), Gemini 3 Flash (v1.0.2), GLM-4.7 (v1.0.3), GLM-4.7 (v1.0.4)
 */


package scout

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// EventWriter writes JSONL events to a stream file
type EventWriter struct {
	file   *os.File
	writer *bufio.Writer
}

// NewEventWriter creates a new event writer for a turn's log file
func NewEventWriter(logFilePath string) (*EventWriter, error) {
	file, err := os.Create(logFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create log file: %w", err)
	}

	return &EventWriter{
		file:   file,
		writer: bufio.NewWriter(file),
	}, nil
}

// WriteEvent writes a single event to the stream
func (ew *EventWriter) WriteEvent(eventType string, data interface{}) error {
	event := StreamEvent{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Type:      eventType,
		Data:      data,
	}

	eventBytes, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	if _, err := ew.writer.Write(eventBytes); err != nil {
		return fmt.Errorf("failed to write event: %w", err)
	}

	if err := ew.writer.WriteByte('\n'); err != nil {
		return fmt.Errorf("failed to write newline: %w", err)
	}

	return ew.writer.Flush()
}

// WriteInitEvent writes the initialization event
func (ew *EventWriter) WriteInitEvent(init InitEvent) error {
	return ew.WriteEvent("init", init)
}

// WriteStatusEvent writes a status/progress event
func (ew *EventWriter) WriteStatusEvent(status StatusEvent) error {
	return ew.WriteEvent("status", status)
}

// WriteCandidatesEvent writes discovered candidates
func (ew *EventWriter) WriteCandidatesEvent(candidates CandidatesEvent) error {
	return ew.WriteEvent("candidates", candidates)
}

// WriteVerifiedEvent writes verified/re-scored candidates
func (ew *EventWriter) WriteVerifiedEvent(verified VerifiedEvent) error {
	return ew.WriteEvent("verified", verified)
}

// WriteDoneEvent writes completion event
func (ew *EventWriter) WriteDoneEvent(done DoneEvent) error {
	return ew.WriteEvent("done", done)
}

// WriteErrorEvent writes error event
func (ew *EventWriter) WriteErrorEvent(errEvent ErrorEvent) error {
	return ew.WriteEvent("error", errEvent)
}

// WriteRawEvent writes a raw JSON line to the stream (for debugging/audit trail)
func (ew *EventWriter) WriteRawEvent(line string) error {
	if _, err := ew.writer.WriteString(line); err != nil {
		return fmt.Errorf("failed to write raw event: %w", err)
	}
	if err := ew.writer.WriteByte('\n'); err != nil {
		return fmt.Errorf("failed to write newline: %w", err)
	}
	return ew.writer.Flush()
}

// Close closes the event writer
func (ew *EventWriter) Close() error {
	var flushErr error
	if ew.writer != nil {
		flushErr = ew.writer.Flush()
	}

	var closeErr error
	if ew.file != nil {
		closeErr = ew.file.Close()
	}

	if flushErr != nil {
		return flushErr
	}
	return closeErr
}

// EventReader reads JSONL events from a stream file
type EventReader struct {
	file   *os.File
	scanner *bufio.Scanner
}

// NewEventReader creates a new event reader from a log file
func NewEventReader(logFilePath string) (*EventReader, error) {
	file, err := os.Open(logFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	return &EventReader{
		file:    file,
		scanner: func() *bufio.Scanner {
			s := bufio.NewScanner(file)
			s.Buffer(make([]byte, 0, 64*1024), maxTokenSize)
			return s
		}(),
	}, nil
}

// ReadEvent reads the next event from the stream
func (er *EventReader) ReadEvent() (*StreamEvent, error) {
	if !er.scanner.Scan() {
		if err := er.scanner.Err(); err != nil {
			return nil, fmt.Errorf("scanner error: %w", err)
		}
		return nil, nil // EOF
	}

	var event StreamEvent
	if err := json.Unmarshal(er.scanner.Bytes(), &event); err != nil {
		return nil, fmt.Errorf("failed to unmarshal event: %w", err)
	}

	return &event, nil
}

// ReadAllEvents reads all events from the stream
func (er *EventReader) ReadAllEvents() ([]StreamEvent, error) {
	var events []StreamEvent

	for {
		event, err := er.ReadEvent()
		if err != nil {
			return events, fmt.Errorf("failed to read event: %w", err)
		}
		if event == nil {
			break
		}
		events = append(events, *event)
	}

	return events, nil
}

// Close closes the event reader
func (er *EventReader) Close() error {
	if er.file != nil {
		return er.file.Close()
	}
	return nil
}

// ProcessorHelper provides utilities for processing events and session state
type ProcessorHelper struct {
	sessionConfig *SessionConfig
}

// NewProcessorHelper creates a new processor helper
func NewProcessorHelper(sessionConfig *SessionConfig) *ProcessorHelper {
	return &ProcessorHelper{
		sessionConfig: sessionConfig,
	}
}

// GetLatestTurnLogFile returns the most recent turn log file
func (ph *ProcessorHelper) GetLatestTurnLogFile(turn int) (string, error) {
	turnDir := ph.sessionConfig.GetTurnDir(turn)
	entries, err := os.ReadDir(turnDir)
	if err != nil {
		return "", fmt.Errorf("failed to read turn directory: %w", err)
	}

	var latestFile string
	var latestTime time.Time

	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".ndjson" {
			info, err := entry.Info()
			if err != nil {
				continue
			}
			if info.ModTime().After(latestTime) {
				latestTime = info.ModTime()
				latestFile = filepath.Join(turnDir, entry.Name())
			}
		}
	}

	if latestFile == "" {
		return "", fmt.Errorf("no log files found in turn directory")
	}

	return latestFile, nil
}

// ReadSessionStatusFromEvents reconstructs status data from events
func (ph *ProcessorHelper) ReadSessionStatusFromEvents(turn int) (*StatusData, error) {
	logFile, err := ph.GetLatestTurnLogFile(turn)
	if err != nil {
		return nil, err
	}

	reader, err := NewEventReader(logFile)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	events, err := reader.ReadAllEvents()
	if err != nil {
		return nil, err
	}

	// Build status from events
	status := &StatusData{
		SessionID: ph.sessionConfig.SessionID,
		Candidates: []Candidate{},
	}

	for i, event := range events {
		// Capture start time from first event
		if i == 0 {
			if parsedTime, err := time.Parse(time.RFC3339Nano, event.Timestamp); err == nil {
				status.StartedAt = parsedTime
				status.ElapsedSeconds = int64(time.Since(parsedTime).Seconds())
			}
		}

		switch event.Type {
		case "init":
			var init InitEvent
			if data, err := json.Marshal(event.Data); err == nil {
				json.Unmarshal(data, &init)
				status.SessionID = init.SessionID
				status.WorkingDirectories = init.WorkingDirectories
			}

		case "status":
			var statusEvent StatusEvent
			if data, err := json.Marshal(event.Data); err == nil {
				json.Unmarshal(data, &statusEvent)
				status.Phase = statusEvent.Phase
			}

		case "candidates":
			var candEvent CandidatesEvent
			if data, err := json.Marshal(event.Data); err == nil {
				json.Unmarshal(data, &candEvent)
				status.Candidates = candEvent.Candidates
				status.TotalFound = candEvent.TotalFound
			}

		case "verified":
			var verifiedEvent VerifiedEvent
			if data, err := json.Marshal(event.Data); err == nil {
				json.Unmarshal(data, &verifiedEvent)
				// Update candidates with verified scores and reasoning
				for _, update := range verifiedEvent.UpdatedCandidates {
					for j, cand := range status.Candidates {
						if cand.FilePath == update.FilePath && cand.WorkdirID == update.WorkdirID {
							status.Candidates[j].Score = update.VerifiedScore
							status.Candidates[j].Reasoning = update.Reason
						}
					}
				}
			}

		case "done":
			var doneEvent DoneEvent
			if data, err := json.Marshal(event.Data); err == nil {
				json.Unmarshal(data, &doneEvent)
				status.Status = doneEvent.Status
				if parsedTime, err := time.Parse(time.RFC3339Nano, event.Timestamp); err == nil {
					status.CompletedAt = &parsedTime
				}
			}

		case "error":
			var errEvent ErrorEvent
			if data, err := json.Marshal(event.Data); err == nil {
				json.Unmarshal(data, &errEvent)
				errMsg := fmt.Sprintf("%s: %s", errEvent.ErrorCode, errEvent.Message)
				status.Error = &errMsg
			}
		}
	}

	return status, nil
}

// CopyReferenceFile copies a reference file into the session references directory
func (ph *ProcessorHelper) CopyReferenceFile(sourceFile string, refType string) error {
	data, err := os.ReadFile(sourceFile)
	if err != nil {
		return fmt.Errorf("failed to read reference file: %w", err)
	}

	destFile := ph.sessionConfig.GetReferenceFile(refType)
	if err := os.WriteFile(destFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write reference file: %w", err)
	}

	return nil
}
