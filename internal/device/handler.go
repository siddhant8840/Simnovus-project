package device

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"mini-device-fleet/internal/api"
)

// Handler exposes HTTP handler endpoints for fleet management.
type Handler struct {
	service *Service
}

// NewHandler creates a new Handler instance.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes registers the REST endpoints with standard library http.ServeMux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /devices", h.Register)
	mux.HandleFunc("GET /devices", h.List)
	mux.HandleFunc("GET /devices/{id}", h.GetByID)
	mux.HandleFunc("POST /devices/{id}/heartbeat", h.Heartbeat)
	mux.HandleFunc("GET /summary", h.Summary)
}

// extractID extracts the device ID from path parameter with fallback.
func (h *Handler) extractID(r *http.Request) string {
	if val := r.PathValue("id"); val != "" {
		return val
	}

	// Fallback path parsing for backward compatibility
	path := strings.TrimPrefix(r.URL.Path, "/devices/")
	parts := strings.Split(path, "/")
	if len(parts) > 0 {
		return parts[0]
	}
	return ""
}

// Register handles POST /devices.
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&req); err != nil {
		slog.Warn("failed to decode register request", "error", err.Error())
		api.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	dev, err := h.service.RegisterDevice(req)
	if err != nil {
		if errors.Is(err, ErrInvalidInput) {
			slog.Warn("invalid device registration data", "id", req.ID, "name", req.Name)
			api.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		if errors.Is(err, ErrDeviceAlreadyExists) {
			slog.Warn("attempted duplicate device registration", "id", req.ID)
			api.WriteError(w, http.StatusConflict, "device already exists")
			return
		}
		slog.Error("internal error during registration", "error", err.Error())
		api.WriteError(w, http.StatusInternalServerError, "failed to register device")
		return
	}

	slog.Info("device registered successfully", "id", dev.ID, "name", dev.Name)
	api.WriteJSON(w, http.StatusCreated, dev)
}

// Heartbeat handles POST /devices/{id}/heartbeat.
func (h *Handler) Heartbeat(w http.ResponseWriter, r *http.Request) {
	id := h.extractID(r)
	if id == "" {
		api.WriteError(w, http.StatusBadRequest, "device id required in URL path")
		return
	}

	var req HeartbeatRequest
	// Support optional JSON payload (timestamp, status, cpu_usage, signal_strength)
	if r.Body != nil && r.ContentLength != 0 {
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&req); err != nil && !errors.Is(err, io.EOF) {
			slog.Warn("invalid heartbeat payload", "id", id, "error", err.Error())
			api.WriteError(w, http.StatusBadRequest, "invalid heartbeat JSON body")
			return
		}
	}

	dev, err := h.service.RecordHeartbeat(id, req)
	if err != nil {
		if errors.Is(err, ErrDeviceNotFound) {
			slog.Warn("heartbeat received for unknown device", "id", id)
			api.WriteError(w, http.StatusNotFound, "device not found")
			return
		}
		slog.Error("error recording heartbeat", "id", id, "error", err.Error())
		api.WriteError(w, http.StatusInternalServerError, "failed to record heartbeat")
		return
	}

	slog.Info("heartbeat received", "id", dev.ID, "status", dev.Status)
	api.WriteJSON(w, http.StatusOK, dev)
}

// List handles GET /devices.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	devices := h.service.ListDevices()
	api.WriteJSON(w, http.StatusOK, devices)
}

// GetByID handles GET /devices/{id}.
func (h *Handler) GetByID(w http.ResponseWriter, r *http.Request) {
	id := h.extractID(r)
	if id == "" {
		api.WriteError(w, http.StatusBadRequest, "device id required in URL path")
		return
	}

	dev, err := h.service.GetDevice(id)
	if err != nil {
		if errors.Is(err, ErrDeviceNotFound) {
			api.WriteError(w, http.StatusNotFound, "device not found")
			return
		}
		api.WriteError(w, http.StatusInternalServerError, "failed to retrieve device")
		return
	}

	api.WriteJSON(w, http.StatusOK, dev)
}

// Summary handles GET /summary.
func (h *Handler) Summary(w http.ResponseWriter, r *http.Request) {
	summary := h.service.GetSummary()
	api.WriteJSON(w, http.StatusOK, summary)
}
