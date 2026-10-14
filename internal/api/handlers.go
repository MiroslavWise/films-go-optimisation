package api

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"films-go-optimisation/internal/auth"
	"films-go-optimisation/internal/blob"
	"films-go-optimisation/internal/config"
	"films-go-optimisation/internal/imageproc"
	"films-go-optimisation/internal/mime"
	"films-go-optimisation/internal/webhook"

	"github.com/google/uuid"
)

type Server struct {
	Cfg  config.Config
	Auth *auth.Verifier
	Blob *blob.Client
	Hook *webhook.Client
}

func NewServer(cfg config.Config) *Server {
	return &Server{
		Cfg:  cfg,
		Auth: auth.NewVerifier(cfg.ProcessorToken, cfg.JobJWTSecret, cfg.JobJWTIssuer, cfg.JobJWTAudience),
		Blob: blob.New(cfg.BlobToken),
		Hook: webhook.New(cfg.WebhookSecret),
	}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	mux.HandleFunc("POST /v1/process", s.handleProcess)
	mux.HandleFunc("POST /v1/process-batch", s.handleProcessBatch)
	return mux
}

func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleProcess(w http.ResponseWriter, r *http.Request) {
	authRes, err := s.Auth.Authenticate(r.Header.Get("Authorization"))
	if err != nil {
		writeErr(w, http.StatusUnauthorized, "unauthorized", err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, s.Cfg.MaxUploadBytes+1<<20)
	if err := r.ParseMultipartForm(s.Cfg.MaxUploadBytes + 1<<20); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_multipart", "invalid multipart form or file too large")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeErr(w, http.StatusBadRequest, "file_required", "file is required")
		return
	}
	defer file.Close()

	payload, err := io.ReadAll(io.LimitReader(file, s.Cfg.MaxUploadBytes+1))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "read_failed", "cannot read uploaded file")
		return
	}
	if int64(len(payload)) > s.Cfg.MaxUploadBytes {
		writeErr(w, http.StatusRequestEntityTooLarge, "too_large", "file exceeds MAX_UPLOAD_BYTES")
		return
	}
	if len(payload) == 0 {
		writeErr(w, http.StatusBadRequest, "empty_file", "empty file")
		return
	}

	kindStr := strings.TrimSpace(r.FormValue("kind"))
	entityID := strings.TrimSpace(r.FormValue("entityId"))
	movieID := strings.TrimSpace(r.FormValue("movieId"))
	jobID := strings.TrimSpace(r.FormValue("jobId"))
	webhookURL := strings.TrimSpace(r.FormValue("webhookUrl"))

	if authRes.Mode == auth.ModeJob && authRes.Claims != nil {
		c := authRes.Claims
		if c.Kind != "" {
			kindStr = c.Kind
		}
		if c.EntityID != "" {
			entityID = c.EntityID
		}
		if c.MovieID != "" {
			movieID = c.MovieID
		}
		if c.JobID != "" {
			jobID = c.JobID
		}
	}

	kind, err := imageproc.ParseKind(kindStr)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "bad_kind", err.Error())
		return
	}
	if kind == imageproc.KindFrame && !isUUID(movieID) {
		writeErr(w, http.StatusBadRequest, "movie_id_required", "movieId UUID is required for kind=frame")
		return
	}
	if entityID == "" || !isUUID(entityID) {
		id, genErr := uuid.NewV7()
		if genErr != nil {
			id = uuid.New()
		}
		entityID = id.String()
	}
	if webhookURL == "" {
		webhookURL = s.Cfg.DefaultWebhookURL
	}
	async := webhookURL != "" && jobID != ""

	fileName := ""
	declared := ""
	if header != nil {
		fileName = header.Filename
		declared = header.Header.Get("Content-Type")
	}

	detected := mime.DetectImageMIME(payload, declared, fileName)
	if detected == "" {
		writeErr(w, http.StatusBadRequest, "unsupported_image_type", "unsupported image type")
		return
	}

	resp, status := s.processOne(r.Context(), processInput{
		Payload:    payload,
		Kind:       kind,
		EntityID:   entityID,
		MovieID:    movieID,
		JobID:      jobID,
		WebhookURL: webhookURL,
		Async:      async,
	})
	writeJSON(w, status, resp)
}

func (s *Server) handleProcessBatch(w http.ResponseWriter, r *http.Request) {
	authRes, err := s.Auth.Authenticate(r.Header.Get("Authorization"))
	if err != nil {
		writeErr(w, http.StatusUnauthorized, "unauthorized", err.Error())
		return
	}
	if authRes.Mode == auth.ModeJob {
		writeErr(w, http.StatusForbidden, "job_token_not_for_batch", "use service token for batch, or call /v1/process per file with job JWT")
		return
	}

	maxForm := s.Cfg.MaxUploadBytes*8 + 2<<20
	if maxForm > 32<<20 {
		maxForm = 32 << 20
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxForm)
	if err := r.ParseMultipartForm(maxForm); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_multipart", "invalid multipart form or body too large")
		return
	}

	kindStr := strings.TrimSpace(r.FormValue("kind"))
	movieID := strings.TrimSpace(r.FormValue("movieId"))
	webhookURL := strings.TrimSpace(r.FormValue("webhookUrl"))
	if webhookURL == "" {
		webhookURL = s.Cfg.DefaultWebhookURL
	}

	kind, err := imageproc.ParseKind(kindStr)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "bad_kind", err.Error())
		return
	}
	if kind == imageproc.KindFrame && !isUUID(movieID) {
		writeErr(w, http.StatusBadRequest, "movie_id_required", "movieId UUID is required for kind=frame")
		return
	}

	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		files = r.MultipartForm.File["file"]
	}
	if len(files) == 0 {
		writeErr(w, http.StatusBadRequest, "files_required", "files (or file) multipart fields required")
		return
	}
	if len(files) > 20 {
		writeErr(w, http.StatusBadRequest, "too_many_files", "max 20 files per batch")
		return
	}

	inputs := make([]processInput, 0, len(files))
	for _, fh := range files {
		f, err := fh.Open()
		if err != nil {
			writeErr(w, http.StatusBadRequest, "read_failed", "cannot open uploaded file")
			return
		}
		payload, err := io.ReadAll(io.LimitReader(f, s.Cfg.MaxUploadBytes+1))
		_ = f.Close()
		if err != nil {
			writeErr(w, http.StatusBadRequest, "read_failed", "cannot read uploaded file")
			return
		}
		if int64(len(payload)) > s.Cfg.MaxUploadBytes {
			writeErr(w, http.StatusRequestEntityTooLarge, "too_large", "one of the files exceeds MAX_UPLOAD_BYTES")
			return
		}
		if len(payload) == 0 {
			continue
		}
		detected := mime.DetectImageMIME(payload, fh.Header.Get("Content-Type"), fh.Filename)
		if detected == "" {
			writeErr(w, http.StatusBadRequest, "unsupported_image_type", "unsupported image type: "+fh.Filename)
			return
		}

		entityID, genErr := uuid.NewV7()
		if genErr != nil {
			entityID = uuid.New()
		}
		jobID, genErr := uuid.NewV7()
		if genErr != nil {
			jobID = uuid.New()
		}

		inputs = append(inputs, processInput{
			Payload:    payload,
			Kind:       kind,
			EntityID:   entityID.String(),
			MovieID:    movieID,
			JobID:      jobID.String(),
			WebhookURL: webhookURL,
			Async:      webhookURL != "",
		})
	}

	out := make([]ProcessResponse, len(inputs))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 4)
	for i := range inputs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			resp, _ := s.processOne(r.Context(), inputs[i])
			out[i] = resp
		}(i)
	}
	wg.Wait()

	allOK := true
	for _, j := range out {
		if !j.OK {
			allOK = false
			break
		}
	}
	writeJSON(w, http.StatusOK, BatchResponse{OK: allOK, Jobs: out})
}

type processInput struct {
	Payload    []byte
	Kind       imageproc.Kind
	EntityID   string
	MovieID    string
	JobID      string
	WebhookURL string
	Async      bool
}

func (s *Server) processOne(ctx context.Context, in processInput) (ProcessResponse, int) {
	processed, err := imageproc.ProcessImage(in.Payload, in.Kind, imageproc.Options{
		Quality: s.Cfg.AVIFQuality,
		Speed:   s.Cfg.AVIFSpeed,
	})
	if err != nil {
		resp := failResponse(in.JobID, in.EntityID, in.Kind, in.MovieID, "encode_failed", err.Error())
		s.maybeWebhook(ctx, in, resp)
		return resp, http.StatusUnprocessableEntity
	}

	pathname, err := imageproc.BlobPath(in.Kind, in.EntityID, in.MovieID)
	if err != nil {
		resp := failResponse(in.JobID, in.EntityID, in.Kind, in.MovieID, "bad_path", err.Error())
		s.maybeWebhook(ctx, in, resp)
		return resp, http.StatusBadRequest
	}

	putCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	put, err := s.Blob.Put(putCtx, pathname, processed.AVIF, "image/avif")
	if err != nil {
		resp := failResponse(in.JobID, in.EntityID, in.Kind, in.MovieID, "blob_failed", err.Error())
		s.maybeWebhook(ctx, in, resp)
		return resp, http.StatusInternalServerError
	}

	resp := successResponse(
		in.JobID,
		in.EntityID,
		in.Kind,
		in.MovieID,
		put.URL,
		processed.Blurhash,
		processed.Width,
		processed.Height,
		len(processed.AVIF),
	)
	s.maybeWebhook(ctx, in, resp)
	return resp, http.StatusOK
}

func (s *Server) maybeWebhook(ctx context.Context, in processInput, resp ProcessResponse) {
	if !in.Async || in.WebhookURL == "" || in.JobID == "" {
		return
	}
	raw, err := json.Marshal(resp)
	if err != nil {
		log.Printf("webhook marshal: %v", err)
		return
	}
	hookCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 45*time.Second)
	defer cancel()
	if err := s.Hook.Notify(hookCtx, in.WebhookURL, in.JobID, raw); err != nil {
		log.Printf("webhook job=%s: %v", in.JobID, err)
	}
}

func isUUID(s string) bool {
	_, err := uuid.Parse(s)
	return err == nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, ProcessResponse{OK: false, Error: code, Message: message})
}
