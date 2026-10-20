package server

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/microcosm-cc/bluemonday"
	_ "modernc.org/sqlite"
)

func testApplication(t *testing.T) (*application, http.Handler) {
	t.Helper()
	root := t.TempDir()
	db, err := sql.Open("sqlite", filepath.Join(root, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	_, err = db.Exec(`PRAGMA foreign_keys=ON;
		CREATE TABLE users (id INTEGER PRIMARY KEY AUTOINCREMENT, username TEXT NOT NULL UNIQUE, salt BLOB NOT NULL, password_hash BLOB NOT NULL);
		CREATE TABLE sessions (token_hash TEXT PRIMARY KEY, user_id INTEGER NOT NULL REFERENCES users(id), expires_at DATETIME NOT NULL);
		CREATE TABLE notes (id INTEGER PRIMARY KEY AUTOINCREMENT, user_id INTEGER NOT NULL REFERENCES users(id), title TEXT NOT NULL, content TEXT NOT NULL DEFAULT '', tags TEXT NOT NULL DEFAULT '[]', updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, pinned INTEGER NOT NULL DEFAULT 0);
		CREATE TABLE uploads (id INTEGER PRIMARY KEY AUTOINCREMENT, user_id INTEGER NOT NULL REFERENCES users(id), name TEXT NOT NULL, disk_name TEXT NOT NULL, size INTEGER NOT NULL, created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP);
		CREATE TABLE profiles (user_id INTEGER PRIMARY KEY REFERENCES users(id), theme TEXT NOT NULL DEFAULT 'light', avatar_name TEXT NOT NULL DEFAULT '');
		CREATE TABLE posts (id INTEGER PRIMARY KEY AUTOINCREMENT, user_id INTEGER NOT NULL REFERENCES users(id), title TEXT NOT NULL, content TEXT NOT NULL DEFAULT '', tags TEXT NOT NULL DEFAULT '[]', status TEXT NOT NULL DEFAULT 'draft', created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP);`)
	if err != nil {
		db.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	policy := bluemonday.NewPolicy()
	policy.AllowElements("p", "b", "strong", "i", "em")
	policy.AllowAttrs("face").OnElements("font")
	app := &application{db: db, root: filepath.Join(root, "files"), secure: false, sanitizer: policy}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/auth/signup", app.signUp)
	mux.HandleFunc("GET /api/dashboard", app.dashboard)
	mux.HandleFunc("GET /api/notes", app.listNotes)
	mux.HandleFunc("POST /api/notes", app.createNote)
	mux.HandleFunc("PATCH /api/notes/{id}/pin", app.pinNote)
	mux.HandleFunc("GET /api/posts", app.listPosts)
	mux.HandleFunc("POST /api/posts", app.createPost)
	mux.HandleFunc("PUT /api/posts/{id}", app.updatePost)
	mux.HandleFunc("DELETE /api/posts/{id}", app.deletePost)
	return app, mux
}

func testClient(t *testing.T, server *httptest.Server, username string) *http.Client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Jar: jar}
	body := `{"username":"` + username + `","password":"long-password-123"}`
	response, err := client.Post(server.URL+"/api/auth/signup", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("signup returned %d", response.StatusCode)
	}
	return client
}

func sendJSON(t *testing.T, client *http.Client, method, url, value string, expectedStatus int, target any) {
	t.Helper()
	request, err := http.NewRequest(method, url, bytes.NewBufferString(value))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != expectedStatus {
		var body bytes.Buffer
		_, _ = body.ReadFrom(response.Body)
		t.Fatalf("%s %s returned %d, want %d: %s", method, url, response.StatusCode, expectedStatus, body.String())
	}
	if target != nil {
		if err := json.NewDecoder(response.Body).Decode(target); err != nil {
			t.Fatal(err)
		}
	}
}

func TestPrivatePostsPinnedNotesAndDashboard(t *testing.T) {
	_, handler := testApplication(t)
	server := httptest.NewServer(handler)
	defer server.Close()
	first := testClient(t, server, "tester_one")

	var createdNote note
	sendJSON(t, first, http.MethodPost, server.URL+"/api/notes", `{"title":"Field notes","content":"<p>Keep this close.</p>","tags":["Ideas","ideas","Personal"]}`, http.StatusCreated, &createdNote)
	if len(createdNote.Tags) != 2 || createdNote.Tags[0] != "ideas" {
		t.Fatalf("note tags were not normalized: %+v", createdNote.Tags)
	}
	sendJSON(t, first, http.MethodPatch, server.URL+"/api/notes/"+jsonNumber(createdNote.ID)+"/pin", `{"pinned":true}`, http.StatusNoContent, nil)
	var createdPost post
	sendJSON(t, first, http.MethodPost, server.URL+"/api/posts", `{"title":"A small story","content":"A private draft","status":"draft","tags":["journal"]}`, http.StatusCreated, &createdPost)
	if len(createdPost.Tags) != 1 || createdPost.Tags[0] != "journal" {
		t.Fatalf("post tags were not returned: %+v", createdPost.Tags)
	}
	sendJSON(t, first, http.MethodPut, server.URL+"/api/posts/"+jsonNumber(createdPost.ID), `{"title":"A small story","content":"A private draft","status":"published","tags":["personal essay"]}`, http.StatusNoContent, nil)
	var listedPosts []post
	sendJSON(t, first, http.MethodGet, server.URL+"/api/posts", "", http.StatusOK, &listedPosts)
	if len(listedPosts) != 1 || listedPosts[0].Status != "published" || len(listedPosts[0].Tags) != 1 || listedPosts[0].Tags[0] != "personal essay" {
		t.Fatalf("updated private post was not returned: %+v", listedPosts)
	}
	tooManyTags := `{"title":"Too many tags","content":"","tags":["a","b","c","d","e","f","g","h","i","j","k"]}`
	sendJSON(t, first, http.MethodPost, server.URL+"/api/notes", tooManyTags, http.StatusBadRequest, nil)

	var dashboard struct {
		NoteCount   int    `json:"noteCount"`
		PostCount   int    `json:"postCount"`
		FileCount   int    `json:"fileCount"`
		RecentNotes []note `json:"recentNotes"`
		RecentPosts []post `json:"recentPosts"`
	}
	sendJSON(t, first, http.MethodGet, server.URL+"/api/dashboard", "", http.StatusOK, &dashboard)
	if dashboard.NoteCount != 1 || dashboard.PostCount != 1 || len(dashboard.RecentNotes) != 1 || !dashboard.RecentNotes[0].Pinned {
		t.Fatalf("unexpected dashboard summary: %+v", dashboard)
	}
	if len(dashboard.RecentNotes[0].Tags) != 2 || len(dashboard.RecentPosts) != 1 || dashboard.RecentPosts[0].Tags[0] != "personal essay" {
		t.Fatalf("dashboard tags were not returned: %+v", dashboard)
	}

	second := testClient(t, server, "tester_two")
	var otherDashboard struct {
		NoteCount int `json:"noteCount"`
		PostCount int `json:"postCount"`
	}
	sendJSON(t, second, http.MethodGet, server.URL+"/api/dashboard", "", http.StatusOK, &otherDashboard)
	if otherDashboard.NoteCount != 0 || otherDashboard.PostCount != 0 {
		t.Fatalf("dashboard leaked another account's content: %+v", otherDashboard)
	}
	sendJSON(t, second, http.MethodPut, server.URL+"/api/posts/"+jsonNumber(createdPost.ID), `{"title":"Changed","content":"No","status":"published"}`, http.StatusNotFound, nil)
}

func TestEnsureNotePinnedColumnMigratesExistingSchema(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "old.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE notes (id INTEGER PRIMARY KEY, title TEXT NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	if err := ensureNotePinnedColumn(db); err != nil {
		t.Fatal(err)
	}
	var found int
	if err := db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('notes') WHERE name = 'pinned'`).Scan(&found); err != nil {
		t.Fatal(err)
	}
	if found != 1 {
		t.Fatal("pinned column was not added")
	}
}

func TestEnsureContentTagsColumnsMigratesExistingSchemas(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "old-content.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE notes (id INTEGER PRIMARY KEY, title TEXT NOT NULL); CREATE TABLE posts (id INTEGER PRIMARY KEY, title TEXT NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	if err := ensureContentTagsColumns(db); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"notes", "posts"} {
		var found int
		query := `SELECT COUNT(*) FROM pragma_table_info('` + table + `') WHERE name = 'tags'`
		if err := db.QueryRow(query).Scan(&found); err != nil {
			t.Fatal(err)
		}
		if found != 1 {
			t.Fatalf("tags column was not added to %s", table)
		}
	}
}

func jsonNumber(value int64) string {
	return strconv.FormatInt(value, 10)
}
