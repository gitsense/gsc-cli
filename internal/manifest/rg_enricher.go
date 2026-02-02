/**
 * Component: Ripgrep Metadata Enricher
 * Block-UUID: 0e4ef9ae-7c6f-40cc-bdda-b900448b9664
 * Parent-UUID: 776c90a3-adb1-4d81-97c5-fcaa26b71f5c
 * Version: 1.0.1
 * Description: Enriches raw ripgrep matches with metadata from the manifest database, adding Chat IDs and field values.
 * Language: Go
 * Created-at: 2026-02-02T19:07:01.374Z
 * Authors: GLM-4.7 (v1.0.0), Claude Haiku 4.5 (v1.0.1)
 */


package manifest

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/yourusername/gsc-cli/internal/db"
	"github.com/yourusername/gsc-cli/pkg/logger"
)

// EnrichMatches takes raw ripgrep matches and enriches them with metadata from the database.
// It looks up each file in the database and fetches all associated metadata fields.
func EnrichMatches(ctx context.Context, matches []RgMatch, dbName string) ([]EnrichedMatch, error) {
	if len(matches) == 0 {
		return []EnrichedMatch{}, nil
	}

	// 1. Validate Database Exists
	if err := ValidateDBExists(dbName); err != nil {
		return nil, err
	}

	// 2. Resolve DB Path
	dbPath, err := ResolveDBPath(dbName)
	if err != nil {
		return nil, err
	}

	// 3. Open Database
	database, err := db.OpenDB(dbPath)
	if err != nil {
		return nil, err
	}
	defer db.CloseDB(database)

	// 4. Enrich each match
	var enriched []EnrichedMatch
	for _, match := range matches {
		enrichedMatch, err := enrichSingleMatch(ctx, database, match)
		if err != nil {
			logger.Warning("Failed to enrich match for file %s: %v", match.FilePath, err)
			// Continue with other matches even if one fails
			continue
		}
		enriched = append(enriched, enrichedMatch)
	}

	logger.Info("Enriched %d matches", len(enriched))
	return enriched, nil
}

// enrichSingleMatch enriches a single ripgrep match with metadata.
func enrichSingleMatch(ctx context.Context, database *sql.DB, match RgMatch) (EnrichedMatch, error) {
	// 1. Get Chat ID and Metadata for the file
	query := `
		SELECT 
			f.chat_id,
			mf.field_name,
			fm.field_value
		FROM files f
		LEFT JOIN file_metadata fm ON f.file_path = fm.file_path
		LEFT JOIN metadata_fields mf ON fm.field_id = mf.field_id
		WHERE f.file_path = ?
	`

	rows, err := database.QueryContext(ctx, query, match.FilePath)
	if err != nil {
		return EnrichedMatch{}, fmt.Errorf("failed to query file metadata: %w", err)
	}
	defer rows.Close()

	var chatID int
	metadata := make(map[string]interface{})

	found := false
	for rows.Next() {
		found = true
		var fieldName, fieldValue sql.NullString
		
		if err := rows.Scan(&chatID, &fieldName, &fieldValue); err != nil {
			return EnrichedMatch{}, fmt.Errorf("failed to scan row: %w", err)
		}

		// Add to metadata map if field name is valid
		if fieldName.Valid {
			if fieldValue.Valid {
				metadata[fieldName.String] = fieldValue.String
			} else {
				metadata[fieldName.String] = nil
			}
		}
	}

	if err := rows.Err(); err != nil {
		return EnrichedMatch{}, err
	}

	if !found {
		// File not found in database, return match without metadata
		logger.Warning("File not found in database: %s", match.FilePath)
		return EnrichedMatch{
			FilePath:   match.FilePath,
			ChatID:     0,
			LineNumber: match.LineNumber,
			Match:      match.MatchText,
			Metadata:   map[string]interface{}{},
		}, nil
	}

	// 2. Construct EnrichedMatch
	return EnrichedMatch{
		FilePath:   match.FilePath,
		ChatID:     chatID,
		LineNumber: match.LineNumber,
		Match:      match.MatchText,
		Metadata:   metadata,
	}, nil
}
