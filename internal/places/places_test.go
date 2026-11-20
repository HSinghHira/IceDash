package places

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFind(t *testing.T) {
	var asked string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked = r.URL.Query().Get("name") + "|" + r.URL.Query().Get("language")
		_, _ = w.Write([]byte(`{"results":[{"name":"Everett","admin1":"Washington","country":"United States","latitude":47.97898,"longitude":-122.20208},{"name":"Everett","admin1":"Massachusetts","country":"United States","latitude":42.40843,"longitude":-71.05366}]}`))
	}))
	defer srv.Close()
	s := &Search{HTTP: srv.Client(), Base: srv.URL}
	got, err := s.Find(context.Background(), " Everett, WA ", "")
	if err != nil {
		t.Fatal(err)
	}
	if asked != "Everett|en" || len(got) != 2 || got[0].Lat != 47.97898 || got[0].Label() != "Everett, Washington, United States" {
		t.Fatalf("asked %q, got %+v", asked, got)
	}
	if _, err := s.Find(context.Background(), "x", "en"); !errors.Is(err, ErrQuery) {
		t.Fatalf("short name: %v", err)
	}
}

func TestFindNone(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(`{"generationtime_ms":0.1}`)) }))
	defer srv.Close()
	got, err := (&Search{HTTP: srv.Client(), Base: srv.URL}).Find(context.Background(), "Nowhereville", "en")
	if err != nil || got == nil || len(got) != 0 {
		t.Fatalf("got %v %v", got, err)
	}
}
