/**
 * Component: Claude Context Bucketer
 * Block-UUID: 69492bc0-3061-4bee-8053-3f5b7bd89ed0
 * Parent-UUID: 6507b69c-f11f-436e-acb5-b86be909bdc4
 * Version: 1.1.0
 * Description: Implements the bucket building strategy for Claude context files. Supports Greedy bucketing for new/changed file sets and Leaware bucketing for stable sets to maximize cache hits.
 * Language: Go
 * Created-at: 2026-03-28T04:07:08.158Z
 * Authors: Gemini 3 Flash (v1.0.0), GLM-4.7 (v1.1.0)
 */


package claude

import (
	"github.com/gitsense/gsc-cli/internal/context"
)

// BuildBuckets decides between Greedy and Leaware bucketing strategies based on 
// the stability of the current context file set compared to the existing map.
func BuildBuckets(currentFiles []context.ContextFile, existingMap *MapFile) []Bucket {
	if existingMap == nil || len(existingMap.ContextFiles) == 0 {
		return greedyBucketing(currentFiles)
	}

	// Check if the set of files is stable (same IDs, growth within leeway)
	if isSetStable(currentFiles, existingMap) {
		return mapToExistingBuckets(currentFiles, existingMap)
	}

	// If set changed (add/delete) or growth exceeded leeway, revert to Greedy
	return greedyBucketing(currentFiles)
}

// isSetStable checks if the current files match the existing map's file set exactly.
// It returns true if the number of files is the same, all Chat IDs match in order,
// and no file has grown beyond the allowed LeewayBytes.
func isSetStable(current []context.ContextFile, existingMap *MapFile) bool {
	// 1. Flatten existing files from all buckets into a single list for comparison
	var existingFiles []FileEntry
	for _, bucket := range existingMap.ContextFiles {
		existingFiles = append(existingFiles, bucket.Files...)
	}

	// 2. Check if count matches
	if len(current) != len(existingFiles) {
		return false
	}

	// 3. Check IDs and Leeway
	// Both lists are expected to be sorted by Chat ID
	for i := range current {
		curr := current[i]
		ex := existingFiles[i]

		// If IDs don't match, the set has changed
		if curr.ChatID != ex.ChatID {
			return false
		}

		// If file grew beyond leeway, trigger a re-bucket (Greedy) to rebalance
		sizeDiff := curr.Size - ex.Size
		if sizeDiff > LeewayBytes {
			return false
		}
	}

	return true
}

// greedyBucketing implements a simple packing algorithm to group files into buckets.
// It respects MaxFileSizeBytes and MaxBucketSizeBytes.
func greedyBucketing(files []context.ContextFile) []Bucket {
	var buckets []Bucket
	var currentBucket *Bucket

	for _, f := range files {
		entry := FileEntry{
			ChatID: f.ChatID,
			Path:   f.Path,
			Size:   f.Size,
		}

		// Case 1: File is too large for standard buckets
		if f.Size > MaxFileSizeBytes {
			// Finalize current bucket if it exists
			if currentBucket != nil {
				buckets = append(buckets, *currentBucket)
				currentBucket = nil
			}
			// Create a dedicated bucket for this large file
			buckets = append(buckets, Bucket{
				MinID:     f.ChatID,
				MaxID:     f.ChatID,
				Files:     []FileEntry{entry},
				TotalSize: f.Size,
			})
			continue
		}

		// Case 2: Try to fit in current bucket
		if currentBucket == nil {
			currentBucket = &Bucket{
				MinID:     f.ChatID,
				MaxID:     f.ChatID,
				Files:     []FileEntry{entry},
				TotalSize: f.Size,
			}
		} else if currentBucket.TotalSize+f.Size <= MaxBucketSizeBytes {
			currentBucket.Files = append(currentBucket.Files, entry)
			currentBucket.TotalSize += f.Size
			currentBucket.MaxID = f.ChatID
		} else {
			// Bucket full, start new one
			buckets = append(buckets, *currentBucket)
			currentBucket = &Bucket{
				MinID:     f.ChatID,
				MaxID:     f.ChatID,
				Files:     []FileEntry{entry},
				TotalSize: f.Size,
			}
		}
	}

	if currentBucket != nil {
		buckets = append(buckets, *currentBucket)
	}

	return buckets
}

// mapToExistingBuckets preserves the existing bucket structure from the map,
// updating the sizes and file entries with the current content.
func mapToExistingBuckets(current []context.ContextFile, existingMap *MapFile) []Bucket {
	// Create a lookup map for O(1) access to current file data
	fileLookup := make(map[int64]context.ContextFile)
	for _, f := range current {
		fileLookup[f.ChatID] = f
	}

	var buckets []Bucket
	for _, exBucket := range existingMap.ContextFiles {
		newBucket := Bucket{
			MinID: exBucket.MinID,
			MaxID: exBucket.MaxID,
		}

		for _, exFile := range exBucket.Files {
			if currFile, ok := fileLookup[exFile.ChatID]; ok {
				newBucket.Files = append(newBucket.Files, FileEntry{
					ChatID: currFile.ChatID,
					Path:   currFile.Path,
					Size:   currFile.Size,
				})
				newBucket.TotalSize += currFile.Size
			}
		}

		if len(newBucket.Files) > 0 {
			buckets = append(buckets, newBucket)
		}
	}

	return buckets
}
