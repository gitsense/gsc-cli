/**
 * Component: Lessons Canonical Store
 * Block-UUID: 2f108404-bf41-443e-992c-c9f321f04849
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Reads, appends, rewrites, finds, and deletes committed lesson records in the canonical JSONL store.
 * Language: Go
 * Created-at: 2026-06-12T12:44:13Z
 * Authors: Codex GPT-5 (v1.0.0)
 */

package lessons

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/gitsense/gsc-cli/pkg/netutil"
)

func LoadRecords() ([]Record, error) {
	path, err := RecordsPath()
	if err != nil {
		return nil, err
	}
	return LoadRecordsFromPath(path, true)
}

func LoadRecordsFromSource(source string) ([]Record, error) {
	if strings.TrimSpace(source) == "" {
		return nil, fmt.Errorf("lesson records source is required")
	}

	if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		tmpFile, err := netutil.DownloadToTemp(source)
		if err != nil {
			return nil, fmt.Errorf("failed to download lesson records source: %w", err)
		}
		defer os.Remove(tmpFile.Name())
		defer tmpFile.Close()
		return LoadRecordsFromReader(tmpFile)
	}

	path := source
	if strings.HasPrefix(source, "file://") {
		parsedURL, err := url.Parse(source)
		if err != nil {
			return nil, fmt.Errorf("failed to parse file:// source: %w", err)
		}
		path = parsedURL.Path
		if runtime.GOOS == "windows" && len(path) > 0 && path[0] == '/' {
			path = path[1:]
		}
		path = filepath.FromSlash(path)
	}

	return LoadRecordsFromPath(path, false)
}

func LoadRecordsFromPath(path string, allowMissing bool) ([]Record, error) {
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

func LoadRecordsFromReader(reader io.Reader) ([]Record, error) {
	var records []Record
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var record Record
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			return nil, fmt.Errorf("invalid records.jsonl entry: %w", err)
		}
		records = append(records, record)
	}
	return records, scanner.Err()
}

func AppendRecord(record Record) error {
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
	var kept []Record
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

func WriteRecords(records []Record) error {
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

func FindRecord(id string) (*Record, error) {
	records, err := LoadRecords()
	if err != nil {
		return nil, err
	}
	for _, record := range records {
		if record.ID == id {
			return &record, nil
		}
	}
	return nil, nil
}
