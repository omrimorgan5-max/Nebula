package server

import (
	"net/http"
	"strings"
	"time"
)

func (a *application) listPosts(w http.ResponseWriter, r *http.Request) {
	current, ok := a.currentUser(w, r)
	if !ok {
		return
	}
	rows, err := a.db.Query(`SELECT id, title, content, tags, status, is_public, created_at, updated_at FROM posts WHERE user_id = ? ORDER BY updated_at DESC, id DESC`, current.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Could not load posts.")
		return
	}
	defer rows.Close()
	posts := []post{}
	for rows.Next() {
		var item post
		var tags string
		if err := rows.Scan(&item.ID, &item.Title, &item.Content, &tags, &item.Status, &item.IsPublic, &item.CreatedAt, &item.UpdatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, "Could not load posts.")
			return
		}
		item.Tags = decodeTags(tags)
		posts = append(posts, item)
	}
	writeJSON(w, http.StatusOK, posts)
}

func (a *application) createPost(w http.ResponseWriter, r *http.Request) {
	current, ok := a.currentUser(w, r)
	if !ok {
		return
	}
	var input struct {
		Title    string   `json:"title"`
		Content  string   `json:"content"`
		Tags     []string `json:"tags"`
		Status   string   `json:"status"`
		IsPublic bool     `json:"isPublic"`
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
	if input.Status == "" {
		input.Status = "draft"
	}
	if input.Title == "" || len(input.Title) > 160 || len(input.Content) > 1_000_000 || (input.Status != "draft" && input.Status != "published") || (input.IsPublic && input.Status != "published") {
		writeError(w, http.StatusBadRequest, "Add a title, keep content under 1 MB, and choose draft or published.")
		return
	}
	input.Content = a.sanitizer.Sanitize(input.Content)
	result, err := a.db.Exec(`INSERT INTO posts(user_id, title, content, tags, status, is_public, updated_at) VALUES(?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`, current.ID, input.Title, input.Content, encodeTags(tags), input.Status, input.IsPublic)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Could not save post.")
		return
	}
	id, _ := result.LastInsertId()
	writeJSON(w, http.StatusCreated, post{ID: id, Title: input.Title, Content: input.Content, Tags: tags, Status: input.Status, IsPublic: input.IsPublic, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()})
}

func (a *application) updatePost(w http.ResponseWriter, r *http.Request) {
	current, ok := a.currentUser(w, r)
	if !ok {
		return
	}
	var input struct {
		Title    string   `json:"title"`
		Content  string   `json:"content"`
		Tags     []string `json:"tags"`
		Status   string   `json:"status"`
		IsPublic bool     `json:"isPublic"`
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
	if input.Title == "" || len(input.Title) > 160 || len(input.Content) > 1_000_000 || (input.Status != "draft" && input.Status != "published") || (input.IsPublic && input.Status != "published") {
		writeError(w, http.StatusBadRequest, "Add a title, keep content under 1 MB, and choose draft or published.")
		return
	}
	input.Content = a.sanitizer.Sanitize(input.Content)
	result, err := a.db.Exec(`UPDATE posts SET title = ?, content = ?, tags = ?, status = ?, is_public = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ? AND user_id = ?`, input.Title, input.Content, encodeTags(tags), input.Status, input.IsPublic, r.PathValue("id"), current.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Could not save post.")
		return
	}
	count, _ := result.RowsAffected()
	if count == 0 {
		writeError(w, http.StatusNotFound, "Post not found.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *application) deletePost(w http.ResponseWriter, r *http.Request) {
	current, ok := a.currentUser(w, r)
	if !ok {
		return
	}
	result, err := a.db.Exec(`DELETE FROM posts WHERE id = ? AND user_id = ?`, r.PathValue("id"), current.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Could not delete post.")
		return
	}
	count, _ := result.RowsAffected()
	if count == 0 {
		writeError(w, http.StatusNotFound, "Post not found.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
