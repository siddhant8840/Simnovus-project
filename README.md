# Mini Device Fleet Monitor

A lightweight, concurrent HTTP service written in Go to monitor a fleet of connected devices via periodic heartbeats.

---

## 1. What the Project Does

The **Mini Device Fleet Monitor** provides an HTTP API to register IoT/lab devices, receive heartbeats reporting health and telemetry, and query individual and fleet-wide status. 

### Core Heartbeat & Timeout Rule
* **ONLINE**: Device sent a heartbeat within the last **30 seconds** (`elapsed <= 30s`).
* **OFFLINE**: No heartbeat has been received for **more than 30 seconds** (`elapsed > 30s`) or no heartbeat has ever been received.
* **Authoritative Clock**: Heartbeat freshness is calculated based on the **server's receipt time** to eliminate issues caused by client clock drift or falsified timestamps.
* **Derived Status**: Status is computed on-demand upon query rather than maintaining a background ticking state machine, eliminating background thread overhead and race hazards.

---

## 2. Design & Architecture

The project follows a clean layered architecture adhering to separation of concerns:

```
                      ┌──────────────────────┐
                      │   Device Simulator   │
                      │  (goroutines/ticker) │
                      │ device-01 .. 05      │
                      └──────────┬───────────┘
                                 │ HTTP POST
                                 │ Heartbeats (every 5s)
                                 ▼
┌─────────────────────────────────────────────────────────────┐
│                    GO FLEET MONITOR                         │
│                                                             │
│   HTTP Handler Layer (internal/device/handler.go)           │
│   • Request parsing, validation, status codes, JSON I/O     │
│             ↓                                               │
│   Device Service Layer (internal/device/service.go)         │
│   • Core business rules: status calculation & summary       │
│             ↓                                               │
│   Repository Layer (internal/device/repository.go)          │
│   • Thread-safe in-memory map + sync.RWMutex                │
│             ↓                                               │
│   Domain Model (internal/device/model.go)                   │
│   • Device entity, DTOs, pure status evaluator              │
└──────────────────────────────┬──────────────────────────────┘
                               │
                         REST Endpoints
                               │
            ┌──────────────────┼──────────────────┐
            ▼                  ▼                  ▼
       GET /devices     GET /devices/{id}    GET /summary
```

### Directory Structure
```
EXAM/
├── cmd/
│   ├── server/
│   │   └── main.go           # Application entrypoint & HTTP server lifecycle
│   └── simulator/
│       └── main.go           # Multi-device concurrent heartbeat simulator
├── internal/
│   ├── api/
│   │   └── response.go       # Reusable JSON response and error serialization
│   └── device/
│       ├── model.go          # Domain models, request/response DTOs, status logic
│       ├── repository.go     # Thread-safe in-memory storage (RWMutex)
│       ├── service.go        # Fleet business logic and validation
│       ├── handler.go        # HTTP handlers & routing
│       └── device_test.go    # Unit tests (boundaries, concurrency, validation)
├── tests/
│   └── integration_test.go   # End-to-end HTTP API integration tests
├── Dockerfile                # Multi-stage container build
├── go.mod                    # Go module definition (standard library only)
├── .gitignore                # Standard Git ignore rules
└── README.md                 # Complete documentation & run guide
```

### Key Engineering Decisions
1. **Standard Library Only**: Built entirely using Go 1.24+ standard library (`net/http`, `encoding/json`, `sync`, `time`, `log/slog`) with zero third-party dependencies.
2. **Concurrency Safety**: The storage layer protects the underlying Go `map[string]Device` with `sync.RWMutex` (readers-writer lock). Multiple heartbeats arriving simultaneously are safely processed without data races.
3. **Derived Status (No Polling Loop)**: Rather than running an expensive background ticker checking every second, status is computed dynamically upon each request:
   $$\text{Status} = \begin{cases} \text{ONLINE} & \text{if } (t_{\text{now}} - t_{\text{last}}) \le 30\text{s} \\ \text{OFFLINE} & \text{otherwise} \end{cases}$$
4. **Authoritative Timestamping**: Although devices can submit telemetry timestamps, the server assigns its receipt timestamp as the source of truth for timeout calculations.

---

## 3. Prerequisites

* **Go 1.22+** (Go 1.24+ recommended)
* **Git** (for version control)
* *(Optional)* **Docker**

---

## 4. How to Build the Application

From the root directory:

```bash
# Build the server binary
go build -o bin/server ./cmd/server

# Build the simulator binary
go build -o bin/simulator ./cmd/simulator
```

*(On Windows PowerShell, use `bin\server.exe` and `bin\simulator.exe`)*

---

## 5. How to Run the Application

Start the fleet monitor server:

```bash
go run ./cmd/server
```

### Configuration (Environment Variables)
| Variable | Default | Description |
|---|---|---|
| `PORT` | `8080` | Port for the HTTP server |
| `HEARTBEAT_TIMEOUT` | `30s` | Maximum duration before a device becomes OFFLINE |

Example with custom settings:
```bash
# Linux/macOS
PORT=9000 HEARTBEAT_TIMEOUT=45s go run ./cmd/server

# Windows PowerShell
$env:PORT="9000"; $env:HEARTBEAT_TIMEOUT="45s"; go run ./cmd/server
```

### Accessing the React Web Frontend
Once the server is running, open your web browser to:
```
http://localhost:8080/
```
The React frontend is pre-built into `frontend/dist` and served directly by the Go backend!

#### Running React in Development Mode (Vite)
If you wish to run the React frontend independently with Hot Module Reloading (HMR):
```bash
cd frontend
npm install
npm run dev
```
Then visit `http://localhost:5173/` (Vite automatically proxies API requests to Go on `:8080`).

---

## 6. How to Run the Simulator

In a separate terminal window, start the device simulator:

```bash
go run ./cmd/simulator
```

By default, the simulator:
* Registers 5 devices (`device-01` through `device-05`).
* Spawns an independent goroutine for each device.
* Emits a heartbeat with simulated telemetry (`cpu_usage`, `signal_strength`) every 5 seconds.

### Simulator CLI Options
```bash
# Point to a custom server URL or change heartbeat interval
go run ./cmd/simulator -server=http://localhost:8080 -interval=5s -count=5

# Start with a specific device stopped to observe 30s timeout
go run ./cmd/simulator -stop=device-03
```

### Interactive Console Controls
While the simulator is running in your terminal, type:
* `stop device-03` : Stops sending heartbeats for `device-03`. After 30 seconds, `device-03` will transition to `OFFLINE`.
* `start device-03`: Resumes heartbeats for `device-03`, immediately bringing it back `ONLINE`.
* `quit` or `Ctrl+C`: Gracefully stops the simulator.

---

## 7. How to Run the Tests

Execute the automated test suite:

```bash
# Run all unit and integration tests with verbose output
go test -v ./...

# Run tests with race detection (requires CGO/gcc)
go test -race ./...
```

### Test Coverage Highlights
* **Boundary Conditions**: Tests exact elapsed durations at $t = 0\text{s}$, $29\text{s}$ (ONLINE), $30\text{s}$ (ONLINE), and $31\text{s}$ (OFFLINE).
* **Validation & Status Codes**: Validates rejection of empty IDs (400), duplicates (409 Conflict), and missing devices (404 Not Found).
* **Concurrency**: 20 goroutines performing simultaneous read and write operations against the repository to verify absence of data races.
* **HTTP Integration**: End-to-end tests using `net/http/httptest` simulating realistic HTTP requests and payloads.

---

## 8. Example API Requests

### 1. Register a Device
```bash
curl -X POST http://localhost:8080/devices \
  -H "Content-Type: application/json" \
  -d '{"id": "device-01", "name": "Lab Device 01"}'
```
**Response (`201 Created`):**
```json
{
  "id": "device-01",
  "name": "Lab Device 01",
  "status": "OFFLINE",
  "last_heartbeat": null
}
```

### 2. Send Heartbeat
```bash
curl -X POST http://localhost:8080/devices/device-01/heartbeat \
  -H "Content-Type: application/json" \
  -d '{
    "timestamp": "2026-09-30T10:30:00Z",
    "status": "OK",
    "cpu_usage": 42.5,
    "signal_strength": -68
  }'
```
**Response (`200 OK`):**
```json
{
  "id": "device-01",
  "name": "Lab Device 01",
  "status": "ONLINE",
  "last_heartbeat": "2026-09-30T10:30:00.123456Z"
}
```

### 3. List All Devices
```bash
curl http://localhost:8080/devices
```
**Response (`200 OK`):**
```json
[
  {
    "id": "device-01",
    "name": "Lab Device 01",
    "status": "ONLINE",
    "last_heartbeat": "2026-09-30T10:30:00.123456Z"
  },
  {
    "id": "device-02",
    "name": "Lab Device 02",
    "status": "OFFLINE",
    "last_heartbeat": null
  }
]
```

### 4. Get Device Details
```bash
curl http://localhost:8080/devices/device-01
```
**Response (`200 OK`):**
```json
{
  "id": "device-01",
  "name": "Lab Device 01",
  "status": "ONLINE",
  "last_heartbeat": "2026-09-30T10:30:00.123456Z"
}
```

### 5. Fleet Summary
```bash
curl http://localhost:8080/summary
```
**Response (`200 OK`):**
```json
{
  "total": 5,
  "online": 4,
  "offline": 1
}
```

---

## 9. Docker Support (Optional)

Run the containerized server:

```bash
# Build Docker image
docker build -t fleet-monitor .

# Run container
docker run -p 8080:8080 fleet-monitor
```

---

## 10. Assumptions Made

1. **In-Memory Storage**: Per the exercise specifications, in-memory storage using `sync.RWMutex` and `map[string]Device` is appropriate. Data resets upon service restart.
2. **Server Authoritative Time**: While the client may send a timestamp in the heartbeat payload, the server stamps `time.Now()` upon receipt to calculate the 30-second window, preventing device clock tampering or skew.
3. **Initial State**: A freshly registered device that has never sent a heartbeat is classified as `OFFLINE`.
4. **Heartbeat Payload Optionality**: Heartbeats accept empty bodies or extended metadata (`cpu_usage`, `signal_strength`).

---

## 11. Known Limitations

1. **Non-Persistent Data**: Restarting the process clears registered devices and heartbeat history.
2. **Single-Node Scale**: State is contained in process memory, meaning horizontal scaling across multiple instances requires a distributed store (e.g., Redis or PostgreSQL).
3. **No Authentication**: The API is unauthenticated, intended for internal network or evaluation environments.

---

## 12. What I Would Improve If I Had One Additional Day

1. **Persistent Storage (SQLite / PostgreSQL)**: Replace `MemoryRepository` with a SQL database implementation implementing the same `Repository` interface.
2. **Metrics & Observability**: Add Prometheus instrumentation (`/metrics`) tracking request latency, heartbeat arrival rates, and active online device gauges.
3. **Real-Time WebSockets / SSE**: Stream live device status transitions to connected operator dashboards without requiring polling.
4. **Device Authentication**: Implement mutual TLS (mTLS) or device API tokens to secure device-to-cloud communication.
5. **Historical Heartbeat Log & Telemetry Query**: Store time-series telemetry to visualize CPU usage and signal degradation trends over time.

---

## AI Usage

* **AI Tools Used**: Google DeepMind Agentic Coding Assistant (Antigravity).
* **What They Were Used For**: Architecture scaffolding, drafting initial Go structs and boilerplate HTTP routes, generating concurrent test suites, and formatting documentation.
* **Suggestion Changed / Improved**: 
  - *Initial Suggestion*: An early suggestion proposed running a background goroutine ticking every second to scan and mutate the status of devices in the map.
  - *My Decision / Improvement*: Rejected the background ticking worker in favor of **derived status calculation on read**. Computing status dynamically eliminates background thread churn, prevents cache coherence races, and simplifies boundary unit testing with zero dependency on `time.Sleep`.
* **Verified Before Submitting**: 
  - Personally verified that the 30-second boundary ($29\text{s} \to \text{ONLINE}$, $30\text{s} \to \text{ONLINE}$, $31\text{s} \to \text{OFFLINE}$) passes automated tests deterministically.
  - Verified concurrent access thread-safety with 20 parallel goroutines and race condition checks.
  - Verified simulator start/stop commands and verified `curl http://localhost:8080/summary` correctly tallies online/offline counts.
