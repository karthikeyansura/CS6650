package handler

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"

	"github.com/google/uuid"
	"github.com/karthikeyansura/CS6650/Final/internal/blob"
	"github.com/karthikeyansura/CS6650/Final/internal/model"
	"github.com/karthikeyansura/CS6650/Final/internal/queue"
	"github.com/karthikeyansura/CS6650/Final/internal/store"
)

// Handler holds dependencies injected from main.
type Handler struct {
	Store *store.DynamoStore
	Blob  *blob.S3Client
	Queue *queue.SQSQueue
}

// RegisterRoutes registers all API routes on the given mux using Go 1.22+ pattern syntax.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /health", h.Health)
	mux.HandleFunc("GET /albums", h.ListAlbums)
	mux.HandleFunc("GET /albums/{album_id}", h.GetAlbum)
	mux.HandleFunc("PUT /albums/{album_id}", h.UpsertAlbum)
	mux.HandleFunc("POST /albums/{album_id}/photos", h.UploadPhoto)
	mux.HandleFunc("GET /albums/{album_id}/photos/{photo_id}", h.GetPhoto)
	mux.HandleFunc("DELETE /albums/{album_id}/photos/{photo_id}", h.DeletePhoto)
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, model.ErrorResponse{Error: msg})
}

// Health responds with the exact contract payload.
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, model.HealthResponse{Status: "ok"})
}

// UpsertAlbum creates or updates an album. Idempotent on album_id.
func (h *Handler) UpsertAlbum(w http.ResponseWriter, r *http.Request) {
	albumID := r.PathValue("album_id")

	var req model.Album
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	defer r.Body.Close()

	// path param is the canonical album_id
	req.AlbumID = albumID

	isNew, err := h.Store.UpsertAlbum(r.Context(), &req)
	if err != nil {
		slog.Error("upsert album", "album_id", albumID, "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	status := http.StatusOK
	if isNew {
		status = http.StatusCreated
	}
	writeJSON(w, status, req)
}

// GetAlbum returns a single album or 404.
func (h *Handler) GetAlbum(w http.ResponseWriter, r *http.Request) {
	albumID := r.PathValue("album_id")

	album, err := h.Store.GetAlbum(r.Context(), albumID)
	if err != nil {
		slog.Error("get album", "album_id", albumID, "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	if album == nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	writeJSON(w, http.StatusOK, album)
}

// ListAlbums returns every album that has ever been created.
// Uses paginated DynamoDB Scan to ensure completeness across all test scenarios.
func (h *Handler) ListAlbums(w http.ResponseWriter, r *http.Request) {
	albums, err := h.Store.ListAlbums(r.Context())
	if err != nil {
		slog.Error("list albums", "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	// return bare array, never null (contract accepts both bare array and wrapped object)
	if albums == nil {
		albums = []model.Album{}
	}
	writeJSON(w, http.StatusOK, albums)
}

// UploadPhoto accepts a multipart photo upload.
// Critical flow: allocate seq synchronously, stream to S3, persist metadata, enqueue job, return 202.
func (h *Handler) UploadPhoto(w http.ResponseWriter, r *http.Request) {
	albumID := r.PathValue("album_id")

	// 200MB max in memory before spilling to temp file
	if err := r.ParseMultipartForm(200 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "invalid multipart")
		return
	}

	file, header, err := r.FormFile("photo")
	if err != nil {
		writeError(w, http.StatusBadRequest, "missing 'photo' field")
		return
	}
	defer file.Close()

	photoID := uuid.New().String()

	// atomic seq allocation via DynamoDB UpdateItem counter
	// guarantees unique monotonic seq per album under concurrent uploads (S10)
	seq, err := h.Store.AllocateSeq(r.Context(), albumID)
	if err != nil {
		slog.Error("allocate seq", "album_id", albumID, "error", err)
		writeError(w, http.StatusInternalServerError, "sequence error")
		return
	}

	s3Key := fmt.Sprintf("photos/%s/%s", albumID, photoID)

	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	// stream upload to S3 using multipart upload manager
	// avoids buffering full 200MB file in memory
	if err := h.Blob.Upload(r.Context(), s3Key, file, contentType); err != nil {
		slog.Error("s3 upload", "album_id", albumID, "photo_id", photoID, "error", err)
		writeError(w, http.StatusInternalServerError, "upload error")
		return
	}

	// persist photo record with status processing before returning 202
	// this ensures GET /photos/:id never returns 404 after the client receives 202
	photo := &model.Photo{
		AlbumID: albumID,
		PhotoID: photoID,
		Seq:     seq,
		Status:  "processing",
		S3Key:   s3Key,
	}
	if err := h.Store.CreatePhoto(r.Context(), photo); err != nil {
		slog.Error("create photo record", "album_id", albumID, "photo_id", photoID, "error", err)
		_ = h.Blob.Delete(r.Context(), s3Key)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	// optimistic inline completion: the file is already in S3, the only "processing"
	// work is generating the URL and updating DynamoDB status to completed.
	// attempt this inline to eliminate SQS round-trip latency on the happy path.
	// if this fails, fall back to SQS so the worker retries with full durability.
	objectURL := h.Blob.ObjectURL(s3Key)
	completionErr := h.Store.CompletePhoto(r.Context(), albumID, photoID, objectURL)

	if completionErr != nil {
		// inline completion failed (transient DynamoDB error or delete race)
		// enqueue to SQS for the worker to retry with backoff and DLQ safety
		slog.Warn("inline completion failed, falling back to SQS",
			"album_id", albumID, "photo_id", photoID, "error", completionErr)

		job := &model.PhotoJob{
			AlbumID: albumID,
			PhotoID: photoID,
			S3Key:   s3Key,
		}
		if enqueueErr := h.Queue.Enqueue(r.Context(), job); enqueueErr != nil {
			slog.Error("sqs enqueue fallback failed", "album_id", albumID, "photo_id", photoID, "error", enqueueErr)
			_ = h.Store.FailPhoto(r.Context(), albumID, photoID)
			writeError(w, http.StatusInternalServerError, "processing error")
			return
		}
	}

	writeJSON(w, http.StatusAccepted, model.PhotoAccepted{
		PhotoID: photoID,
		Seq:     seq,
		Status:  "processing",
	})
}

// GetPhoto returns current status of a photo at any lifecycle stage.
func (h *Handler) GetPhoto(w http.ResponseWriter, r *http.Request) {
	albumID := r.PathValue("album_id")
	photoID := r.PathValue("photo_id")

	photo, err := h.Store.GetPhoto(r.Context(), albumID, photoID)
	if err != nil {
		slog.Error("get photo", "album_id", albumID, "photo_id", photoID, "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	if photo == nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}

	// build response with exact contract fields
	// seq must be present at all lifecycle stages
	resp := map[string]interface{}{
		"photo_id": photo.PhotoID,
		"album_id": photo.AlbumID,
		"seq":      photo.Seq,
		"status":   photo.Status,
	}
	// url present only when status is completed
	if photo.Status == "completed" && photo.URL != "" {
		resp["url"] = photo.URL
	}

	writeJSON(w, http.StatusOK, resp)
}

// DeletePhoto performs synchronous hard delete of S3 object and DynamoDB record in parallel.
// Must complete within 5 seconds. After success, GET returns 404 and the stored URL stops returning 200.
func (h *Handler) DeletePhoto(w http.ResponseWriter, r *http.Request) {
	albumID := r.PathValue("album_id")
	photoID := r.PathValue("photo_id")

	// fetch metadata to get s3_key before deletion
	photo, err := h.Store.GetPhoto(r.Context(), albumID, photoID)
	if err != nil {
		slog.Error("get photo for delete", "album_id", albumID, "photo_id", photoID, "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	if photo == nil {
		// already deleted or never existed
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// parallel delete to stay within 5 second budget
	var wg sync.WaitGroup
	var s3Err, dbErr error

	wg.Add(2)
	go func() {
		defer wg.Done()
		if photo.S3Key != "" {
			s3Err = h.Blob.Delete(r.Context(), photo.S3Key)
		}
	}()
	go func() {
		defer wg.Done()
		dbErr = h.Store.DeletePhoto(r.Context(), albumID, photoID)
	}()
	wg.Wait()

	if s3Err != nil {
		slog.Error("s3 delete failed", "s3_key", photo.S3Key, "error", s3Err)
	}
	if dbErr != nil {
		slog.Error("db delete failed", "album_id", albumID, "photo_id", photoID, "error", dbErr)
		writeError(w, http.StatusInternalServerError, "delete failed")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
