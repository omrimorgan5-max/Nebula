package server

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"
)

const (
	maxTagsPerItem = 10
	maxTagLength   = 28
)

func normalizeTags(tags []string) ([]string, error) {
	if len(tags) > maxTagsPerItem {
		return nil, fmt.Errorf("use at most %d tags", maxTagsPerItem)
	}
	seen := make(map[string]bool, len(tags))
	normalized := make([]string, 0, len(tags))
	for _, tag := range tags {
		tag = strings.ToLower(strings.TrimSpace(tag))
		if tag == "" || seen[tag] {
			continue
		}
		if utf8.RuneCountInString(tag) > maxTagLength {
			return nil, fmt.Errorf("tags must be %d characters or fewer", maxTagLength)
		}
		seen[tag] = true
		normalized = append(normalized, tag)
	}
	return normalized, nil
}

func encodeTags(tags []string) string {
	encoded, _ := json.Marshal(tags)
	return string(encoded)
}

func decodeTags(encoded string) []string {
	var tags []string
	if err := json.Unmarshal([]byte(encoded), &tags); err != nil || tags == nil {
		return []string{}
	}
	return tags
}

func ensureContentTagsColumns(db *sql.DB) error {
	for _, table := range []string{"notes", "posts"} {
		rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
		if err != nil {
			return err
		}
		found := false
		for rows.Next() {
			var cid, notNull, primaryKey int
			var name, columnType string
			var defaultValue sql.NullString
			if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
				rows.Close()
				return err
			}
			if name == "tags" {
				found = true
			}
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return err
		}
		if err := rows.Close(); err != nil {
			return err
		}
		if !found {
			if _, err := db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN tags TEXT NOT NULL DEFAULT '[]'", table)); err != nil {
				return err
			}
		}
	}
	return nil
}
