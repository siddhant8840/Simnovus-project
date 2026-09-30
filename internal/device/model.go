package device

import "time"

// Status represents the operational status of a device.
type Status string

const (
	StatusOnline  Status = "ONLINE"
	StatusOffline Status = "OFFLINE"
)

// Device represents the core entity in the fleet.
type Device struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	LastHeartbeat time.Time `json:"last_heartbeat"`
}

// DeviceResponse is the serialized representation returned by APIs.
type DeviceResponse struct {
	ID            string     `json:"id"`
	Name          string     `json:"name"`
	Status        Status     `json:"status"`
	LastHeartbeat *time.Time `json:"last_heartbeat"`
}

// RegisterRequest holds the payload required to register a device.
type RegisterRequest struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// HeartbeatRequest represents the incoming heartbeat data from a device.
type HeartbeatRequest struct {
	Timestamp      *time.Time `json:"timestamp,omitempty"`
	Status         string     `json:"status,omitempty"`
	CPUUsage       *float64   `json:"cpu_usage,omitempty"`
	SignalStrength *int       `json:"signal_strength,omitempty"`
}

// FleetSummary summarizes fleet-wide health status.
type FleetSummary struct {
	Total   int `json:"total"`
	Online  int `json:"online"`
	Offline int `json:"offline"`
}

// CalculateStatus evaluates whether a device is ONLINE or OFFLINE
// based on the elapsed time since its last heartbeat.
func CalculateStatus(lastHeartbeat time.Time, now time.Time, timeout time.Duration) Status {
	if lastHeartbeat.IsZero() {
		return StatusOffline
	}
	// A device is ONLINE if the elapsed duration is within the timeout window.
	// Since clocks or concurrent calls can have slight delta, we check if duration <= timeout
	// and lastHeartbeat is not in the future beyond normal jitter.
	diff := now.Sub(lastHeartbeat)
	if diff <= timeout && diff >= -1*time.Second {
		return StatusOnline
	}
	return StatusOffline
}

// ToResponse converts an internal Device model to DeviceResponse with computed status.
func (d Device) ToResponse(now time.Time, timeout time.Duration) DeviceResponse {
	resp := DeviceResponse{
		ID:     d.ID,
		Name:   d.Name,
		Status: CalculateStatus(d.LastHeartbeat, now, timeout),
	}
	if !d.LastHeartbeat.IsZero() {
		hbCopy := d.LastHeartbeat
		resp.LastHeartbeat = &hbCopy
	}
	return resp
}
