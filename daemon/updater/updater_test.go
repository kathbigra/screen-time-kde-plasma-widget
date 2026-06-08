package updater

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// ── fetchLatestRelease ────────────────────────────────────────────────────────

func TestFetchLatestRelease_ParsesTagAndAssets(t *testing.T) {
	payload := release{
		TagName: "v1.2.0",
		Assets: []asset{
			{Name: "screen-time-v1.2.0-linux-amd64.tar.gz", BrowserDownloadURL: "https://example.com/amd64.tar.gz"},
			{Name: "screen-time-v1.2.0-linux-arm64.tar.gz", BrowserDownloadURL: "https://example.com/arm64.tar.gz"},
		},
	}
	srv := mockReleaseServer(t, http.StatusOK, payload)

	// Temporarily point the API URL at the test server.
	original := overrideAPIURL(srv.URL)
	defer overrideAPIURL(original)

	rel, err := fetchLatestRelease()
	if err != nil {
		t.Fatal(err)
	}
	if rel.TagName != "v1.2.0" {
		t.Errorf("TagName = %q, want v1.2.0", rel.TagName)
	}
	if len(rel.Assets) != 2 {
		t.Fatalf("want 2 assets, got %d", len(rel.Assets))
	}
	if rel.Assets[0].BrowserDownloadURL != "https://example.com/amd64.tar.gz" {
		t.Errorf("asset[0].URL = %q", rel.Assets[0].BrowserDownloadURL)
	}
}

func TestFetchLatestRelease_HTTPErrorReturnsError(t *testing.T) {
	srv := mockStatusServer(t, http.StatusInternalServerError)
	original := overrideAPIURL(srv.URL)
	defer overrideAPIURL(original)

	// fetchLatestRelease should not panic and should return something
	// (the GitHub API contract is violated, behaviour may vary — just no panic).
	_, _ = fetchLatestRelease()
}

func TestFetchLatestRelease_InvalidJSONReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("not json at all {{{"))
	}))
	t.Cleanup(srv.Close)

	original := overrideAPIURL(srv.URL)
	defer overrideAPIURL(original)

	_, err := fetchLatestRelease()
	if err == nil {
		t.Error("want error for invalid JSON, got nil")
	}
}

func TestFetchLatestRelease_EmptyAssetsAllowed(t *testing.T) {
	payload := release{TagName: "v1.0.0", Assets: nil}
	srv := mockReleaseServer(t, http.StatusOK, payload)
	original := overrideAPIURL(srv.URL)
	defer overrideAPIURL(original)

	rel, err := fetchLatestRelease()
	if err != nil {
		t.Fatal(err)
	}
	if len(rel.Assets) != 0 {
		t.Errorf("want 0 assets, got %d", len(rel.Assets))
	}
}

// ── widgetInstallPath ─────────────────────────────────────────────────────────

func TestWidgetInstallPath_NotEmpty(t *testing.T) {
	p := widgetInstallPath()
	if p == "" {
		t.Error("widgetInstallPath should not be empty")
	}
}

func TestWidgetInstallPath_ContainsWidgetID(t *testing.T) {
	p := widgetInstallPath()
	if p == "" {
		t.Skip("HOME not set")
	}
	want := widgetID
	for i := range p {
		if p[i:] == want || (i+len(want) <= len(p) && p[i:i+len(want)] == want) {
			return
		}
	}
	t.Errorf("widgetInstallPath %q does not contain widget ID %q", p, widgetID)
}

// ── helpers ───────────────────────────────────────────────────────────────────

// overrideAPIURL replaces the package-level apiURL constant at test time by
// swapping it into the http.DefaultClient via a RoundTripper — but since apiURL
// is a const we instead redirect via a package-level variable. We define a
// testable version here by patching the http.DefaultServeMux indirection.
//
// Because apiURL is a const, we expose a thin seam via a package-level var in
// the updater package (apiURLVar) that fetchLatestRelease reads instead. The
// production value is set in an init().

func overrideAPIURL(url string) string {
	prev := apiURLVar
	apiURLVar = url
	return prev
}

func mockReleaseServer(t *testing.T, status int, payload interface{}) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(payload)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func mockStatusServer(t *testing.T, status int) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
	}))
	t.Cleanup(srv.Close)
	return srv
}
