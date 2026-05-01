package models

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDiscoverModels(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer testkey" {
			t.Error("missing auth header")
			w.WriteHeader(401)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"qwen-4b","owned_by":"darkbloom"},{"id":"llama-8b","owned_by":"darkbloom"}]}`))
	}))
	defer srv.Close()

	d := NewDiscoverer(srv.URL, "testkey")
	if err := d.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if n := d.Count(); n != 2 {
		t.Errorf("Count = %d, want 2", n)
	}
	models := d.Models()
	if len(models) != 2 {
		t.Errorf("Models len = %d, want 2", len(models))
	}
	if models[0].ID != "qwen-4b" {
		t.Errorf("Models[0].ID = %q, want %q", models[0].ID, "qwen-4b")
	}
}

func TestDiscoverModelsEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer srv.Close()

	d := NewDiscoverer(srv.URL, "key")
	if err := d.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if n := d.Count(); n != 0 {
		t.Errorf("Count = %d, want 0", n)
	}
}
