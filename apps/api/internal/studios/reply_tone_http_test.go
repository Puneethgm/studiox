package studios

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func newReplyToneTestHandler(t *testing.T) (*Handler, uuid.UUID) {
	t.Helper()
	repo, pool := setupAIKeyStoreTest(t)
	ctx := context.Background()
	var studioID uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT id FROM studios LIMIT 1`).Scan(&studioID); err != nil {
		t.Skip("skipping test; no studio found in DB")
	}
	original, err := repo.GetByID(ctx, studioID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	t.Cleanup(func() { _ = repo.SetReplyTone(context.Background(), studioID, original.ReplyTone) })
	return &Handler{svc: &Service{repo: repo}}, studioID
}

func reqWithStudioID(method, path, body string, studioID uuid.UUID) *http.Request {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("studioId", studioID.String())
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

// TestPutReplyTone_ValidAndInvalid covers: a valid tone is accepted and
// persisted, and an invalid one is rejected with 422 — without changing
// the already-stored value.
func TestPutReplyTone_ValidAndInvalid(t *testing.T) {
	h, studioID := newReplyToneTestHandler(t)
	ctx := context.Background()

	req := reqWithStudioID(http.MethodPut, "/api/v1/studios/"+studioID.String()+"/reply-tone", `{"replyTone":"casual"}`, studioID)
	rr := httptest.NewRecorder()
	h.PutReplyTone(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("valid tone: status = %d, body = %s", rr.Code, rr.Body.String())
	}
	got, err := h.svc.repo.GetByID(ctx, studioID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.ReplyTone != "casual" {
		t.Errorf("after PUT casual, ReplyTone = %q, want casual", got.ReplyTone)
	}

	req2 := reqWithStudioID(http.MethodPut, "/api/v1/studios/"+studioID.String()+"/reply-tone", `{"replyTone":"sarcastic"}`, studioID)
	rr2 := httptest.NewRecorder()
	h.PutReplyTone(rr2, req2)
	if rr2.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid tone: status = %d, want 422, body = %s", rr2.Code, rr2.Body.String())
	}
	stillCasual, err := h.svc.repo.GetByID(ctx, studioID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if stillCasual.ReplyTone != "casual" {
		t.Errorf("an invalid PUT must not change the stored value — got %q, want still casual", stillCasual.ReplyTone)
	}
}
