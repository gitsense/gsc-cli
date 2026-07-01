/**
 * Component: Rules Canonical Store
 * Block-UUID: e5f6a7b8-c9d0-1234-efab-345678901234
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Reads, appends, rewrites, and deletes committed rule records in the canonical JSONL store.
 * Language: Go
 * Created-at: 2026-06-20T19:00:00Z
 * Authors: MiMo-v2.5-pro (v1.0.0)
 */


package rules

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func LoadRecords() ([]Rule, error) {
	path, err := RecordsPath()
	if err != nil {
		return nil, err
	}
	return LoadRecordsFromPath(path, true)
}

func LoadRecordsFromPath(path string, allowMissing bool) ([]Rule, error) {
	file, err := os.Open(path)
	if allowMissing && os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return LoadRecordsFromReader(file)
}

func LoadRecordsFromReader(reader io.Reader) ([]Rule, error) {
	var records []Rule
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var record Rule
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			return nil, fmt.Errorf("invalid records.jsonl entry: %w", err)
		}
		// Normalize topics for backward compatibility
		record.NormalizeTopics()
		records = append(records, record)
	}
	return records, scanner.Err()
}

func AppendRecord(record Rule) error {
	path, err := RecordsPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer file.Close()
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}
	if _, err := file.Write(append(data, '\n')); err != nil {
		return err
	}
	return nil
}

func DeleteRecord(id string) (bool, error) {
	records, err := LoadRecords()
	if err != nil {
		return false, err
	}
	var kept []Rule
	deleted := false
	for _, record := range records {
		if record.ID == id {
			deleted = true
			continue
		}
		kept = append(kept, record)
	}
	if !deleted {
		return false, nil
	}
	return true, WriteRecords(kept)
}

func WriteRecords(records []Rule) error {
	path, err := RecordsPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	for _, record := range records {
		data, err := json.Marshal(record)
		if err != nil {
			return err
		}
		if _, err := file.Write(append(data, '\n')); err != nil {
			return err
		}
	}
	return nil
}
