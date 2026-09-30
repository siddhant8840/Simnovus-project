# How to Run Mini Device Fleet Monitor

This guide provides simple, step-by-step instructions to run the **Go Backend**, **React Frontend**, **Device Simulator**, and **Automated Tests**.

---

## Prerequisites

* **Go 1.22+** (Go 1.24+ recommended)
* **Node.js & npm** *(Optional — only needed if modifying or running the React frontend in Vite dev mode)*
* A modern web browser (Chrome, Edge, Firefox, Brave, Safari)

---

## 🚀 Quick Start (Fastest Way)

### Step 1: Start the Backend Server
Open a terminal in the project root (`EXAM/`) and run:

```bash
go run ./cmd/server
```

> **Note:** The server starts on `http://localhost:8080`. It serves both the **REST API** and the pre-built **React Frontend UI** automatically!

### Step 2: Open the React Dashboard
Open your browser and navigate to:
👉 **[http://localhost:8080/](http://localhost:8080/)**

You will see the live **Mini Device Fleet Monitor** dashboard with:
* Real-time metrics (Total, Online, Offline, Fleet Availability %)
* Search & status filter tabs (All, Online, Offline)
* Live countdown timers showing seconds until the 30-second timeout
* An interactive **⚡ Heartbeat** button to revive/refresh any device

### Step 3: Run the Multi-Device Simulator
Open a **second terminal** in the project root (`EXAM/`) and run:

```bash
go run ./cmd/simulator
```

* This automatically registers and starts **5 devices** (`device-01` through `device-05`).
* Each device sends a heartbeat with simulated telemetry (`cpu_usage`, `signal_strength`) every 5 seconds.
* Check your browser at `http://localhost:8080/` — all 5 devices will turn **ONLINE** with pulsing emerald badges!

---

## 🧪 Demonstrating the 30-Second Timeout Rule

To observe the 30-second timeout in action:

1. Keep your browser open at `http://localhost:8080/`.
2. In the **Simulator terminal**, type:
   ```text
   stop device-03
   ```
   *(Or launch the simulator with `go run ./cmd/simulator -stop=device-03`)*
3. Watch the browser screen:
   * The countdown for `device-03` counts down from 30 seconds.
   * Exactly after 30 seconds of no heartbeats, `device-03` automatically transitions to **OFFLINE** with a red badge!
   * The fleet metrics card updates immediately (`Offline: 1`, `Online: 4`).
4. To revive the device, you can either:
   * Click the **⚡ Heartbeat** button in the browser row for `device-03`.
   * Or type `start device-03` in the simulator terminal.
   * `device-03` instantly returns to **ONLINE**!

---

## 💻 Optional: Running React in Development Mode (Vite HMR)

If you want to edit or develop the React frontend components with hot module reloading:

1. Open a terminal in the `frontend/` directory:
   ```bash
   cd frontend
   npm install
   npm run dev
   ```
2. Open your browser at:
   👉 **[http://localhost:5173/](http://localhost:5173/)**
   *(Vite automatically proxies API requests to the Go backend on port `8080`).*

---

## 🧪 Running Automated Tests

Run the full suite of unit, boundary, and HTTP integration tests:

```bash
# Run all tests with verbose output
go test -v ./...

# Run without cache to verify fresh execution
go test -count=1 ./...
```

### What the Tests Verify
* **Status Boundary Conditions**: Tests exact elapsed durations at $t = 0\text{s}$, $29\text{s}$ (ONLINE), $30\text{s}$ (ONLINE), and $31\text{s}$ (OFFLINE).
* **Validation**: Rejection of empty IDs/names (400 Bad Request) and malformed payloads.
* **Conflict Prevention**: Rejection of duplicate registrations (409 Conflict).
* **Missing Devices**: Heartbeats to nonexistent devices (404 Not Found).
* **Concurrency**: 20 concurrent goroutines performing simultaneous read and write operations.

---

## 📡 Testing via cURL / Command Line

You can also interact with the REST APIs directly via `curl`:

### 1. Register a Device
```bash
curl -X POST http://localhost:8080/devices \
  -H "Content-Type: application/json" \
  -d '{"id": "device-06", "name": "Lab Device 06"}'
```

### 2. Send Heartbeat
```bash
curl -X POST http://localhost:8080/devices/device-06/heartbeat \
  -H "Content-Type: application/json" \
  -d '{"timestamp": "2026-09-30T10:30:00Z", "status": "OK"}'
```

### 3. List All Devices
```bash
curl http://localhost:8080/devices
```

### 4. Get Single Device Details
```bash
curl http://localhost:8080/devices/device-06
```

### 5. Get Fleet Summary
```bash
curl http://localhost:8080/summary
```
*Sample Response:*
```json
{
  "total": 6,
  "online": 5,
  "offline": 1
}
```

---

## ⚙️ Configuration (Environment Variables)

| Variable | Default | Description |
|---|---|---|
| `PORT` | `8080` | Port for the HTTP server |
| `HEARTBEAT_TIMEOUT` | `30s` | Maximum duration before a device becomes OFFLINE |

Example with custom settings:
```bash
# PowerShell
$env:PORT="9000"; $env:HEARTBEAT_TIMEOUT="45s"; go run ./cmd/server

# Bash / Linux / macOS
PORT=9000 HEARTBEAT_TIMEOUT=45s go run ./cmd/server
```
