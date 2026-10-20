package server

import "net/http"

func (a *application) dashboard(w http.ResponseWriter, r *http.Request) {
	current, ok := a.currentUser(w, r)
	if !ok {
		return
	}
	var noteCount, fileCount, postCount int
	for _, query := range []struct {
		statement string
		target    *int
	}{
		{`SELECT COUNT(*) FROM notes WHERE user_id = ?`, &noteCount},
		{`SELECT COUNT(*) FROM uploads WHERE user_id = ?`, &fileCount},
		{`SELECT COUNT(*) FROM posts WHERE user_id = ?`, &postCount},
	} {
		if err := a.db.QueryRow(query.statement, current.ID).Scan(query.target); err != nil {
			writeError(w, http.StatusInternalServerError, "Could not load your dashboard.")
			return
		}
	}
	rows, err := a.db.Query(`SELECT id, title, content, tags, pinned, updated_at FROM notes WHERE user_id = ? ORDER BY pinned DESC, updated_at DESC, id DESC LIMIT 5`, current.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Could not load your dashboard.")
		return
	}
	recentNotes := []note{}
	for rows.Next() {
		var item note
		var tags string
		if err := rows.Scan(&item.ID, &item.Title, &item.Content, &tags, &item.Pinned, &item.UpdatedAt); err != nil {
			rows.Close()
			writeError(w, http.StatusInternalServerError, "Could not load your dashboard.")
			return
		}
		item.Tags = decodeTags(tags)
		recentNotes = append(recentNotes, item)
	}
	if err := rows.Close(); err != nil {
		writeError(w, http.StatusInternalServerError, "Could not load your dashboard.")
		return
	}
	postRows, err := a.db.Query(`SELECT id, title, content, tags, status, created_at, updated_at FROM posts WHERE user_id = ? ORDER BY updated_at DESC, id DESC LIMIT 5`, current.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Could not load your dashboard.")
		return
	}
	recentPosts := []post{}
	for postRows.Next() {
		var item post
		var tags string
		if err := postRows.Scan(&item.ID, &item.Title, &item.Content, &tags, &item.Status, &item.CreatedAt, &item.UpdatedAt); err != nil {
			postRows.Close()
			writeError(w, http.StatusInternalServerError, "Could not load your dashboard.")
			return
		}
		item.Tags = decodeTags(tags)
		recentPosts = append(recentPosts, item)
	}
	if err := postRows.Close(); err != nil {
		writeError(w, http.StatusInternalServerError, "Could not load your dashboard.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"noteCount": noteCount, "fileCount": fileCount, "postCount": postCount,
		"recentNotes": recentNotes, "recentPosts": recentPosts,
	})
}
