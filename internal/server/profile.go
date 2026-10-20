package server

import (
	"bytes"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
)

func (a *application) updatePreferences(w http.ResponseWriter, r *http.Request) {
	current, ok := a.currentUser(w, r)
	if !ok {
		return
	}
	var input struct {
		Theme string `json:"theme"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	if input.Theme != "light" && input.Theme != "dark" {
		writeError(w, http.StatusBadRequest, "Choose a supported theme.")
		return
	}
	_, err := a.db.Exec(`INSERT INTO profiles(user_id, theme) VALUES(?, ?) ON CONFLICT(user_id) DO UPDATE SET theme = excluded.theme`, current.ID, input.Theme)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Could not save your preferences.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"theme": input.Theme})
}

func (a *application) getAvatar(w http.ResponseWriter, r *http.Request) {
	current, ok := a.currentUser(w, r)
	if !ok {
		return
	}
	var avatarName string
	if err := a.db.QueryRow(`SELECT avatar_name FROM profiles WHERE user_id = ?`, current.ID).Scan(&avatarName); err != nil || avatarName == "" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "private, max-age=300")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Type", mime.TypeByExtension(filepath.Ext(avatarName)))
	http.ServeFile(w, r, filepath.Join(a.root, current.Username, "profile", filepath.Base(avatarName)))
}

func (a *application) uploadAvatar(w http.ResponseWriter, r *http.Request) {
	current, ok := a.currentUser(w, r)
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxAvatarSize+(1<<20))
	if err := r.ParseMultipartForm(maxAvatarSize); err != nil {
		writeError(w, http.StatusRequestEntityTooLarge, "Profile images must be 5 MB or smaller.")
		return
	}
	file, _, err := r.FormFile("avatar")
	if err != nil {
		writeError(w, http.StatusBadRequest, "Choose an image to upload.")
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxAvatarSize+1))
	if err != nil || len(data) > maxAvatarSize {
		writeError(w, http.StatusRequestEntityTooLarge, "Profile images must be 5 MB or smaller.")
		return
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || config.Width < 1 || config.Height < 1 || config.Width > 2048 || config.Height > 2048 || int64(config.Width)*int64(config.Height) > 4_000_000 {
		writeError(w, http.StatusBadRequest, "Use a valid image no larger than 2048 by 2048 pixels.")
		return
	}
	extensions := map[string]string{"jpeg": ".jpg", "png": ".png", "gif": ".gif"}
	extension, supported := extensions[format]
	if !supported {
		writeError(w, http.StatusBadRequest, "Use a PNG, JPEG, or GIF image.")
		return
	}
	if err := a.createUserFolders(current.Username); err != nil {
		writeError(w, http.StatusInternalServerError, "Could not access profile storage.")
		return
	}
	avatarName := randomName() + extension
	avatarPath := filepath.Join(a.root, current.Username, "profile", avatarName)
	if err := os.WriteFile(avatarPath, data, 0600); err != nil {
		writeError(w, http.StatusInternalServerError, "Could not save profile image.")
		return
	}
	var previous string
	_ = a.db.QueryRow(`SELECT avatar_name FROM profiles WHERE user_id = ?`, current.ID).Scan(&previous)
	_, err = a.db.Exec(`INSERT INTO profiles(user_id, avatar_name) VALUES(?, ?) ON CONFLICT(user_id) DO UPDATE SET avatar_name = excluded.avatar_name`, current.ID, avatarName)
	if err != nil {
		_ = os.Remove(avatarPath)
		writeError(w, http.StatusInternalServerError, "Could not save profile image.")
		return
	}
	if previous != "" && previous != avatarName {
		_ = os.Remove(filepath.Join(a.root, current.Username, "profile", filepath.Base(previous)))
	}
	writeJSON(w, http.StatusOK, map[string]string{"avatarUrl": "/api/avatar"})
}

func (a *application) deleteAvatar(w http.ResponseWriter, r *http.Request) {
	current, ok := a.currentUser(w, r)
	if !ok {
		return
	}
	var avatarName string
	_ = a.db.QueryRow(`SELECT avatar_name FROM profiles WHERE user_id = ?`, current.ID).Scan(&avatarName)
	_, err := a.db.Exec(`INSERT INTO profiles(user_id, avatar_name) VALUES(?, '') ON CONFLICT(user_id) DO UPDATE SET avatar_name = ''`, current.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Could not remove profile image.")
		return
	}
	if avatarName != "" {
		_ = os.Remove(filepath.Join(a.root, current.Username, "profile", filepath.Base(avatarName)))
	}
	w.WriteHeader(http.StatusNoContent)
}
