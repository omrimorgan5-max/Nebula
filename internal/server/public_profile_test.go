package server

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPublicProfileRequiresIndependentOptIns(t *testing.T) {
	_, handler := testApplication(t)
	server := httptest.NewServer(handler)
	defer server.Close()
	client := testClient(t, server, "profile_owner")

	var settings profileSettings
	sendJSON(t, client, http.MethodGet, server.URL+"/api/profile", "", http.StatusOK, &settings)
	if settings.PublicEnabled || settings.Accent != "moss" {
		t.Fatalf("unexpected initial profile settings: %+v", settings)
	}
	response, err := client.Get(server.URL + "/u/profile_owner")
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("private profile returned %d, want 404", response.StatusCode)
	}

	var item post
	sendJSON(t, client, http.MethodPost, server.URL+"/api/posts", `{"title":"Private draft","content":"Secret copy","status":"published"}`, http.StatusCreated, &item)
	sendJSON(t, client, http.MethodPut, server.URL+"/api/profile", `{"displayName":"Robin North","bio":"Writer and collector.","accent":"ocean","publicEnabled":true,"showAvatar":false}`, http.StatusOK, &settings)
	if settings.PublicURL != "/u/profile_owner" || settings.Accent != "ocean" {
		t.Fatalf("profile customization was not saved: %+v", settings)
	}
	response, err = client.Get(server.URL + "/u/profile_owner")
	if err != nil {
		t.Fatal(err)
	}
	privatePage, _ := io.ReadAll(response.Body)
	response.Body.Close()
	if response.StatusCode != http.StatusOK || !bytes.Contains(privatePage, []byte("Robin North")) || bytes.Contains(privatePage, []byte("Private draft")) || bytes.Contains(privatePage, []byte("Secret copy")) {
		t.Fatalf("profile page exposed unexpected content: status=%d body=%s", response.StatusCode, privatePage)
	}
	response, err = client.Get(server.URL + "/u/profile_owner/avatar")
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("hidden avatar returned %d, want 404", response.StatusCode)
	}

	sendJSON(t, client, http.MethodPut, server.URL+"/api/posts/"+jsonNumber(item.ID), `{"title":"Public story","content":"<p>Shared copy.</p>","status":"published","isPublic":true}`, http.StatusNoContent, nil)
	response, err = client.Get(server.URL + "/u/profile_owner")
	if err != nil {
		t.Fatal(err)
	}
	publicPage, _ := io.ReadAll(response.Body)
	response.Body.Close()
	if response.StatusCode != http.StatusOK || !bytes.Contains(publicPage, []byte("Public story")) || !bytes.Contains(publicPage, []byte("Shared copy.")) {
		t.Fatalf("explicitly public post was not rendered: status=%d body=%s", response.StatusCode, publicPage)
	}

	sendJSON(t, client, http.MethodPut, server.URL+"/api/profile", `{"displayName":"Robin North","bio":"Writer and collector.","accent":"ocean","publicEnabled":false,"showAvatar":false}`, http.StatusOK, nil)
	response, err = client.Get(server.URL + "/u/profile_owner")
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("disabled profile returned %d, want 404", response.StatusCode)
	}
	sendJSON(t, client, http.MethodPut, server.URL+"/api/profile", `{"displayName":"Robin North","bio":"Writer and collector.","accent":"ocean","publicEnabled":true,"showAvatar":false}`, http.StatusOK, nil)
	response, err = client.Get(server.URL + "/u/profile_owner")
	if err != nil {
		t.Fatal(err)
	}
	revivedPage, _ := io.ReadAll(response.Body)
	response.Body.Close()
	if response.StatusCode != http.StatusOK || bytes.Contains(revivedPage, []byte("Public story")) {
		t.Fatalf("disabled post was unintentionally republished: status=%d body=%s", response.StatusCode, revivedPage)
	}
}
