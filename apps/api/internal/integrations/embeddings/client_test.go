package embeddings

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestEmbedQueryPassagePrefixFlag guards against the one failure mode that
// wouldn't be caught by a compile error or an obvious crash: EmbedQuery and
// EmbedPassage silently sending the wrong is_query value, which would
// silently degrade retrieval quality rather than fail loudly.
func TestEmbedQueryPassagePrefixFlag(t *testing.T) {
	var gotIsQuery bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req embedRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		gotIsQuery = req.IsQuery
		json.NewEncoder(w).Encode(embedResponse{Embedding: []float32{0.1, 0.2, 0.3}})
	}))
	defer srv.Close()

	c := New(srv.URL)

	vec, err := c.EmbedQuery(t.Context(), "how much does a trial cost")
	if err != nil {
		t.Fatalf("EmbedQuery: %v", err)
	}
	if !gotIsQuery {
		t.Error("EmbedQuery did not send is_query=true")
	}
	if len(vec) != 3 {
		t.Errorf("expected 3-dim vector, got %d", len(vec))
	}

	if _, err := c.EmbedPassage(t.Context(), "trials cost $19 for the first week"); err != nil {
		t.Fatalf("EmbedPassage: %v", err)
	}
	if gotIsQuery {
		t.Error("EmbedPassage sent is_query=true")
	}
}

func TestNewReturnsNilForEmptyBaseURL(t *testing.T) {
	if New("") != nil {
		t.Error("New(\"\") should return nil")
	}
}

func TestEmbedNilReceiver(t *testing.T) {
	var c *Client
	if _, err := c.Embed(t.Context(), "text", false); err == nil {
		t.Error("expected error calling Embed on nil client")
	}
}
