package server

import (
	"context"
	"syscall"
	"os/signal"
	"database/sql"
	"embed"
	"errors"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/microcosm-cc/bluemonday"
	_ "modernc.org/sqlite"
)

//go:embed web/templates/*.html web/static
var webFiles embed.FS

const (
	maxUploadSize  = 25 << 20
	maxAvatarSize  = 5 << 20
	sessionAge     = 30 * 24 * time.Hour
	passwordRounds = 310_000
)

var usernamePattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{3,32}$`)

type application struct {
	db        *sql.DB
	root      string
	secure    bool
	sanitizer *bluemonday.Policy
	templates *template.Template
}

type user struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	Theme     string `json:"theme"`
	AvatarURL string `json:"avatarUrl,omitempty"`
}

type note struct {
	ID        int64     `json:"id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	Tags      []string  `json:"tags"`
	Pinned    bool      `json:"pinned"`
	IsPublic  bool      `json:"isPublic"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type post struct {
	ID        int64     `json:"id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	Tags      []string  `json:"tags"`
	Status    string    `json:"status"`
	IsPublic  bool      `json:"isPublic"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type profileSettings struct {
	DisplayName   string `json:"displayName"`
	Bio           string `json:"bio"`
	Accent        string `json:"accent"`
	PublicEnabled bool   `json:"publicEnabled"`
	ShowAvatar    bool   `json:"showAvatar"`
	PublicURL     string `json:"publicUrl,omitempty"`
}

type storedFile struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Size      int64     `json:"size"`
	CreatedAt time.Time `json:"createdAt"`
}

func Run() {
	root := env("DATA_DIR", "./files")
	if err := os.MkdirAll(root, 0700); err != nil {
		logFatal("create data directory", err)
	}
	db, err := sql.Open("sqlite", filepath.Join(root, "nebula.db"))
	if err != nil {
		logFatal("open database", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`PRAGMA journal_mode=WAL; PRAGMA foreign_keys=ON;
		CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT NOT NULL UNIQUE,
			salt BLOB NOT NULL,
			password_hash BLOB NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE IF NOT EXISTS sessions (
			token_hash TEXT PRIMARY KEY,
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			expires_at DATETIME NOT NULL
		);
		CREATE TABLE IF NOT EXISTS notes (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			title TEXT NOT NULL,
			content TEXT NOT NULL DEFAULT '',
			tags TEXT NOT NULL DEFAULT '[]',
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE IF NOT EXISTS uploads (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			name TEXT NOT NULL,
			disk_name TEXT NOT NULL,
			size INTEGER NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE IF NOT EXISTS profiles (
			user_id INTEGER PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
			theme TEXT NOT NULL DEFAULT 'light' CHECK(theme IN ('light', 'dark')),
			avatar_name TEXT NOT NULL DEFAULT '',
			public_enabled INTEGER NOT NULL DEFAULT 0,
			display_name TEXT NOT NULL DEFAULT '',
			bio TEXT NOT NULL DEFAULT '',
			accent TEXT NOT NULL DEFAULT 'moss',
			show_avatar INTEGER NOT NULL DEFAULT 0
		);
		CREATE TABLE IF NOT EXISTS posts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			title TEXT NOT NULL,
			content TEXT NOT NULL DEFAULT '',
			tags TEXT NOT NULL DEFAULT '[]',
			status TEXT NOT NULL DEFAULT 'draft' CHECK(status IN ('draft', 'published')),
			is_public INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);`); err != nil {
		logFatal("initialize database", err)
	}
	if err := ensureNotePinnedColumn(db); err != nil {
		logFatal("upgrade notes schema", err)
	}
	if err := ensureContentTagsColumns(db); err != nil {
		logFatal("upgrade content tags", err)
	}
	if err := ensurePublicProfileColumns(db); err != nil {
		logFatal("upgrade public profile settings", err)
	}
	if err := ensurePublicPostColumn(db); err != nil {
		logFatal("upgrade public post visibility", err)
	}
	if err := os.Chmod(filepath.Join(root, "nebula.db"), 0600); err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Printf("set database permissions: %v", err)
	}
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		logFatal("resolve data directory", err)
	}
	contentPolicy := bluemonday.NewPolicy()
	contentPolicy.AllowElements("b", "strong", "i", "em", "p", "div", "br", "font")
	contentPolicy.AllowAttrs("face").Matching(regexp.MustCompile(`(?i)^(Georgia|Arial|Trebuchet MS|Courier New)$`)).OnElements("font")
	templates, err := template.ParseFS(webFiles, "web/templates/*.html")
	if err != nil {
		logFatal("load page templates", err)
	}
	staticFiles, err := fs.Sub(webFiles, "web/static")
	if err != nil {
		logFatal("load static assets", err)
	}
	app := &application{
		db: db, root: absoluteRoot, secure: env("COOKIE_SECURE", "auto") != "false",
		sanitizer: contentPolicy, templates: templates,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/auth/signup", app.signUp)
	mux.HandleFunc("POST /api/auth/signin", app.signIn)
	mux.HandleFunc("POST /api/auth/signout", app.signOut)
	mux.HandleFunc("GET /api/me", app.me)
	mux.HandleFunc("GET /api/profile", app.getProfileSettings)
	mux.HandleFunc("PUT /api/profile", app.updateProfileSettings)
	mux.HandleFunc("GET /api/dashboard", app.dashboard)
	mux.HandleFunc("PUT /api/preferences", app.updatePreferences)
	mux.HandleFunc("GET /api/avatar", app.getAvatar)
	mux.HandleFunc("POST /api/avatar", app.uploadAvatar)
	mux.HandleFunc("DELETE /api/avatar", app.deleteAvatar)
	mux.HandleFunc("GET /api/notes", app.listNotes)
	mux.HandleFunc("POST /api/notes", app.createNote)
	mux.HandleFunc("PUT /api/notes/{id}", app.updateNote)
	mux.HandleFunc("PATCH /api/notes/{id}/pin", app.pinNote)
	mux.HandleFunc("DELETE /api/notes/{id}", app.deleteNote)
	mux.HandleFunc("GET /api/posts", app.listPosts)
	mux.HandleFunc("POST /api/posts", app.createPost)
	mux.HandleFunc("PUT /api/posts/{id}", app.updatePost)
	mux.HandleFunc("DELETE /api/posts/{id}", app.deletePost)
	mux.HandleFunc("GET /u/{username}", app.publicProfile)
	mux.HandleFunc("GET /u/{username}/avatar", app.publicAvatar)
	mux.HandleFunc("GET /api/files", app.listFiles)
	mux.HandleFunc("POST /api/files", app.uploadFile)
	mux.HandleFunc("GET /api/files/{id}/download", app.downloadFile)
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFiles))))
	mux.HandleFunc("GET /{$}", app.renderPage)
	mux.HandleFunc("GET /dashboard", app.renderPage)
	mux.HandleFunc("GET /notes", app.renderPage)
	mux.HandleFunc("GET /posts", app.renderPage)
	mux.HandleFunc("GET /files", app.renderPage)
	addr := env("ADDR", ":8080")
    log.Printf("Nebula listening on %s", addr)
    
    httpServer := &http.Server{
        Addr:              addr, 
        Handler:           securityHeaders(mux), 
        ReadHeaderTimeout: 10 * time.Second, 
        IdleTimeout:       90 * time.Second,
    }

    // 1. Start the HTTP server in a goroutine so it doesn't block
    go func() {
        if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
            log.Printf("HTTP server error: %v", err)
        }
    }()

    // 2. Listen for interrupt signals (Ctrl+C, SIGTERM)
    stop := make(chan os.Signal, 1)
    signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
    <-stop // Block until a signal is received

    log.Println("Shutting down server...")

    // 3. Gracefully shut down the HTTP server
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    if err := httpServer.Shutdown(ctx); err != nil {
        log.Printf("Server forced shutdown: %v", err)
    }

    log.Println("Server exited cleanly.")
}

func (a *application) renderPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	if err := a.templates.ExecuteTemplate(w, "layout.html", nil); err != nil {
		log.Printf("render page: %v", err)
		http.Error(w, "Could not render page.", http.StatusInternalServerError)
	}
}
