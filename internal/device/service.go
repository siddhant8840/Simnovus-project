package device

import (
	"errors"
	"sort"
	"strings"
	"time"
)

var (
	// ErrInvalidInput is returned when required fields are missing or empty.
	ErrInvalidInput = errors.New("id and name are required")
)

// Service encapsulates the business operations for device fleet monitoring.
type Service struct {
	repo    Repository
	timeout time.Duration
	nowFunc func() time.Time
}

// NewService instantiates a new Service with standard or custom configuration.
func NewService(repo Repository, timeout time.Duration) *Service {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &Service{
		repo:    repo,
		timeout: timeout,
		nowFunc: time.Now,
	}
}

// SetNowFunc overrides the time source (useful for deterministic tests).
func (s *Service) SetNowFunc(fn func() time.Time) {
	s.nowFunc = fn
}

// RegisterDevice validates and creates a new device record.
func (s *Service) RegisterDevice(req RegisterRequest) (DeviceResponse, error) {
	trimmedID := strings.TrimSpace(req.ID)
	trimmedName := strings.TrimSpace(req.Name)

	if trimmedID == "" || trimmedName == "" {
		return DeviceResponse{}, ErrInvalidInput
	}

	dev := Device{
		ID:            trimmedID,
		Name:          trimmedName,
		LastHeartbeat: time.Time{}, // zero value indicates no heartbeat received yet
	}

	if err := s.repo.Register(dev); err != nil {
		return DeviceResponse{}, err
	}

	return dev.ToResponse(s.nowFunc(), s.timeout), nil
}

// RecordHeartbeat records receipt of a device heartbeat at the current server time.
// Note: While client payload may contain a timestamp, the server's receipt time
// acts as the authoritative source of truth for freshness and drift safety.
func (s *Service) RecordHeartbeat(id string, req HeartbeatRequest) (DeviceResponse, error) {
	trimmedID := strings.TrimSpace(id)
	if trimmedID == "" {
		return DeviceResponse{}, ErrInvalidInput
	}

	receivedAt := s.nowFunc()
	dev, err := s.repo.RecordHeartbeat(trimmedID, receivedAt)
	if err != nil {
		return DeviceResponse{}, err
	}

	return dev.ToResponse(receivedAt, s.timeout), nil
}

// GetDevice retrieves the current details and derived status for a device.
func (s *Service) GetDevice(id string) (DeviceResponse, error) {
	trimmedID := strings.TrimSpace(id)
	if trimmedID == "" {
		return DeviceResponse{}, ErrInvalidInput
	}

	dev, err := s.repo.GetByID(trimmedID)
	if err != nil {
		return DeviceResponse{}, err
	}

	return dev.ToResponse(s.nowFunc(), s.timeout), nil
}

// ListDevices returns all registered devices with their current derived status,
// sorted by device ID for deterministic ordering.
func (s *Service) ListDevices() []DeviceResponse {
	devices := s.repo.GetAll()
	now := s.nowFunc()

	// Sort deterministically by ID
	sort.Slice(devices, func(i, j int) bool {
		return devices[i].ID < devices[j].ID
	})

	responses := make([]DeviceResponse, len(devices))
	for i, dev := range devices {
		responses[i] = dev.ToResponse(now, s.timeout)
	}

	return responses
}

// GetSummary calculates the fleet-wide summary (total, online, offline).
func (s *Service) GetSummary() FleetSummary {
	devices := s.repo.GetAll()
	now := s.nowFunc()

	var summary FleetSummary
	summary.Total = len(devices)

	for _, dev := range devices {
		if CalculateStatus(dev.LastHeartbeat, now, s.timeout) == StatusOnline {
			summary.Online++
		} else {
			summary.Offline++
		}
	}

	return summary
}
