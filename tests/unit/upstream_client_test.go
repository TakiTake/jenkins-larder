package unit

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yourorg/jenkins-larder/src/upstream"
)

func TestCheckPluginExists(t *testing.T) {
	t.Run("returns true for 200", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "HEAD" {
				t.Errorf("expected HEAD, got %s", r.Method)
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer ts.Close()

		client := upstream.NewClient(ts.URL, 10)
		exists, err := client.CheckPluginExists(context.Background(), "git", "4.11.0", "hpi")
		if err != nil {
			t.Fatal(err)
		}
		if !exists {
			t.Error("expected exists=true")
		}
	})

	t.Run("returns false for 404", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer ts.Close()

		client := upstream.NewClient(ts.URL, 10)
		exists, err := client.CheckPluginExists(context.Background(), "git", "99.0.0", "hpi")
		if err != nil {
			t.Fatal(err)
		}
		if exists {
			t.Error("expected exists=false")
		}
	})

	t.Run("returns error for unexpected status", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer ts.Close()

		client := upstream.NewClient(ts.URL, 10)
		_, err := client.CheckPluginExists(context.Background(), "git", "1.0", "hpi")
		if err == nil {
			t.Fatal("expected error for 500 status")
		}
	})

	t.Run("returns error on connection failure", func(t *testing.T) {
		client := upstream.NewClient("http://127.0.0.1:1", 10)
		_, err := client.CheckPluginExists(context.Background(), "git", "1.0", "hpi")
		if err == nil {
			t.Fatal("expected error on connection failure")
		}
	})
}

func TestDownloadPluginErrors(t *testing.T) {
	t.Run("returns error for non-200 status", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer ts.Close()

		client := upstream.NewClient(ts.URL, 10)
		_, err := client.DownloadPlugin(context.Background(), "nonexistent", "1.0", "hpi")
		if err == nil {
			t.Fatal("expected error for 404")
		}
	})

	t.Run("returns error on connection failure", func(t *testing.T) {
		client := upstream.NewClient("http://127.0.0.1:1", 10)
		_, err := client.DownloadPlugin(context.Background(), "git", "1.0", "hpi")
		if err == nil {
			t.Fatal("expected error on connection failure")
		}
	})
}

func TestGetPluginMetadata(t *testing.T) {
	client := upstream.NewClient("https://updates.jenkins.io", 10)
	_, err := client.GetPluginMetadata(context.Background(), "git")
	if err == nil {
		t.Fatal("expected 'not implemented' error")
	}
}
