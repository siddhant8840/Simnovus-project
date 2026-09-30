package device_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"mini-device-fleet/internal/device"
)

func TestStatusCalculationBoundaries(t *testing.T) {
	baseTime := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	timeout := 30 * time.Second

	tests := []struct {
		name          string
		lastHeartbeat time.Time
		now           time.Time
		expected      device.Status
	}{
		{
			name:          "Zero heartbeat is OFFLINE",
			lastHeartbeat: time.Time{},
			now:           baseTime,
			expected:      device.StatusOffline,
		},
		{
			name:          "Just now (0s) is ONLINE",
			lastHeartbeat: baseTime,
			now:           baseTime,
			expected:      device.StatusOnline,
		},
		{
			name:          "29 seconds elapsed is ONLINE",
			lastHeartbeat: baseTime,
			now:           baseTime.Add(29 * time.Second),
			expected:      device.StatusOnline,
		},
		{
			name:          "Exact 30 seconds boundary is ONLINE",
			lastHeartbeat: baseTime,
			now:           baseTime.Add(30 * time.Second),
			expected:      device.StatusOnline,
		},
		{
			name:          "31 seconds elapsed is OFFLINE",
			lastHeartbeat: baseTime,
			now:           baseTime.Add(31 * time.Second),
			expected:      device.StatusOffline,
		},
		{
			name:          "60 seconds elapsed is OFFLINE",
			lastHeartbeat: baseTime,
			now:           baseTime.Add(60 * time.Second),
			expected:      device.StatusOffline,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := device.CalculateStatus(tc.lastHeartbeat, tc.now, timeout)
			if got != tc.expected {
				t.Errorf("CalculateStatus() = %v, expected %v", got, tc.expected)
			}
		})
	}
}

func TestDeviceService_RegistrationAndValidation(t *testing.T) {
	repo := device.NewMemoryRepository()
	service := device.NewService(repo, 30*time.Second)

	// Valid registration
	dev, err := service.RegisterDevice(device.RegisterRequest{
		ID:   "device-01",
		Name: "Lab Device 01",
	})
	if err != nil {
		t.Fatalf("unexpected error registering device: %v", err)
	}
	if dev.ID != "device-01" || dev.Name != "Lab Device 01" {
		t.Errorf("mismatched device fields: %+v", dev)
	}
	if dev.Status != device.StatusOffline {
		t.Errorf("newly registered device without heartbeat should be OFFLINE, got %s", dev.Status)
	}

	// Duplicate registration
	_, err = service.RegisterDevice(device.RegisterRequest{
		ID:   "device-01",
		Name: "Duplicate",
	})
	if err != device.ErrDeviceAlreadyExists {
		t.Errorf("expected ErrDeviceAlreadyExists, got %v", err)
	}

	// Empty ID validation
	_, err = service.RegisterDevice(device.RegisterRequest{
		ID:   "  ",
		Name: "Valid Name",
	})
	if err != device.ErrInvalidInput {
		t.Errorf("expected ErrInvalidInput for blank ID, got %v", err)
	}

	// Empty Name validation
	_, err = service.RegisterDevice(device.RegisterRequest{
		ID:   "device-02",
		Name: "",
	})
	if err != device.ErrInvalidInput {
		t.Errorf("expected ErrInvalidInput for blank Name, got %v", err)
	}
}

func TestDeviceService_HeartbeatAndSummary(t *testing.T) {
	repo := device.NewMemoryRepository()
	service := device.NewService(repo, 30*time.Second)

	simTime := time.Date(2026, 9, 30, 10, 0, 0, 0, time.UTC)
	service.SetNowFunc(func() time.Time { return simTime })

	// Register 2 devices
	_, err := service.RegisterDevice(device.RegisterRequest{ID: "device-01", Name: "Alpha"})
	if err != nil {
		t.Fatalf("failed registering dev 1: %v", err)
	}
	_, err = service.RegisterDevice(device.RegisterRequest{ID: "device-02", Name: "Beta"})
	if err != nil {
		t.Fatalf("failed registering dev 2: %v", err)
	}

	// Before any heartbeats: Total=2, Online=0, Offline=2
	summary := service.GetSummary()
	if summary.Total != 2 || summary.Online != 0 || summary.Offline != 2 {
		t.Fatalf("expected 2 total, 0 online, 2 offline, got %+v", summary)
	}

	// Heartbeat on device-01 at T=10:00:00
	hbResp, err := service.RecordHeartbeat("device-01", device.HeartbeatRequest{})
	if err != nil {
		t.Fatalf("unexpected error on heartbeat: %v", err)
	}
	if hbResp.Status != device.StatusOnline {
		t.Errorf("expected device-01 to be ONLINE after heartbeat, got %s", hbResp.Status)
	}

	// Heartbeat on nonexistent device
	_, err = service.RecordHeartbeat("unknown-device", device.HeartbeatRequest{})
	if err != device.ErrDeviceNotFound {
		t.Errorf("expected ErrDeviceNotFound, got %v", err)
	}

	// Now Summary at T=10:00:00: Total=2, Online=1, Offline=1
	summary = service.GetSummary()
	if summary.Online != 1 || summary.Offline != 1 {
		t.Fatalf("expected 1 online, 1 offline, got %+v", summary)
	}

	// Advance time by 25 seconds (T=10:00:25) -> device-01 still ONLINE
	simTime = simTime.Add(25 * time.Second)
	dev1, _ := service.GetDevice("device-01")
	if dev1.Status != device.StatusOnline {
		t.Errorf("device-01 should be ONLINE at +25s, got %s", dev1.Status)
	}

	// Advance time past 30s timeout (T=10:00:35) -> device-01 transitions to OFFLINE
	simTime = simTime.Add(10 * time.Second)
	dev1, _ = service.GetDevice("device-01")
	if dev1.Status != device.StatusOffline {
		t.Errorf("device-01 should be OFFLINE at +35s, got %s", dev1.Status)
	}

	summary = service.GetSummary()
	if summary.Online != 0 || summary.Offline != 2 {
		t.Fatalf("expected 0 online, 2 offline at +35s, got %+v", summary)
	}
}

func TestDeviceRepository_ConcurrentAccess(t *testing.T) {
	repo := device.NewMemoryRepository()
	numDevices := 20
	numHeartbeatsPerDevice := 50

	// Register devices
	for i := 0; i < numDevices; i++ {
		id := fmt.Sprintf("dev-%d", i)
		if err := repo.Register(device.Device{ID: id, Name: "Concurrent Test"}); err != nil {
			t.Fatalf("failed registering: %v", err)
		}
	}

	var wg sync.WaitGroup
	// Concurrently send heartbeats and read devices
	for i := 0; i < numDevices; i++ {
		id := fmt.Sprintf("dev-%d", i)
		wg.Add(2)

		// Goroutine 1: write heartbeats
		go func(deviceID string) {
			defer wg.Done()
			for h := 0; h < numHeartbeatsPerDevice; h++ {
				_, _ = repo.RecordHeartbeat(deviceID, time.Now())
			}
		}(id)

		// Goroutine 2: read devices
		go func(deviceID string) {
			defer wg.Done()
			for h := 0; h < numHeartbeatsPerDevice; h++ {
				_, _ = repo.GetByID(deviceID)
				_ = repo.GetAll()
			}
		}(id)
	}

	wg.Wait()

	all := repo.GetAll()
	if len(all) != numDevices {
		t.Errorf("expected %d devices after concurrent ops, got %d", numDevices, len(all))
	}
}
