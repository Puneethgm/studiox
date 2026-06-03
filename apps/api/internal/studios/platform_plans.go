package studios

import (
	"encoding/json"
	"net/http"

	"github.com/projectx/api/internal/platform/httpx"
)

func (h *Handler) GetPlatformPlans(w http.ResponseWriter, r *http.Request) {
	val, err := h.svc.GetPlatformSetting(r.Context(), "platform_plans")
	if err != nil || val == "" {
		// Fallback defaults
		val = `[{"name":"Trial Pass","price":300,"cycle":"One-time","description":"Entry-level setup to validate AI integration and lead generation.","features":["1 Connected Channel","Basic AI Auto-Replies (200/mo)","1-day automated follow-up","Google Sheets contact sync"]},{"name":"Growth Tier","price":999,"cycle":"Monthly","description":"Automate workflows, payments, and client acquisition.","features":["3 Connected Channels","Full AI Auto-Replies (2,000/mo)","Dedicated Knowledge Base","Visual drag-and-drop Pipeline","Stripe account integration"]},{"name":"Pro Tier","price":1299,"cycle":"Monthly","description":"For active studios looking to scale reach via social media and paid advertising.","features":["8 Connected Channels","Extended AI Auto-Replies (10,000/mo)","Dual model routing (Gemini + Claude)","Advanced Social Planner","Google Ads Channel Integration","Studio Plan Option (Scheduling)"],"highlight":true},{"name":"Enterprise Tier","price":1599,"cycle":"Monthly","description":"Maximum scale, custom branding, and multi-location management.","features":["Unlimited Connected Channels","Unlimited AI Auto-Replies","Multi-Location Hub","Whitelabel Dashboard","Enterprise Studio Plan Option","Priority Support SLA"]}]`
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(val))
}

func (h *Handler) UpdatePlatformPlans(w http.ResponseWriter, r *http.Request) {
	var payload []map[string]any
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_json", "failed to decode request body")
		return
	}

	b, _ := json.Marshal(payload)
	if err := h.svc.UpdatePlatformSetting(r.Context(), "platform_plans", string(b)); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "db_error", "failed to update platform plans")
		return
	}

	httpx.JSON(w, http.StatusOK, map[string]any{"ok": true})
}
