package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
)

type DeviceConfig struct {
	ID   string
	Name string
}

type HeartbeatPayload struct {
	Timestamp      string  `json:"timestamp"`
	Status         string  `json:"status"`
	CPUUsage       float64 `json:"cpu_usage"`
	SignalStrength int     `json:"signal_strength"`
}

type Simulator struct {
	serverURL string
	interval  time.Duration
	client    *http.Client

	mu      sync.Mutex
	devices map[string]context.CancelFunc
}

func NewSimulator(serverURL string, interval time.Duration) *Simulator {
	return &Simulator{
		serverURL: strings.TrimRight(serverURL, "/"),
		interval:  interval,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
		devices: make(map[string]context.CancelFunc),
	}
}

// RegisterDevice registers a device on the server if not already registered.
func (s *Simulator) RegisterDevice(id, name string) error {
	payload, _ := json.Marshal(map[string]string{
		"id":   id,
		"name": name,
	})

	resp, err := s.client.Post(s.serverURL+"/devices", "application/json", bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("connection error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusCreated {
		slog.Info("Device registered successfully", "id", id, "name", name)
		return nil
	}
	if resp.StatusCode == http.StatusConflict {
		slog.Info("Device already registered on server", "id", id)
		return nil
	}

	return fmt.Errorf("unexpected status %d registering device %s", resp.StatusCode, id)
}

// StartDevice initiates a background goroutine sending heartbeats periodically.
func (s *Simulator) StartDevice(id string, startDelay time.Duration) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, running := s.devices[id]; running {
		slog.Warn("Device is already running", "id", id)
		return false
	}

	ctx, cancel := context.WithCancel(context.Background())
	s.devices[id] = cancel

	go s.heartbeatLoop(ctx, id, startDelay)
	if startDelay > 0 {
		slog.Info("Scheduled device heartbeats", "id", id, "interval", s.interval.String(), "initial_delay", startDelay.String())
	} else {
		slog.Info("Started heartbeats for device", "id", id, "interval", s.interval.String())
	}
	return true
}

// StopDevice halts the heartbeat loop for a device, allowing it to time out to OFFLINE.
func (s *Simulator) StopDevice(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	cancel, running := s.devices[id]
	if !running {
		slog.Warn("Device is not currently sending heartbeats", "id", id)
		return false
	}

	cancel()
	delete(s.devices, id)
	slog.Warn("STOPPED device heartbeats (device will become OFFLINE after 30s timeout)", "id", id)
	return true
}

func (s *Simulator) heartbeatLoop(ctx context.Context, id string, startDelay time.Duration) {
	if startDelay > 0 {
		select {
		case <-ctx.Done():
			return
		case <-time.After(startDelay):
		}
	}

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	// Send an immediate heartbeat for this device after its offset
	s.sendHeartbeat(id)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.sendHeartbeat(id)
		}
	}
}

func (s *Simulator) sendHeartbeat(id string) {
	payload := HeartbeatPayload{
		Timestamp:      time.Now().UTC().Format(time.RFC3339),
		Status:         "OK",
		CPUUsage:       float64(20 + rand.Intn(45)),
		SignalStrength: -(50 + rand.Intn(35)),
	}

	data, err := json.Marshal(payload)
	if err != nil {
		slog.Error("failed to serialize heartbeat payload", "id", id, "error", err)
		return
	}

	url := fmt.Sprintf("%s/devices/%s/heartbeat", s.serverURL, id)
	resp, err := s.client.Post(url, "application/json", bytes.NewBuffer(data))
	if err != nil {
		slog.Error("failed to send heartbeat", "id", id, "error", err.Error())
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		slog.Info("Heartbeat sent", "id", id, "status", payload.Status, "cpu", payload.CPUUsage)
	} else {
		slog.Warn("Heartbeat rejected", "id", id, "status_code", resp.StatusCode)
	}
}

func main() {
	serverURL := flag.String("server", "http://localhost:8080", "Server base URL")
	interval := flag.Duration("interval", 5*time.Second, "Heartbeat interval")
	count := flag.Int("count", 5, "Number of devices to simulate")
	stopTarget := flag.String("stop", "", "Device ID to stop immediately after start (e.g. device-03)")
	interactive := flag.Bool("interactive", true, "Enable interactive CLI terminal controls")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	fmt.Println("==================================================")
	fmt.Println("      Mini Device Fleet Simulator (Go)            ")
	fmt.Println("==================================================")
	fmt.Printf("Server:   %s\n", *serverURL)
	fmt.Printf("Interval: %s\n", interval.String())
	fmt.Printf("Count:    %d devices\n", *count)
	fmt.Println("--------------------------------------------------")

	sim := NewSimulator(*serverURL, *interval)

	// Step 1: Register and start simulated devices with staggered offsets
	// This ensures each device has an independent heartbeat cadence and timestamp
	for i := 1; i <= *count; i++ {
		id := fmt.Sprintf("device-%02d", i)
		name := fmt.Sprintf("Lab Sensor %02d", i)

		if err := sim.RegisterDevice(id, name); err != nil {
			slog.Error("Could not register device", "id", id, "error", err)
		}
		// Stagger device start across the interval window (e.g. 0s, 1s, 2s, 3s, 4s)
		staggerDelay := time.Duration(i-1) * (*interval / time.Duration(*count))
		sim.StartDevice(id, staggerDelay)
	}

	// Step 2: If a target stop device was supplied via CLI flag, stop it
	if *stopTarget != "" {
		fmt.Printf("\n--> Stopping %s as requested by flag. Watch it turn OFFLINE in 30s.\n", *stopTarget)
		sim.StopDevice(*stopTarget)
	}

	// Handle OS interrupt signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	if *interactive {
		fmt.Println("\nInteractive commands available:")
		fmt.Println("  stop <id>   - Stop sending heartbeats for a device (e.g. stop device-03)")
		fmt.Println("  start <id>  - Resume sending heartbeats for a device (e.g. start device-03)")
		fmt.Println("  help        - Show this help message")
		fmt.Println("  exit/quit   - Exit simulator")
		fmt.Println()

		scanner := bufio.NewScanner(os.Stdin)
		go func() {
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if line == "" {
					continue
				}
				parts := strings.Fields(line)
				cmd := strings.ToLower(parts[0])

				switch cmd {
				case "stop":
					if len(parts) < 2 {
						fmt.Println("Usage: stop <device-id>")
						continue
					}
					sim.StopDevice(parts[1])
				case "start":
					if len(parts) < 2 {
						fmt.Println("Usage: start <device-id>")
						continue
					}
					sim.StartDevice(parts[1], 0)
				case "help":
					fmt.Println("Commands: stop <id>, start <id>, exit")
				case "exit", "quit":
					sigChan <- os.Interrupt
					return
				default:
					fmt.Println("Unknown command. Type 'help' for command list.")
				}
			}
		}()
	}

	<-sigChan
	fmt.Println("\nShutting down simulator...")
}
