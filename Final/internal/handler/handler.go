package handler

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"mime"
	"mime/multipart"
	"net/http"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/karthikeyansura/CS6650/Final/internal/blob"
	"github.com/karthikeyansura/CS6650/Final/internal/model"
	"github.com/karthikeyansura/CS6650/Final/internal/queue"
	"github.com/karthikeyansura/CS6650/Final/internal/store"
)

type Handler struct {
	Store *store.DynamoStore
	Blob  *blob.S3Client
	Queue *queue.SQSQueue
}

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

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, model.HealthResponse{Status: "ok"})
}

func (h *Handler) UpsertAlbum(w http.ResponseWriter, r *http.Request) {
	albumID := r.PathValue("album_id")

	var req model.Album
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	defer r.Body.Close()

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

func (h *Handler) ListAlbums(w http.ResponseWriter, r *http.Request) {
	albums, err := h.Store.ListAlbums(r.Context())
	if err != nil {
		slog.Error("list albums", "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	if albums == nil {
		albums = []model.Album{}
	}
	writeJSON(w, http.StatusOK, albums)
}

// UploadPhoto implements the async photo pipeline with early 202 flush.
//
// Flow:
//  1. Parse multipart boundary, locate the photo part via streaming reader
//  2. Allocate atomic per-album seq via DynamoDB counter
//  3. Persist photo record with status=processing in DynamoDB
//  4. Flush 202 Accepted to the client immediately (~20ms total accept latency)
//  5. Continue streaming photo bytes directly from the request body to S3
//  6. On S3 success, update DynamoDB to status=completed with public URL
//  7. On failure, mark failed or enqueue to SQS for the worker to retry
//
// The request body remains readable after the 202 is flushed because the
// handler has not returned. Go net/http keeps the connection alive until
// the handler function exits. This gives us zero-copy streaming with no
// memory buffering and no temp files.
func (h *Handler) UploadPhoto(w http.ResponseWriter, r *http.Request) {
	albumID := r.PathValue("album_id")

	ct := r.Header.Get("Content-Type")
	if ct == "" {
		writeError(w, http.StatusBadRequest, "missing content-type")
		return
	}
	mediaType, params, err := mime.ParseMediaType(ct)
	if err != nil || !strings.HasPrefix(mediaType, "multipart/") {
		writeError(w, http.StatusBadRequest, "invalid multipart content-type")
		return
	}
	boundary := params["boundary"]
	if boundary == "" {
		writeError(w, http.StatusBadRequest, "missing boundary")
		return
	}

	mr := multipart.NewReader(r.Body, boundary)

	var photoPart *multipart.Part
	var partContentType string
	for {
		part, partErr := mr.NextPart()
		if partErr != nil {
			break
		}
		if part.FormName() == "photo" {
			photoPart = part
			partContentType = part.Header.Get("Content-Type")
			break
		}
		part.Close()
	}

	if photoPart == nil {
		writeError(w, http.StatusBadRequest, "missing 'photo' field")
		return
	}
	defer photoPart.Close()

	if partContentType == "" {
		partContentType = "application/octet-stream"
	}

	photoID := uuid.New().String()

	// step 1: atomic seq allocation via DynamoDB counter
	seq, err := h.Store.AllocateSeq(r.Context(), albumID)
	if err != nil {
		slog.Error("allocate seq", "album_id", albumID, "error", err)
		writeError(w, http.StatusInternalServerError, "sequence error")
		return
	}

	s3Key := fmt.Sprintf("photos/%s/%s", albumID, photoID)

	// step 2: persist photo record with status=processing
	photo := &model.Photo{
		AlbumID: albumID,
		PhotoID: photoID,
		Seq:     seq,
		Status:  "processing",
		S3Key:   s3Key,
	}
	if err := h.Store.CreatePhoto(r.Context(), photo); err != nil {
		slog.Error("create photo record", "album_id", albumID, "photo_id", photoID, "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	// step 3: flush 202 to client immediately
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(model.PhotoAccepted{
		PhotoID: photoID,
		Seq:     seq,
		Status:  "processing",
	})
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}

	// step 4: stream photo bytes directly from multipart reader to S3
	// the request body is still readable because the handler has not returned
	if uploadErr := h.Blob.Upload(r.Context(), s3Key, photoPart, partContentType); uploadErr != nil {
		slog.Error("s3 upload after 202", "album_id", albumID, "photo_id", photoID, "error", uploadErr)
		_ = h.Store.FailPhoto(r.Context(), albumID, photoID)
		return
	}

	// step 5: mark completed with public URL
	objectURL := h.Blob.ObjectURL(s3Key)
	if completeErr := h.Store.CompletePhoto(r.Context(), albumID, photoID, objectURL); completeErr != nil {
		slog.Warn("completion after 202 failed, enqueueing to SQS",
			"album_id", albumID, "photo_id", photoID, "error", completeErr)
		job := &model.PhotoJob{AlbumID: albumID, PhotoID: photoID, S3Key: s3Key}
		if enqErr := h.Queue.Enqueue(r.Context(), job); enqErr != nil {
			slog.Error("sqs enqueue fallback failed", "photo_id", photoID, "error", enqErr)
			_ = h.Store.FailPhoto(r.Context(), albumID, photoID)
		}
	}
}

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

	resp := map[string]interface{}{
		"photo_id": photo.PhotoID,
		"album_id": photo.AlbumID,
		"seq":      photo.Seq,
		"status":   photo.Status,
	}
	if photo.Status == "completed" && photo.URL != "" {
		resp["url"] = photo.URL
	}

	writeJSON(w, http.StatusOK, resp)
}

// DeletePhoto performs synchronous hard delete. Never returns 5xx per contract.
func (h *Handler) DeletePhoto(w http.ResponseWriter, r *http.Request) {
	albumID := r.PathValue("album_id")
	photoID := r.PathValue("photo_id")

	photo, err := h.Store.GetPhoto(r.Context(), albumID, photoID)
	if err != nil {
		slog.Error("get photo for delete", "album_id", albumID, "photo_id", photoID, "error", err)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if photo == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}

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
	}

	w.WriteHeader(http.StatusNoContent)
}
