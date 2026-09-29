package studios

import (
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/projectx/api/internal/integrations/mistral"
	"github.com/projectx/api/internal/platform/httpx"
)

var ocrAllowedExt = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".webp": true,
}

// OCRKnowledgeBaseImage godoc
//
//	@Summary		Extract text from an image for the Knowledge Base
//	@Description	Runs Mistral OCR on an uploaded image and returns the extracted text. Does NOT save anything to the knowledge base or trigger embedding — this is the review step; the admin confirms the (editable) text client-side before it's added via the normal knowledge-base save, which is what actually triggers embedding.
//	@Tags			Knowledge Base
//	@Security		CookieAuth
//	@Accept			multipart/form-data
//	@Produce		json
//	@Param			studioId	path		string	true	"Studio ID"
//	@Param			file		formData	file	true	"Image to OCR (jpg/jpeg/png/webp)"
//	@Success		200			{object}	map[string]interface{}
//	@Failure		400			{object}	httpx.ErrorResponse	"missing Mistral API key, unsupported file type, or missing file"
//	@Failure		500			{object}	httpx.ErrorResponse
//	@Failure		502			{object}	httpx.ErrorResponse	"OCR request failed"
//	@Router			/api/v1/studios/{studioId}/knowledge-base/ocr [post]
func (h *Handler) OCRKnowledgeBaseImage(w http.ResponseWriter, r *http.Request) {
	studioID, err := uuid.Parse(chi.URLParam(r, "studioId"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_id", "invalid id")
		return
	}

	const maxSize = 20 << 20 // 20 MB
	if err := r.ParseMultipartForm(maxSize); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "file too large or bad multipart form")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "missing file field")
		return
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(header.Filename))
	contentType := header.Header.Get("Content-Type")
	if ext == "" {
		exts, _ := mime.ExtensionsByType(contentType)
		if len(exts) > 0 {
			ext = strings.ToLower(exts[0])
		}
	}
	if !ocrAllowedExt[ext] {
		httpx.WriteError(w, http.StatusBadRequest, "bad_request", "unsupported file type — only jpg/jpeg/png/webp images can be OCR'd")
		return
	}
	if contentType == "" {
		contentType = "image/" + strings.TrimPrefix(ext, ".")
	}

	imageBytes, err := io.ReadAll(file)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "failed to read uploaded file")
		return
	}

	studio, err := h.svc.GetByID(r.Context(), studioID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal", "failed to load studio config")
		return
	}
	if studio.MistralAPIKey == "" {
		httpx.WriteError(w, http.StatusBadRequest, "missing_api_key", "Please configure your Mistral API Key in Settings → AI Assistant to extract text from images.")
		return
	}

	text, err := mistral.New().OCRImage(r.Context(), studio.MistralAPIKey, imageBytes, contentType)
	if err != nil {
		httpx.WriteError(w, http.StatusBadGateway, "ocr_failed", "Text extraction failed: "+err.Error())
		return
	}

	httpx.JSON(w, http.StatusOK, map[string]any{"text": text})
}
