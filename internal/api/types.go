package api

import "films-go-optimisation/internal/imageproc"

type ProcessResponse struct {
	OK          bool    `json:"ok"`
	JobID       string  `json:"jobId,omitempty"`
	EntityID    string  `json:"entityId,omitempty"`
	Kind        string  `json:"kind,omitempty"`
	MovieID     *string `json:"movieId"`
	URL         string  `json:"url,omitempty"`
	Blurhash    string  `json:"blurhash,omitempty"`
	Width       int     `json:"width,omitempty"`
	Height      int     `json:"height,omitempty"`
	ContentType string  `json:"contentType,omitempty"`
	Bytes       int     `json:"bytes,omitempty"`
	Error       string  `json:"error,omitempty"`
	Message     string  `json:"message,omitempty"`
}

type BatchResponse struct {
	OK   bool              `json:"ok"`
	Jobs []ProcessResponse `json:"jobs"`
}

func movieIDPtr(id string) *string {
	if id == "" {
		return nil
	}
	return &id
}

func successResponse(jobID, entityID string, kind imageproc.Kind, movieID, url, blurhash string, w, h, nbytes int) ProcessResponse {
	return ProcessResponse{
		OK:          true,
		JobID:       jobID,
		EntityID:    entityID,
		Kind:        string(kind),
		MovieID:     movieIDPtr(movieID),
		URL:         url,
		Blurhash:    blurhash,
		Width:       w,
		Height:      h,
		ContentType: "image/avif",
		Bytes:       nbytes,
	}
}

func failResponse(jobID, entityID string, kind imageproc.Kind, movieID, code, message string) ProcessResponse {
	return ProcessResponse{
		OK:      false,
		JobID:   jobID,
		EntityID: entityID,
		Kind:    string(kind),
		MovieID: movieIDPtr(movieID),
		Error:   code,
		Message: message,
	}
}
