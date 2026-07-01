/**
 * Component: Topics Canonical Store
 * Block-UUID: a1b2c3d4-e5f6-7890-abcd-100000000004
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Reads, appends, rewrites, and deletes topic records in the canonical JSONL store.
 * Language: Go
 * Created-at: 2026-06-22T10:00:00Z
 * Authors: MiMo-v2.5-pro (v1.0.0)
 */

package topics

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gitsense/gsc-cli/internal/git"
	"github.com/gitsense/gsc-cli/pkg/settings"
)

// Registry holds all topics in memory for validation and lookup.
type Registry struct {
	Topics []Topic
	index  map[string]int // slug -> index in Topics
}

// NewRegistry creates a Registry from a topic list and builds the lookup index.
func NewRegistry(topics []Topic) *Registry {
	r := &Registry{
		Topics: topics,
		index:  make(map[string]int, len(topics)),
	}
	for i, t := range topics {
		r.index[strings.ToLower(t.Slug)] = i
	}
	return r
}

// Exists returns true if the slug is registered.
func (r *Registry) Exists(slug string) bool {
	_, ok := r.index[strings.ToLower(slug)]
	return ok
}

// Get returns the topic for the given slug, or nil if not found.
func (r *Registry) Get(slug string) *Topic {
	idx, ok := r.index[strings.ToLower(slug)]
	if !ok {
		return nil
	}
	return &r.Topics[idx]
}

// Slugs returns all registered slugs.
func (r *Registry) Slugs() []string {
	slugs := make([]string, len(r.Topics))
	for i, t := range r.Topics {
		slugs[i] = t.Slug
	}
	return slugs
}

func rootDir() (string, error) {
	return git.FindProjectRoot()
}

func gitsenseDir() (string, error) {
	root, err := rootDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, settings.GitSenseDir), nil
}

// TopicsDir returns the path to .gitsense/topics/
func TopicsDir() (string, error) {
	dir, err := gitsenseDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "topics"), nil
}

// RecordsPath returns the path to .gitsense/topics/records.jsonl
func RecordsPath() (string, error) {
	dir, err := TopicsDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "records.jsonl"), nil
}

// LoadRegistry reads all topics and returns a Registry.
func LoadRegistry() (*Registry, error) {
	topics, err := LoadRecords()
	if err != nil {
		return nil, err
	}
	return NewRegistry(topics), nil
}

// LoadRecords reads all topic records from the JSONL store.
func LoadRecords() ([]Topic, error) {
	path, err := RecordsPath()
	if err != nil {
		return nil, err
	}
	return LoadRecordsFromPath(path, true)
}

// LoadRecordsFromPath reads topic records from a specific path.
func LoadRecordsFromPath(path string, allowMissing bool) ([]Topic, error) {
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

// LoadRecordsFromReader reads topic records from a reader.
func LoadRecordsFromReader(reader *os.File) ([]Topic, error) {
	var topics []Topic
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var topic Topic
		if err := json.Unmarshal([]byte(line), &topic); err != nil {
			return nil, fmt.Errorf("invalid topics/records.jsonl entry: %w", err)
		}
		topics = append(topics, topic)
	}
	return topics, scanner.Err()
}

// AppendRecord adds a new topic to the JSONL store.
func AppendRecord(topic Topic) error {
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
	data, err := json.Marshal(topic)
	if err != nil {
		return err
	}
	if _, err := file.Write(append(data, '\n')); err != nil {
		return err
	}
	return nil
}

// UpdateRecord updates an existing topic in the JSONL store.
func UpdateRecord(slug string, description string) (bool, error) {
	topics, err := LoadRecords()
	if err != nil {
		return false, err
	}

	found := false
	for i, t := range topics {
		if strings.EqualFold(t.Slug, slug) {
			topics[i].Description = description
			topics[i].UpdatedAt = time.Now().UTC()
			found = true
			break
		}
	}

	if !found {
		return false, nil
	}

	return true, WriteRecords(topics)
}

// WriteRecords rewrites the entire JSONL store.
func WriteRecords(topics []Topic) error {
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
	for _, topic := range topics {
		data, err := json.Marshal(topic)
		if err != nil {
			return err
		}
		if _, err := file.Write(append(data, '\n')); err != nil {
			return err
		}
	}
	return nil
}
