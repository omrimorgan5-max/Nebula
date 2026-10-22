package server

import (
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func (a *application) listFiles(w http.ResponseWriter, r *http.Request) {
	current, ok := a.currentUser(w, r)
	if !ok {
		return
	}
	rows, err := a.db.Query(`SELECT id, name, size, created_at FROM uploads WHERE user_id = ? ORDER BY created_at DESC, id DESC`, current.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Could not load files.")
		return
	}
	defer rows.Close()
	files := []storedFile{}
	for rows.Next() {
		var item storedFile
		if err := rows.Scan(&item.ID, &item.Name, &item.Size, &item.CreatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, "Could not load files.")
			return
		}
		rows.Err()
		files = append(files, item)
	}
	writeJSON(w, http.StatusOK, files)
}

func (a *application) uploadFile(w http.ResponseWriter, r *http.Request) {
	current, ok := a.currentUser(w, r)
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize+(1<<20))
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		writeError(w, http.StatusRequestEntityTooLarge, "Files must be 25 MB or smaller.")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "Choose a file to upload.")
		return
	}
	defer file.Close()
	if header.Size > maxUploadSize {
		writeError(w, http.StatusRequestEntityTooLarge, "Files must be 25 MB or smaller.")
		return
	}
	name := filepath.Base(strings.ReplaceAll(header.Filename, "\\", "/"))
	name = strings.TrimSpace(strings.Map(func(r rune) rune {
		if r < 32 || r == 127 {
			return -1
		}
		return r
	}, name))
	if name == "" || name == "." || name == string(filepath.Separator) || len(name) > 180 {
		writeError(w, http.StatusBadRequest, "That file name is not supported.")
		return
	}
	userDir := filepath.Join(a.root, current.Username, "file-uploads")
	if err := os.MkdirAll(userDir, 0700); err != nil {
		writeError(w, http.StatusInternalServerError, "Could not access private storage.")
		return
	}
	diskName := randomName() + filepath.Ext(name)
	destination := filepath.Join(userDir, diskName)
	out, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Could not save file.")
		return
	}
	written, copyErr := io.Copy(out, io.LimitReader(file, maxUploadSize+1))
	closeErr := out.Close()
	if copyErr != nil || closeErr != nil || written > maxUploadSize {
		_ = os.Remove(destination)
		status := http.StatusInternalServerError
		message := "Could not save file."
		if written > maxUploadSize {
			status = http.StatusRequestEntityTooLarge
			message = "Files must be 25 MB or smaller."
		}
		writeError(w, status, message)
		return
	}
	result, err := a.db.Exec(`INSERT INTO uploads(user_id, name, disk_name, size) VALUES(?, ?, ?, ?)`, current.ID, name, diskName, written)
	if err != nil {
		_ = os.Remove(destination)
		writeError(w, http.StatusInternalServerError, "Could not save file.")
		return
	}
	id, _ := result.LastInsertId()
	writeJSON(w, http.StatusCreated, storedFile{ID: id, Name: name, Size: written, CreatedAt: time.Now().UTC()})
}

func (a *application) downloadFile(w http.ResponseWriter, r *http.Request) {
	current, ok := a.currentUser(w, r)
	if !ok {
		return
	}
	var item storedFile
	var diskName string
	if err := a.db.QueryRow(`SELECT name, disk_name, size, created_at FROM uploads WHERE id = ? AND user_id = ?`, r.PathValue("id"), current.ID).Scan(&item.Name, &diskName, &item.Size, &item.CreatedAt); err != nil {
		writeError(w, http.StatusNotFound, "File not found.")
		return
	}
	path := filepath.Join(a.root, current.Username, "file-uploads", filepath.Base(diskName))
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": item.Name}))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Type", mime.TypeByExtension(filepath.Ext(item.Name)))
	if w.Header().Get("Content-Type") == "" {
		w.Header().Set("Content-Type", "application/octet-stream")
	}
	http.ServeFile(w, r, path)
}
