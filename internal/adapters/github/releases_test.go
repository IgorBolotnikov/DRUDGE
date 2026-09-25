package github

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const testTimeout = 5 * time.Second

func TestLatestVersion(t *testing.T) {
	cases := []struct {
		name        string
		handler     http.HandlerFunc
		wantVersion string
		wantErr     string
	}{
		{
			name: "reads the tag from the redirect",
			handler: func(writer http.ResponseWriter, request *http.Request) {
				http.Redirect(writer, request, "/IgorBolotnikov/DRUDGE/releases/tag/v0.1.1", http.StatusFound)
			},
			wantVersion: "v0.1.1",
		},
		{
			name: "reads the tag from an absolute redirect",
			handler: func(writer http.ResponseWriter, request *http.Request) {
				http.Redirect(writer, request, "https://github.com/IgorBolotnikov/DRUDGE/releases/tag/v0.2.0", http.StatusFound)
			},
			wantVersion: "v0.2.0",
		},
		{
			name: "refuses an answer without a redirect",
			handler: func(writer http.ResponseWriter, request *http.Request) {
				http.NotFound(writer, request)
			},
			wantErr: "answered 404 Not Found, expected a redirect",
		},
		{
			name: "refuses a redirect without a location",
			handler: func(writer http.ResponseWriter, request *http.Request) {
				writer.WriteHeader(http.StatusFound)
			},
			wantErr: "the latest release redirect has no location",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			var gotPath string
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				gotPath = request.URL.Path
				testCase.handler(writer, request)
			}))
			defer server.Close()

			version, err := New(server.URL, testTimeout).LatestVersion()

			if gotPath != latestReleasePath {
				t.Errorf("requested %q, want %q", gotPath, latestReleasePath)
			}
			if testCase.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), testCase.wantErr) {
					t.Fatalf("expected an error containing %q, got %v", testCase.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if version != testCase.wantVersion {
				t.Errorf("version = %q, want %q", version, testCase.wantVersion)
			}
		})
	}
}

func TestDownloadFile(t *testing.T) {
	cases := []struct {
		name         string
		status       int
		body         string
		wantContents string
		wantErr      string
	}{
		{name: "returns the file", status: http.StatusOK, body: "archive bytes", wantContents: "archive bytes"},
		{name: "refuses a missing file", status: http.StatusNotFound, body: "Not Found", wantErr: "answered 404 Not Found"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			var gotPath string
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				gotPath = request.URL.Path
				writer.WriteHeader(testCase.status)
				writer.Write([]byte(testCase.body))
			}))
			defer server.Close()

			contents, err := New(server.URL, testTimeout).DownloadFile("v0.1.1", "drg_linux_arm64.tar.gz")

			if wantPath := "/releases/download/v0.1.1/drg_linux_arm64.tar.gz"; gotPath != wantPath {
				t.Errorf("requested %q, want %q", gotPath, wantPath)
			}
			if testCase.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), testCase.wantErr) {
					t.Fatalf("expected an error containing %q, got %v", testCase.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if string(contents) != testCase.wantContents {
				t.Errorf("contents = %q, want %q", contents, testCase.wantContents)
			}
		})
	}
}
