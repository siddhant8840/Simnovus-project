package device

import (
	"errors"
	"sync"
	"time"
)

var (
	// ErrDeviceNotFound is returned when a requested device does not exist.
	ErrDeviceNotFound = errors.New("device not found")

	// ErrDeviceAlreadyExists is returned when attempting to register a device with an existing ID.
	ErrDeviceAlreadyExists = errors.New("device already exists")
)

// Repository defines storage operations for device fleet management.
type Repository interface {
	Register(dev Device) error
	GetByID(id string) (Device, error)
	GetAll() []Device
	RecordHeartbeat(id string, timestamp time.Time) (Device, error)
}

// MemoryRepository is a thread-safe in-memory implementation of Repository.
type MemoryRepository struct {
	mu      sync.RWMutex
	devices map[string]Device
}

// NewMemoryRepository initializes an empty in-memory repository.
func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		devices: make(map[string]Device),
	}
}

// Register adds a new device to the repository.
func (r *MemoryRepository) Register(dev Device) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.devices[dev.ID]; exists {
		return ErrDeviceAlreadyExists
	}
	r.devices[dev.ID] = dev
	return nil
}

// GetByID looks up a device by ID.
func (r *MemoryRepository) GetByID(id string) (Device, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	dev, exists := r.devices[id]
	if !exists {
		return Device{}, ErrDeviceNotFound
	}
	return dev, nil
}

// GetAll returns a snapshot slice of all registered devices.
func (r *MemoryRepository) GetAll() []Device {
	r.mu.RLock()
	defer r.mu.RUnlock()

	list := make([]Device, 0, len(r.devices))
	for _, dev := range r.devices {
		list = append(list, dev)
	}
	return list
}

// RecordHeartbeat updates the last heartbeat timestamp for a registered device.
func (r *MemoryRepository) RecordHeartbeat(id string, timestamp time.Time) (Device, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	dev, exists := r.devices[id]
	if !exists {
		return Device{}, ErrDeviceNotFound
	}
	dev.LastHeartbeat = timestamp
	r.devices[id] = dev
	return dev, nil
}
