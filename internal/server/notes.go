package server

import (
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func (a *application) listNotes(w http.ResponseWriter, r *http.Request) {
	current, ok := a.currentUser(w, r)
	if !ok {
		return
	}
	rows, err := a.db.Query(`SELECT id, title, content, tags, pinned, updated_at FROM notes WHERE user_id = ? ORDER BY pinned DESC, updated_at DESC, id DESC`, current.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Could not load notes.")
		return
	}
	defer rows.Close()
	notes := []note{}
	for rows.Next() {
		var item note
		var tags string
		if err := rows.Scan(&item.ID, &item.Title, &item.Content, &tags, &item.Pinned, &item.UpdatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, "Could not load notes.")
			return
		}
		item.Tags = decodeTags(tags)
		notes = append(notes, item)
	}
	writeJSON(w, http.StatusOK, notes)
}

func (a *application) createNote(w http.ResponseWriter, r *http.Request) {
	current, ok := a.currentUser(w, r)
	if !ok {
		return
	}
	var input struct {
		Title   string   `json:"title"`
		Content string   `json:"content"`
		Tags    []string `json:"tags"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	input.Title = strings.TrimSpace(input.Title)
	tags, err := normalizeTags(input.Tags)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if input.Title == "" || len(input.Title) > 160 || len(input.Content) > 1_000_000 {
		writeError(w, http.StatusBadRequest, "Enter a title under 160 characters. Note content must be under 1 MB.")
		return
	}
	input.Content = a.sanitizer.Sanitize(input.Content)
	result, err := a.db.Exec(`INSERT INTO notes(user_id, title, content, tags, updated_at) VALUES(?, ?, ?, ?, CURRENT_TIMESTAMP)`, current.ID, input.Title, input.Content, encodeTags(tags))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Could not save note.")
		return
	}
	id, _ := result.LastInsertId()
	if err := a.writeNoteFile(current.Username, id, input.Content); err != nil {
		_, _ = a.db.Exec(`DELETE FROM notes WHERE id = ? AND user_id = ?`, id, current.ID)
		writeError(w, http.StatusInternalServerError, "Could not save note.")
		return
	}
	writeJSON(w, http.StatusCreated, note{ID: id, Title: input.Title, Content: input.Content, Tags: tags, UpdatedAt: time.Now().UTC()})
}

func (a *application) updateNote(w http.ResponseWriter, r *http.Request) {
	current, ok := a.currentUser(w, r)
	if !ok {
		return
	}
	var input struct {
		Title   string   `json:"title"`
		Content string   `json:"content"`
		Tags    []string `json:"tags"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	input.Title = strings.TrimSpace(input.Title)
	tags, err := normalizeTags(input.Tags)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if input.Title == "" || len(input.Title) > 160 || len(input.Content) > 1_000_000 {
		writeError(w, http.StatusBadRequest, "Enter a title under 160 characters. Note content must be under 1 MB.")
		return
	}
	input.Content = a.sanitizer.Sanitize(input.Content)
	result, err := a.db.Exec(`UPDATE notes SET title = ?, content = ?, tags = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ? AND user_id = ?`, input.Title, input.Content, encodeTags(tags), r.PathValue("id"), current.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Could not save note.")
		return
	}
	count, _ := result.RowsAffected()
	if count == 0 {
		writeError(w, http.StatusNotFound, "Note not found.")
		return
	}
	if err := a.writeNoteFile(current.Username, parseID(r.PathValue("id")), input.Content); err != nil {
		writeError(w, http.StatusInternalServerError, "Could not save note.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *application) pinNote(w http.ResponseWriter, r *http.Request) {
	current, ok := a.currentUser(w, r)
	if !ok {
		return
	}
	var input struct {
		Pinned bool `json:"pinned"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := a.db.Exec(`UPDATE notes SET pinned = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ? AND user_id = ?`, input.Pinned, r.PathValue("id"), current.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Could not update note.")
		return
	}
	count, _ := result.RowsAffected()
	if count == 0 {
		writeError(w, http.StatusNotFound, "Note not found.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *application) deleteNote(w http.ResponseWriter, r *http.Request) {
	current, ok := a.currentUser(w, r)
	if !ok {
		return
	}
	result, err := a.db.Exec(`DELETE FROM notes WHERE id = ? AND user_id = ?`, r.PathValue("id"), current.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Could not delete note.")
		return
	}
	count, _ := result.RowsAffected()
	if count == 0 {
		writeError(w, http.StatusNotFound, "Note not found.")
		return
	}
	_ = os.Remove(a.notePath(current.Username, parseID(r.PathValue("id"))))
	w.WriteHeader(http.StatusNoContent)
}

func (a *application) createUserFolders(username string) error {
	for _, name := range []string{"file-uploads", "notes", "profile"} {
		if err := os.MkdirAll(filepath.Join(a.root, username, name), 0700); err != nil {
			return err
		}
	}
	return nil
}

func (a *application) notePath(username string, id int64) string {
	return filepath.Join(a.root, username, "notes", fmt.Sprintf("note-%d.html", id))
}

func (a *application) writeNoteFile(username string, id int64, content string) error {
	return os.WriteFile(a.notePath(username, id), []byte(content), 0600)
}

func parseID(value string) int64 {
	id, _ := strconv.ParseInt(value, 10, 64)
	return id
}

func ensureNotePinnedColumn(db *sql.DB) error {
	rows, err := db.Query(`PRAGMA table_info(notes)`)
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
		if name == "pinned" {
			found = true
		}
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if found {
		return nil
	}
	_, err = db.Exec(`ALTER TABLE notes ADD COLUMN pinned INTEGER NOT NULL DEFAULT 0 CHECK(pinned IN (0, 1))`)
	return err
}
