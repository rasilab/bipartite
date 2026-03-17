package pubmed

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestGetPaper(t *testing.T) {
	xmlData, err := os.ReadFile("../../testdata/pubmed/efetch_response.xml")
	if err != nil {
		t.Fatalf("reading test fixture: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/efetch.fcgi" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("db") != "pubmed" {
			t.Errorf("expected db=pubmed, got %s", r.URL.Query().Get("db"))
		}
		if r.URL.Query().Get("id") != "19872477" {
			t.Errorf("expected id=19872477, got %s", r.URL.Query().Get("id"))
		}
		w.Header().Set("Content-Type", "application/xml")
		w.Write(xmlData)
	}))
	defer server.Close()

	client := NewClient(WithBaseURL(server.URL))
	article, err := client.GetPaper(context.Background(), "19872477")
	if err != nil {
		t.Fatalf("GetPaper: %v", err)
	}

	if article.MedlineCitation.PMID.Value != "19872477" {
		t.Errorf("PMID = %q, want %q", article.MedlineCitation.PMID.Value, "19872477")
	}
	if article.MedlineCitation.Article.Title != "Genome-wide analysis in vivo of translation with nucleotide resolution using ribosome profiling." {
		t.Errorf("Title = %q", article.MedlineCitation.Article.Title)
	}
	if len(article.MedlineCitation.Article.AuthorList.Authors) != 4 {
		t.Errorf("len(Authors) = %d, want 4", len(article.MedlineCitation.Article.AuthorList.Authors))
	}
}

func TestGetPaper_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// PubMed returns an empty article set for invalid PMIDs
		w.Header().Set("Content-Type", "application/xml")
		w.Write([]byte(`<?xml version="1.0" ?><PubmedArticleSet></PubmedArticleSet>`))
	}))
	defer server.Close()

	client := NewClient(WithBaseURL(server.URL))
	_, err := client.GetPaper(context.Background(), "99999999999")
	if !IsNotFound(err) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestSearch(t *testing.T) {
	jsonData, err := os.ReadFile("../../testdata/pubmed/esearch_response.json")
	if err != nil {
		t.Fatalf("reading test fixture: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/esearch.fcgi" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("db") != "pubmed" {
			t.Errorf("expected db=pubmed, got %s", r.URL.Query().Get("db"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(jsonData)
	}))
	defer server.Close()

	client := NewClient(WithBaseURL(server.URL))
	result, err := client.Search(context.Background(), "ribosome profiling", 5)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}

	if result.Count != "142" {
		t.Errorf("Count = %q, want %q", result.Count, "142")
	}
	if len(result.IDList) != 5 {
		t.Errorf("len(IDList) = %d, want 5", len(result.IDList))
	}
	if result.IDList[0] != "19872477" {
		t.Errorf("IDList[0] = %q, want %q", result.IDList[0], "19872477")
	}
}

func TestGetSummaries(t *testing.T) {
	jsonData, err := os.ReadFile("../../testdata/pubmed/esummary_response.json")
	if err != nil {
		t.Fatalf("reading test fixture: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/esummary.fcgi" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(jsonData)
	}))
	defer server.Close()

	client := NewClient(WithBaseURL(server.URL))
	summaries, err := client.GetSummaries(context.Background(), []string{"19872477"})
	if err != nil {
		t.Fatalf("GetSummaries: %v", err)
	}

	doc, ok := summaries["19872477"]
	if !ok {
		t.Fatal("expected summary for PMID 19872477")
	}
	if doc.Title != "Genome-wide analysis in vivo of translation with nucleotide resolution using ribosome profiling." {
		t.Errorf("Title = %q", doc.Title)
	}
	if doc.Source != "Science" {
		t.Errorf("Source = %q, want %q", doc.Source, "Science")
	}
	if len(doc.Authors) != 4 {
		t.Errorf("len(Authors) = %d, want 4", len(doc.Authors))
	}
}

func TestRateLimitError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "10")
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte("Rate limit exceeded"))
	}))
	defer server.Close()

	client := NewClient(WithBaseURL(server.URL))
	_, err := client.GetPaper(context.Background(), "12345")
	if !IsRateLimited(err) {
		t.Errorf("expected rate limit error, got %v", err)
	}
}

func TestAuthParams(t *testing.T) {
	var gotKey, gotEmail string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.URL.Query().Get("api_key")
		gotEmail = r.URL.Query().Get("email")
		w.Header().Set("Content-Type", "application/xml")
		w.Write([]byte(`<?xml version="1.0" ?><PubmedArticleSet></PubmedArticleSet>`))
	}))
	defer server.Close()

	client := NewClient(
		WithBaseURL(server.URL),
		WithAPIKey("test-key"),
		WithEmail("test@example.com"),
	)
	client.GetPaper(context.Background(), "12345")

	if gotKey != "test-key" {
		t.Errorf("api_key = %q, want %q", gotKey, "test-key")
	}
	if gotEmail != "test@example.com" {
		t.Errorf("email = %q, want %q", gotEmail, "test@example.com")
	}
}
