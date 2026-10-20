package server

import (
	"crypto/rand"
	"crypto/subtle"
	"net/http"
	"strings"
	"time"
)

func (a *application) signUp(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	input.Username = strings.ToLower(strings.TrimSpace(input.Username))
	if !usernamePattern.MatchString(input.Username) || len(input.Password) < 10 || len(input.Password) > 256 {
		writeError(w, http.StatusBadRequest, "Use a 3-32 character username and a password of at least 10 characters.")
		return
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		writeError(w, http.StatusInternalServerError, "Could not create account.")
		return
	}
	hash := derivePassword(input.Password, salt)
	result, err := a.db.Exec(`INSERT INTO users(username, salt, password_hash) VALUES(?, ?, ?)`, input.Username, salt, hash)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			writeError(w, http.StatusConflict, "That username is already taken.")
			return
		}
		writeError(w, http.StatusInternalServerError, "Could not create account.")
		return
	}
	userID, err := result.LastInsertId()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Could not create account.")
		return
	}
	if err := a.createUserFolders(input.Username); err != nil {
		_, _ = a.db.Exec(`DELETE FROM users WHERE id = ?`, userID)
		writeError(w, http.StatusInternalServerError, "Could not prepare private storage.")
		return
	}
	if err := a.createSession(w, r, userID); err != nil {
		writeError(w, http.StatusInternalServerError, "Account created, but sign-in failed.")
		return
	}
	writeJSON(w, http.StatusCreated, user{ID: userID, Username: input.Username})
}

func (a *application) signIn(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	var current user
	var salt, expected []byte
	err := a.db.QueryRow(`SELECT id, username, salt, password_hash FROM users WHERE username = ?`, strings.ToLower(strings.TrimSpace(input.Username))).Scan(&current.ID, &current.Username, &salt, &expected)
	actual := derivePassword(input.Password, salt)
	if err != nil || subtle.ConstantTimeCompare(actual, expected) != 1 {
		writeError(w, http.StatusUnauthorized, "Username or password is incorrect.")
		return
	}
	if err := a.createUserFolders(current.Username); err != nil {
		writeError(w, http.StatusInternalServerError, "Could not access private storage.")
		return
	}
	if err := a.createSession(w, r, current.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "Could not sign in.")
		return
	}
	writeJSON(w, http.StatusOK, current)
}

func (a *application) signOut(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie("nebula_session"); err == nil {
		_, _ = a.db.Exec(`DELETE FROM sessions WHERE token_hash = ?`, hashToken(cookie.Value))
	}
	http.SetCookie(w, &http.Cookie{Name: "nebula_session", Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: a.secureCookie(r), SameSite: http.SameSiteStrictMode})
	w.WriteHeader(http.StatusNoContent)
}

func (a *application) me(w http.ResponseWriter, r *http.Request) {
	current, ok := a.currentUser(w, r)
	if ok {
		writeJSON(w, http.StatusOK, current)
	}
}

func (a *application) currentUser(w http.ResponseWriter, r *http.Request) (user, bool) {
	cookie, err := r.Cookie("nebula_session")
	if err != nil || cookie.Value == "" {
		writeError(w, http.StatusUnauthorized, "Sign in to continue.")
		return user{}, false
	}
	var current user
	var avatarName string
	err = a.db.QueryRow(`SELECT users.id, users.username, COALESCE(profiles.theme, 'light'), COALESCE(profiles.avatar_name, '') FROM sessions JOIN users ON users.id = sessions.user_id LEFT JOIN profiles ON profiles.user_id = users.id WHERE sessions.token_hash = ? AND sessions.expires_at > ?`, hashToken(cookie.Value), time.Now().UTC()).Scan(&current.ID, &current.Username, &current.Theme, &avatarName)
	if err != nil {
		http.SetCookie(w, &http.Cookie{Name: "nebula_session", Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: a.secureCookie(r), SameSite: http.SameSiteStrictMode})
		writeError(w, http.StatusUnauthorized, "Your session has expired. Sign in again.")
		return user{}, false
	}
	if avatarName != "" {
		current.AvatarURL = "/api/avatar"
	}
	return current, true
}

func (a *application) createSession(w http.ResponseWriter, r *http.Request, userID int64) error {
	token := randomName() + randomName()
	expires := time.Now().UTC().Add(sessionAge)
	if _, err := a.db.Exec(`INSERT INTO sessions(token_hash, user_id, expires_at) VALUES(?, ?, ?)`, hashToken(token), userID, expires); err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{Name: "nebula_session", Value: token, Path: "/", Expires: expires, MaxAge: int(sessionAge.Seconds()), HttpOnly: true, Secure: a.secureCookie(r), SameSite: http.SameSiteStrictMode})
	return nil
}

func (a *application) secureCookie(r *http.Request) bool {
	if !a.secure {
		return false
	}
	return r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") || env("COOKIE_SECURE", "auto") == "true"
}
