package server

import (
	"database/sql"
	"html/template"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var profileAccents = map[string]bool{
	"moss":  true,
	"ocean": true,
	"clay":  true,
	"plum":  true,
}

type publicPost struct {
	Title     string
	Content   template.HTML
	CreatedAt string
}

type publicProfilePage struct {
	Username    string
	DisplayName string
	Bio         string
	Accent      string
	AvatarURL   string
	Posts       []publicPost
}

func (a *application) getProfileSettings(w http.ResponseWriter, r *http.Request) {
	current, ok := a.currentUser(w, r)
	if !ok {
		return
	}
	var settings profileSettings
	var enabled, showAvatar int
	err := a.db.QueryRow(`SELECT display_name, bio, accent, public_enabled, show_avatar FROM profiles WHERE user_id = ?`, current.ID).Scan(&settings.DisplayName, &settings.Bio, &settings.Accent, &enabled, &showAvatar)
	if err != nil && err != sql.ErrNoRows {
		writeError(w, http.StatusInternalServerError, "Could not load profile settings.")
		return
	}
	settings.PublicEnabled = enabled != 0
	settings.ShowAvatar = showAvatar != 0
	if settings.Accent == "" {
		settings.Accent = "moss"
	}
	if settings.PublicEnabled {
		settings.PublicURL = "/u/" + current.Username
	}
	writeJSON(w, http.StatusOK, settings)
}

func (a *application) updateProfileSettings(w http.ResponseWriter, r *http.Request) {
	current, ok := a.currentUser(w, r)
	if !ok {
		return
	}
	var settings profileSettings
	if !decodeJSON(w, r, &settings) {
		return
	}
	settings.DisplayName = strings.TrimSpace(settings.DisplayName)
	settings.Bio = strings.TrimSpace(settings.Bio)
	if !settings.PublicEnabled {
		settings.ShowAvatar = false
	}
	if len([]rune(settings.DisplayName)) > 80 || len([]rune(settings.Bio)) > 280 || !profileAccents[settings.Accent] {
		writeError(w, http.StatusBadRequest, "Use a name under 80 characters, bio under 280 characters, and a supported accent.")
		return
	}
	tx, err := a.db.Begin()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Could not save profile settings.")
		return
	}
	if _, err := tx.Exec(`INSERT INTO profiles(user_id, display_name, bio, accent, public_enabled, show_avatar) VALUES(?, ?, ?, ?, ?, ?) ON CONFLICT(user_id) DO UPDATE SET display_name = excluded.display_name, bio = excluded.bio, accent = excluded.accent, public_enabled = excluded.public_enabled, show_avatar = excluded.show_avatar`, current.ID, settings.DisplayName, settings.Bio, settings.Accent, settings.PublicEnabled, settings.ShowAvatar); err != nil {
		_ = tx.Rollback()
		writeError(w, http.StatusInternalServerError, "Could not save profile settings.")
		return
	}
	if !settings.PublicEnabled {
		if _, err := tx.Exec(`UPDATE posts SET is_public = 0 WHERE user_id = ?`, current.ID); err != nil {
			_ = tx.Rollback()
			writeError(w, http.StatusInternalServerError, "Could not make your posts private.")
			return
		}
	}
	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, "Could not save profile settings.")
		return
	}
	if settings.PublicEnabled {
		settings.PublicURL = "/u/" + current.Username
	}
	writeJSON(w, http.StatusOK, settings)
}

func (a *application) publicProfile(w http.ResponseWriter, r *http.Request) {
	username := strings.ToLower(r.PathValue("username"))
	if !usernamePattern.MatchString(username) {
		http.NotFound(w, r)
		return
	}
	var page publicProfilePage
	var avatarName string
	var showAvatarValue int
	err := a.db.QueryRow(`SELECT users.username, profiles.display_name, profiles.bio, profiles.accent, profiles.avatar_name, profiles.show_avatar FROM users JOIN profiles ON profiles.user_id = users.id WHERE users.username = ? AND profiles.public_enabled = 1`, username).Scan(&page.Username, &page.DisplayName, &page.Bio, &page.Accent, &avatarName, &showAvatarValue)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	showAvatar := showAvatarValue == 1
	if showAvatar && avatarName != "" {
		page.AvatarURL = "/u/" + page.Username + "/avatar"
	} else {
		page.AvatarURL = ""
	}
	if page.DisplayName == "" {
		page.DisplayName = page.Username
	}
	rows, err := a.db.Query(`SELECT title, content, created_at FROM posts WHERE user_id = (SELECT id FROM users WHERE username = ?) AND status = 'published' AND is_public = 1 ORDER BY created_at DESC`, username)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Could not load public posts.")
		return
	}
	defer rows.Close()
	page.Posts = []publicPost{}
	for rows.Next() {
		var item publicPost
		var content string
		var createdAt time.Time
		if err := rows.Scan(&item.Title, &content, &createdAt); err != nil {
			writeError(w, http.StatusInternalServerError, "Could not load public posts.")
			return
		}
		item.Content = template.HTML(a.sanitizer.Sanitize(content))
		item.CreatedAt = createdAt.Format("January 2, 2006")
		page.Posts = append(page.Posts, item)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "Could not load public posts.")
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := a.templates.ExecuteTemplate(w, "public-profile.html", page); err != nil {
		writeError(w, http.StatusInternalServerError, "Could not render public profile.")
	}
}

func (a *application) publicAvatar(w http.ResponseWriter, r *http.Request) {
	username := strings.ToLower(r.PathValue("username"))
	var avatarName string
	err := a.db.QueryRow(`SELECT profiles.avatar_name FROM users JOIN profiles ON profiles.user_id = users.id WHERE users.username = ? AND profiles.public_enabled = 1 AND profiles.show_avatar = 1`, username).Scan(&avatarName)
	if err != nil || avatarName == "" {
		http.NotFound(w, r)
		return
	}
	path := filepath.Join(a.root, username, "profile", filepath.Base(avatarName))
	if _, err := os.Stat(path); err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=300")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Type", mime.TypeByExtension(filepath.Ext(avatarName)))
	http.ServeFile(w, r, path)
}

func ensurePublicProfileColumns(db *sql.DB) error {
	columns := []struct {
		name string
		ddl  string
	}{
		{"public_enabled", `INTEGER NOT NULL DEFAULT 0`},
		{"display_name", `TEXT NOT NULL DEFAULT ''`},
		{"bio", `TEXT NOT NULL DEFAULT ''`},
		{"accent", `TEXT NOT NULL DEFAULT 'moss'`},
		{"show_avatar", `INTEGER NOT NULL DEFAULT 0`},
	}
	for _, column := range columns {
		found, err := hasColumn(db, "profiles", column.name)
		if err != nil {
			return err
		}
		if !found {
			if _, err := db.Exec(`ALTER TABLE profiles ADD COLUMN ` + column.name + ` ` + column.ddl); err != nil {
				return err
			}
		}
	}
	return nil
}

func ensurePublicPostColumn(db *sql.DB) error {
	found, err := hasColumn(db, "posts", "is_public")
	if err != nil || found {
		return err
	}
	_, err = db.Exec(`ALTER TABLE posts ADD COLUMN is_public INTEGER NOT NULL DEFAULT 0`)
	return err
}

func hasColumn(db *sql.DB, table, column string) (bool, error) {
	rows, err := db.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var cid, notNull, primaryKey int
		var name, columnType string
		var defaultValue sql.NullString
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			return false, err
		}
		if name == column {
			return true, nil
		}
	}
	return false, rows.Err()
}
