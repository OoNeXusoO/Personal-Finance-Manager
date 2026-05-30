package transactions

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"personal-finance-manager/internal/auth"
	"personal-finance-manager/internal/models"
	"personal-finance-manager/pkg/response"
)

type Handler struct {
	svc       Service
	repo      Repository
	uploadDir string
	maxSize   int64
}

func NewHandler(svc Service, repo Repository, uploadDir string, maxUploadSize int64) *Handler {
	return &Handler{svc: svc, repo: repo, uploadDir: uploadDir, maxSize: maxUploadSize}
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	q := r.URL.Query()

	f := Filter{
		Type:    q.Get("type"),
		Page:    intQuery(q.Get("page"), 1),
		PerPage: intQuery(q.Get("per_page"), 20),
	}
	if cat := q.Get("category"); cat != "" {
		if id, err := strconv.ParseInt(cat, 10, 64); err == nil {
			f.Category = id
		}
	}
	if from := q.Get("from"); from != "" {
		if t, err := time.Parse("2006-01-02", from); err == nil {
			f.From = t
		}
	}
	if to := q.Get("to"); to != "" {
		if t, err := time.Parse("2006-01-02", to); err == nil {
			f.To = t
		}
	}

	txs, total, err := h.svc.List(r.Context(), userID, f)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to list transactions")
		return
	}
	if txs == nil {
		txs = []models.Transaction{}
	}
	response.JSONWithMeta(w, http.StatusOK, txs, &response.Meta{
		Page:    f.Page,
		PerPage: f.PerPage,
		Total:   total,
	})
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())

	var req models.CreateTransactionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	tx, err := h.svc.Create(r.Context(), userID, &req)
	if err != nil {
		if isValidationErr(err) {
			response.Error(w, http.StatusBadRequest, err.Error())
			return
		}
		response.Error(w, http.StatusInternalServerError, "failed to create transaction")
		return
	}
	response.JSON(w, http.StatusCreated, tx)
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	id := urlParamInt64(r, "id")

	var req models.UpdateTransactionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	tx, err := h.svc.Update(r.Context(), id, userID, &req)
	if err != nil {
		switch {
		case errors.Is(err, ErrNotFound):
			response.Error(w, http.StatusNotFound, "transaction not found")
		case isValidationErr(err):
			response.Error(w, http.StatusBadRequest, err.Error())
		default:
			response.Error(w, http.StatusInternalServerError, "failed to update transaction")
		}
		return
	}
	response.JSON(w, http.StatusOK, tx)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	id := urlParamInt64(r, "id")

	if err := h.svc.Delete(r.Context(), id, userID); err != nil {
		if errors.Is(err, ErrNotFound) {
			response.Error(w, http.StatusNotFound, "transaction not found")
			return
		}
		response.Error(w, http.StatusInternalServerError, "failed to delete transaction")
		return
	}
	response.NoContent(w)
}

func (h *Handler) UploadReceipt(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	txID := urlParamInt64(r, "id")

	if _, err := h.svc.GetByID(r.Context(), txID, userID); err != nil {
		if errors.Is(err, ErrNotFound) {
			response.Error(w, http.StatusNotFound, "transaction not found")
			return
		}
		response.Error(w, http.StatusInternalServerError, "internal error")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, h.maxSize)
	if err := r.ParseMultipartForm(h.maxSize); err != nil {
		response.Error(w, http.StatusBadRequest, "file too large or invalid form")
		return
	}

	file, header, err := r.FormFile("receipt")
	if err != nil {
		response.Error(w, http.StatusBadRequest, "field 'receipt' is required")
		return
	}
	defer file.Close()

	ct := header.Header.Get("Content-Type")
	if ct != "image/jpeg" && ct != "image/png" && ct != "image/gif" && ct != "image/webp" {
		response.Error(w, http.StatusBadRequest, "only image files are accepted (jpeg, png, gif, webp)")
		return
	}

	ext := filepath.Ext(header.Filename)
	uniqueName := fmt.Sprintf("%d_%d_%d%s", userID, txID, time.Now().UnixNano(), ext)
	destPath := filepath.Join(h.uploadDir, uniqueName)

	if err := os.MkdirAll(h.uploadDir, 0755); err != nil {
		response.Error(w, http.StatusInternalServerError, "upload directory error")
		return
	}

	dest, err := os.Create(destPath)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to save file")
		return
	}
	defer dest.Close()

	written, err := io.Copy(dest, file)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to write file")
		return
	}

	attachment := &models.Attachment{
		TransactionID: txID,
		UserID:        userID,
		FileName:      header.Filename,
		FilePath:      uniqueName,
		FileSize:      written,
		ContentType:   ct,
	}
	if err := h.repo.AddAttachment(r.Context(), attachment); err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to save attachment record")
		return
	}

	response.JSON(w, http.StatusCreated, attachment)
}

func (h *Handler) DownloadAttachment(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	attachmentID := urlParamInt64(r, "attachmentId")

	att, err := h.repo.GetAttachmentByID(r.Context(), attachmentID, userID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.Error(w, http.StatusNotFound, "attachment not found")
			return
		}
		response.Error(w, http.StatusInternalServerError, "failed to get attachment")
		return
	}

	filePath := filepath.Join(h.uploadDir, att.FilePath)
	w.Header().Set("Content-Type", att.ContentType)
	w.Header().Set("Content-Disposition", `attachment; filename="`+att.FileName+`"`)
	w.Header().Set("Content-Length", strconv.FormatInt(att.FileSize, 10))
	http.ServeFile(w, r, filePath)
}

func (h *Handler) GetAttachments(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	txID := urlParamInt64(r, "id")

	list, err := h.repo.ListAttachments(r.Context(), txID, userID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to list attachments")
		return
	}
	if list == nil {
		list = []models.Attachment{}
	}
	response.JSON(w, http.StatusOK, list)
}

func (h *Handler) GetBalance(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	currency := r.URL.Query().Get("currency")
	if currency == "" {
		currency = "PLN"
	}

	b, err := h.svc.Balance(r.Context(), userID, currency)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to calculate balance")
		return
	}
	response.JSON(w, http.StatusOK, b)
}

func urlParamInt64(r *http.Request, key string) int64 {
	v, _ := strconv.ParseInt(chi.URLParam(r, key), 10, 64)
	return v
}

func intQuery(s string, def int) int {
	if v, err := strconv.Atoi(s); err == nil && v > 0 {
		return v
	}
	return def
}

func isValidationErr(err error) bool {
	var ve models.ValidationError
	return errors.As(err, &ve)
}
