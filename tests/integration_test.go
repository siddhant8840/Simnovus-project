package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"mini-device-fleet/internal/device"
)

func setupTestServer(timeout time.Duration) (*device.Service, *http.ServeMux) {
	repo := device.NewMemoryRepository()
	service := device.NewService(repo, timeout)
	handler := device.NewHandler(service)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	return service, mux
}

func TestAPIRegistration(t *testing.T) {
	_, mux := setupTestServer(30 * time.Second)

	// 1. Success registration
	reqBody := `{"id": "device-01", "name": "Lab Device 01"}`
	req := httptest.NewRequest(http.MethodPost, "/devices", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", w.Code, w.Body.String())
	}

	var resp device.DeviceResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.ID != "device-01" || resp.Name != "Lab Device 01" || resp.Status != device.StatusOffline {
		t.Errorf("unexpected registered device fields: %+v", resp)
	}

	// 2. Duplicate registration -> 409 Conflict
	reqDup := httptest.NewRequest(http.MethodPost, "/devices", bytes.NewBufferString(reqBody))
	reqDup.Header.Set("Content-Type", "application/json")
	wDup := httptest.NewRecorder()
	mux.ServeHTTP(wDup, reqDup)

	if wDup.Code != http.StatusConflict {
		t.Fatalf("expected 409 Conflict for duplicate ID, got %d", wDup.Code)
	}

	// 3. Invalid inputs -> 400 Bad Request
	invalidBodies := []string{
		`{"id": "", "name": "Test"}`,
		`{"id": "device-02", "name": "  "}`,
		`{not valid json}`,
	}
	for _, b := range invalidBodies {
		r := httptest.NewRequest(http.MethodPost, "/devices", bytes.NewBufferString(b))
		r.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, r)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request for %q, got %d", b, rec.Code)
		}
	}
}

func TestAPIHeartbeatAndDetails(t *testing.T) {
	_, mux := setupTestServer(30 * time.Second)

	// Register device
	reg := httptest.NewRequest(http.MethodPost, "/devices", bytes.NewBufferString(`{"id": "device-01", "name": "Sensor 1"}`))
	mux.ServeHTTP(httptest.NewRecorder(), reg)

	// Heartbeat on existing device
	hbPayload := `{"timestamp": "2026-09-30T10:30:00Z", "status": "OK", "cpu_usage": 32.5, "signal_strength": -70}`
	hbReq := httptest.NewRequest(http.MethodPost, "/devices/device-01/heartbeat", bytes.NewBufferString(hbPayload))
	hbRec := httptest.NewRecorder()
	mux.ServeHTTP(hbRec, hbReq)

	if hbRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for heartbeat, got %d: %s", hbRec.Code, hbRec.Body.String())
	}

	var hbResp device.DeviceResponse
	if err := json.Unmarshal(hbRec.Body.Bytes(), &hbResp); err != nil {
		t.Fatalf("failed decoding heartbeat response: %v", err)
	}
	if hbResp.Status != device.StatusOnline {
		t.Errorf("expected status ONLINE after heartbeat, got %s", hbResp.Status)
	}
	if hbResp.LastHeartbeat == nil {
		t.Errorf("expected non-nil last_heartbeat timestamp")
	}

	// Heartbeat on non-existent device -> 404 Not Found
	hbUnknownReq := httptest.NewRequest(http.MethodPost, "/devices/unknown-99/heartbeat", bytes.NewBufferString(hbPayload))
	hbUnknownRec := httptest.NewRecorder()
	mux.ServeHTTP(hbUnknownRec, hbUnknownReq)
	if hbUnknownRec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown device heartbeat, got %d", hbUnknownRec.Code)
	}

	// GET /devices/device-01
	getReq := httptest.NewRequest(http.MethodGet, "/devices/device-01", nil)
	getRec := httptest.NewRecorder()
	mux.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for GET /devices/device-01, got %d", getRec.Code)
	}

	// GET /devices/unknown-99 -> 404
	getUnknownReq := httptest.NewRequest(http.MethodGet, "/devices/unknown-99", nil)
	getUnknownRec := httptest.NewRecorder()
	mux.ServeHTTP(getUnknownRec, getUnknownReq)
	if getUnknownRec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown device lookup, got %d", getUnknownRec.Code)
	}
}

func TestAPIListAndSummary(t *testing.T) {
	_, mux := setupTestServer(30 * time.Second)

	// Register 3 devices: dev-1, dev-2, dev-3
	devices := []string{"dev-1", "dev-2", "dev-3"}
	for _, id := range devices {
		b := fmt.Sprintf(`{"id": "%s", "name": "Device %s"}`, id, id)
		mux.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/devices", bytes.NewBufferString(b)))
	}

	// Send heartbeats only to dev-1 and dev-2
	mux.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/devices/dev-1/heartbeat", nil))
	mux.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/devices/dev-2/heartbeat", nil))

	// GET /devices
	listRec := httptest.NewRecorder()
	mux.ServeHTTP(listRec, httptest.NewRequest(http.MethodGet, "/devices", nil))
	if listRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for GET /devices, got %d", listRec.Code)
	}

	var list []device.DeviceResponse
	if err := json.Unmarshal(listRec.Body.Bytes(), &list); err != nil {
		t.Fatalf("failed decoding devices list: %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("expected 3 devices in list, got %d", len(list))
	}

	// GET /summary
	sumRec := httptest.NewRecorder()
	mux.ServeHTTP(sumRec, httptest.NewRequest(http.MethodGet, "/summary", nil))
	if sumRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for GET /summary, got %d", sumRec.Code)
	}

	var summary device.FleetSummary
	if err := json.Unmarshal(sumRec.Body.Bytes(), &summary); err != nil {
		t.Fatalf("failed decoding summary: %v", err)
	}
	if summary.Total != 3 || summary.Online != 2 || summary.Offline != 1 {
		t.Errorf("unexpected summary: %+v (expected total:3, online:2, offline:1)", summary)
	}
}

func TestAPITimeoutOnlineToOffline(t *testing.T) {
	service, mux := setupTestServer(30 * time.Second)

	simTime := time.Date(2026, 9, 30, 15, 0, 0, 0, time.UTC)
	service.SetNowFunc(func() time.Time { return simTime })

	// Register device-01
	mux.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/devices", bytes.NewBufferString(`{"id": "device-01", "name": "Device 01"}`)))

	// Send heartbeat at 15:00:00
	mux.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/devices/device-01/heartbeat", nil))

	// Check status at 15:00:00 -> ONLINE
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/devices/device-01", nil))
	var resp device.DeviceResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Status != device.StatusOnline {
		t.Errorf("expected ONLINE, got %s", resp.Status)
	}

	// Advance time to 15:00:30 (exact boundary) -> ONLINE
	simTime = simTime.Add(30 * time.Second)
	rec30 := httptest.NewRecorder()
	mux.ServeHTTP(rec30, httptest.NewRequest(http.MethodGet, "/devices/device-01", nil))
	_ = json.Unmarshal(rec30.Body.Bytes(), &resp)
	if resp.Status != device.StatusOnline {
		t.Errorf("expected ONLINE at 30s boundary, got %s", resp.Status)
	}

	// Advance time to 15:00:31 (+31s elapsed) -> OFFLINE
	simTime = simTime.Add(1 * time.Second)
	rec31 := httptest.NewRecorder()
	mux.ServeHTTP(rec31, httptest.NewRequest(http.MethodGet, "/devices/device-01", nil))
	_ = json.Unmarshal(rec31.Body.Bytes(), &resp)
	if resp.Status != device.StatusOffline {
		t.Errorf("expected OFFLINE at 31s, got %s", resp.Status)
	}

	// Advance summary check at +31s -> Offline count = 1
	recSum := httptest.NewRecorder()
	mux.ServeHTTP(recSum, httptest.NewRequest(http.MethodGet, "/summary", nil))
	var sum device.FleetSummary
	_ = json.Unmarshal(recSum.Body.Bytes(), &sum)
	if sum.Online != 0 || sum.Offline != 1 {
		t.Errorf("expected summary {total:1, online:0, offline:1}, got %+v", sum)
	}
}
