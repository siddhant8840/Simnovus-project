import React, { useState, useEffect, useCallback } from 'react'

function App() {
  const [devices, setDevices] = useState([])
  const [summary, setSummary] = useState({ total: 0, online: 0, offline: 0 })
  const [filter, setFilter] = useState('ALL')
  const [search, setSearch] = useState('')
  const [autoRefresh, setAutoRefresh] = useState(true)
  const [isModalOpen, setIsModalOpen] = useState(false)
  const [newDevice, setNewDevice] = useState({ id: '', name: '' })
  const [errorMsg, setErrorMsg] = useState('')
  const [currentTime, setCurrentTime] = useState(new Date())
  const [isSidePanelOpen, setIsSidePanelOpen] = useState(true)
  const [activeSideTab, setActiveSideTab] = useState('quickstart') // 'quickstart' | 'polling' | 'heartbeat' | 'arch'

  // Update current time clock every second for live countdowns
  useEffect(() => {
    const timer = setInterval(() => setCurrentTime(new Date()), 1000)
    return () => clearInterval(timer)
  }, [])

  // Fetch summary and device list from backend API
  const fetchData = useCallback(async () => {
    try {
      const [sumRes, devRes] = await Promise.all([
        fetch('/summary'),
        fetch('/devices')
      ])

      if (sumRes.ok) {
        const sumData = await sumRes.json()
        setSummary(sumData)
      }

      if (devRes.ok) {
        const devData = await devRes.json()
        setDevices(devData || [])
      }
    } catch (err) {
      console.error('Error fetching fleet data:', err)
    }
  }, [])

  // Initial load and polling
  useEffect(() => {
    fetchData()

    if (!autoRefresh) return

    const interval = setInterval(() => {
      fetchData()
    }, 2000)

    return () => clearInterval(interval)
  }, [fetchData, autoRefresh])

  // Handle register new device
  const handleRegister = async (e) => {
    e.preventDefault()
    setErrorMsg('')

    if (!newDevice.id.trim() || !newDevice.name.trim()) {
      setErrorMsg('Both Device ID and Name are required.')
      return
    }

    try {
      const res = await fetch('/devices', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          id: newDevice.id.trim(),
          name: newDevice.name.trim()
        })
      })

      if (res.status === 201) {
        setIsModalOpen(false)
        setNewDevice({ id: '', name: '' })
        fetchData()
      } else if (res.status === 409) {
        setErrorMsg('Device with this ID already exists.')
      } else {
        const data = await res.json()
        setErrorMsg(data.error || 'Failed to register device.')
      }
    } catch (err) {
      setErrorMsg('Network error registering device.')
    }
  }

  // Quick helper to register a sample device directly from the side panel
  const handleQuickRegisterSample = async () => {
    const randomNum = Math.floor(10 + Math.random() * 90)
    const sampleId = `sensor-node-${randomNum}`
    const sampleName = `Edge Sensor Unit ${randomNum}`

    try {
      const res = await fetch('/devices', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ id: sampleId, name: sampleName })
      })
      if (res.ok || res.status === 201) {
        fetchData()
      }
    } catch (err) {
      console.error('Quick register failed:', err)
    }
  }

  // Handle manual heartbeat trigger for a single device
  const handleSendHeartbeat = async (id) => {
    try {
      const payload = {
        timestamp: new Date().toISOString(),
        status: 'OK',
        cpu_usage: Math.floor(20 + Math.random() * 40),
        signal_strength: -Math.floor(50 + Math.random() * 30)
      }

      const res = await fetch(`/devices/${id}/heartbeat`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload)
      })

      if (res.ok) {
        const updatedDev = await res.json()
        setDevices((prevDevices) =>
          prevDevices.map((d) => (d.id === id ? updatedDev : d))
        )

        const sumRes = await fetch('/summary')
        if (sumRes.ok) {
          const sumData = await sumRes.json()
          setSummary(sumData)
        }
      }
    } catch (err) {
      console.error('Error sending heartbeat:', err)
    }
  }

  // Calculate real-time status strictly based on the 30-second timeout window
  const getDeviceStatus = (dev) => {
    if (!dev.last_heartbeat) return 'OFFLINE'
    const hbDate = new Date(dev.last_heartbeat)
    const elapsedSeconds = Math.max(0, Math.floor((currentTime - hbDate) / 1000))
    return elapsedSeconds <= 30 ? 'ONLINE' : 'OFFLINE'
  }

  // Calculate elapsed time and countdown strictly from EACH individual device's own lastHeartbeat
  const formatHeartbeatInfo = (lastHeartbeat) => {
    if (!lastHeartbeat) {
      return {
        timestampFormatted: '—',
        elapsedText: 'Never received',
        countdownText: 'OFFLINE'
      }
    }

    const hbDate = new Date(lastHeartbeat)
    const elapsedSeconds = Math.max(0, Math.floor((currentTime - hbDate) / 1000))
    const timestampFormatted = hbDate.toLocaleTimeString([], { hour12: false })

    if (elapsedSeconds <= 30) {
      const remaining = 30 - elapsedSeconds
      return {
        timestampFormatted,
        elapsedText: elapsedSeconds === 0 ? '0s ago (Just now)' : `${elapsedSeconds}s ago`,
        countdownText: `${remaining}s until timeout`
      }
    }

    return {
      timestampFormatted,
      elapsedText: `${elapsedSeconds}s ago`,
      countdownText: 'Timeout expired (>30s)'
    }
  }

  // Derive real-time metrics dynamically every second
  const computedOnline = devices.filter((d) => getDeviceStatus(d) === 'ONLINE').length
  const computedOffline = devices.length - computedOnline
  const computedTotal = devices.length
  const healthPercent =
    computedTotal > 0 ? Math.round((computedOnline / computedTotal) * 100) : 0

  // Filter and search using live real-time status
  const filteredDevices = devices
    .map((dev) => ({
      ...dev,
      currentStatus: getDeviceStatus(dev)
    }))
    .filter((dev) => {
      const matchesFilter =
        filter === 'ALL' ||
        (filter === 'ONLINE' && dev.currentStatus === 'ONLINE') ||
        (filter === 'OFFLINE' && dev.currentStatus === 'OFFLINE')

      const matchesSearch =
        dev.id.toLowerCase().includes(search.toLowerCase()) ||
        dev.name.toLowerCase().includes(search.toLowerCase())

      return matchesFilter && matchesSearch
    })

  return (
    <div className="dashboard-container">
      {/* Top Header */}
      <header className="header">
        <div className="brand">
          <div className="brand-icon">
            <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="#ffffff" strokeWidth="2.5">
              <path d="M12 2v4M12 18v4M4.93 4.93l2.83 2.83M16.24 16.24l2.83 2.83M2 12h4M18 12h4M4.93 19.07l2.83-2.83M16.24 7.76l2.83-2.83" />
            </svg>
          </div>
          <div>
            <h1>Mini Device Fleet Monitor</h1>
            <span>Real-time IoT Concurrency Dashboard • 30s Timeout Window</span>
          </div>
        </div>

        <div className="controls">
          <button
            className={`btn ${isSidePanelOpen ? 'btn-active-panel' : 'btn-secondary'}`}
            onClick={() => setIsSidePanelOpen(!isSidePanelOpen)}
            title="Toggle Quick Setup & Explanations Side Panel"
          >
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" style={{ marginRight: '4px' }}>
              <rect x="3" y="3" width="18" height="18" rx="2" ry="2"/>
              <line x1="15" y1="3" x2="15" y2="21"/>
            </svg>
            {isSidePanelOpen ? 'Hide Guide' : '📖 Side Panel Guide'}
          </button>

          <button
            className="btn btn-secondary"
            onClick={() => setAutoRefresh(!autoRefresh)}
            title="Toggle live 2s polling"
          >
            <span className={`pulsing-dot ${autoRefresh ? 'online' : 'offline'}`} />
            {autoRefresh ? 'Live Polling: ON' : 'Live Polling: PAUSED'}
          </button>

          <button className="btn btn-secondary" onClick={fetchData}>
            ↻ Refresh
          </button>

          <button className="btn btn-primary" onClick={() => setIsModalOpen(true)}>
            + Register Device
          </button>
        </div>
      </header>

      {/* Main Grid: Left is Fleet Tables & Metrics, Right is Guide Side Panel */}
      <div className={`dashboard-layout ${isSidePanelOpen ? 'with-sidebar' : 'full-width'}`}>
        <main className="main-content">
          {/* Metrics Fleet Summary Cards */}
          <div className="metrics-grid">
            <div className="metric-card total">
              <div className="metric-label">Total Devices</div>
              <div className="metric-value">{computedTotal}</div>
              <div className="metric-sub">Registered in fleet map</div>
            </div>

            <div className="metric-card online">
              <div className="metric-label">
                Online Devices
                <span className="pulsing-dot online" />
              </div>
              <div className="metric-value">
                {computedOnline}
              </div>
              <div className="metric-sub">Heartbeat within ≤ 30s</div>
            </div>

            <div className="metric-card offline">
              <div className="metric-label">
                Offline Devices
                <span className="pulsing-dot offline" />
              </div>
              <div className="metric-value">
                {computedOffline}
              </div>
              <div className="metric-sub">No heartbeat for &gt; 30s</div>
            </div>

            <div className="metric-card health">
              <div className="metric-label">Fleet Availability</div>
              <div className="metric-value">
                {healthPercent}%
              </div>
              <div className="metric-sub">
                {computedOnline} of {computedTotal} operational
              </div>
            </div>
          </div>

          {/* Main Devices Table */}
          <div className="table-card">
            <div className="table-header-bar">
              <div className="table-title">Fleet Devices ({filteredDevices.length})</div>

              <div className="table-controls">
                <div className="filter-group">
                  <button
                    className={`filter-btn ${filter === 'ALL' ? 'active' : ''}`}
                    onClick={() => setFilter('ALL')}
                  >
                    All ({computedTotal})
                  </button>
                  <button
                    className={`filter-btn ${filter === 'ONLINE' ? 'active' : ''}`}
                    onClick={() => setFilter('ONLINE')}
                  >
                    Online ({computedOnline})
                  </button>
                  <button
                    className={`filter-btn ${filter === 'OFFLINE' ? 'active' : ''}`}
                    onClick={() => setFilter('OFFLINE')}
                  >
                    Offline ({computedOffline})
                  </button>
                </div>

                <input
                  type="text"
                  className="search-input"
                  placeholder="Search by ID or name..."
                  value={search}
                  onChange={(e) => setSearch(e.target.value)}
                />
              </div>
            </div>

            {filteredDevices.length === 0 ? (
              <div className="empty-state">
                <p>No devices matching your criteria.</p>
                <p style={{ marginTop: '0.5rem', fontSize: '0.85rem' }}>
                  Register a device or use the <strong>Quick Setup</strong> side panel to test!
                </p>
                <div style={{ marginTop: '1rem', display: 'flex', gap: '0.5rem', justifyContent: 'center' }}>
                  <button className="btn btn-primary btn-sm" onClick={() => setIsModalOpen(true)}>
                    + Register Device
                  </button>
                  <button className="btn btn-secondary btn-sm" onClick={handleQuickRegisterSample}>
                    ⚡ Quick Sample Device
                  </button>
                </div>
              </div>
            ) : (
              <div className="table-scroll">
                <table className="device-table">
                  <thead>
                    <tr>
                      <th>Device ID</th>
                      <th>Device Name</th>
                      <th>Status</th>
                      <th>Backend Timestamp</th>
                      <th>Elapsed (30s Window)</th>
                      <th>Actions</th>
                    </tr>
                  </thead>
                  <tbody>
                    {filteredDevices.map((dev) => {
                      const isOnline = dev.currentStatus === 'ONLINE'
                      const { timestampFormatted, elapsedText, countdownText } = formatHeartbeatInfo(dev.last_heartbeat)

                      return (
                        <tr key={dev.id}>
                          <td>
                            <span className="device-id">{dev.id}</span>
                          </td>
                          <td>
                            <span className="device-name">{dev.name}</span>
                          </td>
                          <td>
                            <span className={`badge ${isOnline ? 'badge-online' : 'badge-offline'}`}>
                              <span className={`pulsing-dot ${isOnline ? 'online' : 'offline'}`} />
                              {dev.currentStatus}
                            </span>
                          </td>
                          <td>
                            <span style={{ fontFamily: 'var(--font-mono)', color: 'var(--accent-blue)', fontSize: '0.85rem' }}>
                              {timestampFormatted}
                            </span>
                          </td>
                          <td>
                            <span className="time-ago">{elapsedText}</span>
                            <span className="timeout-countdown">{countdownText}</span>
                          </td>
                          <td>
                            <button
                              className="btn btn-secondary btn-sm"
                              onClick={() => handleSendHeartbeat(dev.id)}
                              title="Send heartbeat to revive or refresh this device"
                            >
                              ⚡ Heartbeat
                            </button>
                          </td>
                        </tr>
                      )
                    })}
                  </tbody>
                </table>
              </div>
            )}
          </div>
        </main>

        {/* Side Panel: Quick Setup & Explanations */}
        {isSidePanelOpen && (
          <aside className="side-panel">
            <div className="side-panel-header">
              <div className="side-panel-title">
                <span className="side-panel-badge">GUIDE</span>
                <h3>Setup & Operations</h3>
              </div>
              <button
                className="close-btn-sm"
                onClick={() => setIsSidePanelOpen(false)}
                title="Collapse side panel"
              >
                ✕
              </button>
            </div>

            {/* Navigation Tabs inside Side Panel */}
            <div className="side-tabs">
              <button
                className={`side-tab-btn ${activeSideTab === 'quickstart' ? 'active' : ''}`}
                onClick={() => setActiveSideTab('quickstart')}
              >
                🚀 Quick Setup
              </button>
              <button
                className={`side-tab-btn ${activeSideTab === 'polling' ? 'active' : ''}`}
                onClick={() => setActiveSideTab('polling')}
              >
                🔄 Live Polling
              </button>
              <button
                className={`side-tab-btn ${activeSideTab === 'heartbeat' ? 'active' : ''}`}
                onClick={() => setActiveSideTab('heartbeat')}
              >
                ⚡ Heartbeats
              </button>
              <button
                className={`side-tab-btn ${activeSideTab === 'arch' ? 'active' : ''}`}
                onClick={() => setActiveSideTab('arch')}
              >
                ⚙️ Backend
              </button>
            </div>

            {/* Tab 1: Quick Setup */}
            {activeSideTab === 'quickstart' && (
              <div className="side-tab-content">
                <div className="guide-step">
                  <div className="step-number">1</div>
                  <div className="step-body">
                    <h4>Register a Device</h4>
                    <p>
                      First, register a device by clicking <strong>+ Register Device</strong> at the top, or click the quick action below.
                    </p>
                    <div className="step-action-box">
                      <button
                        className="btn btn-primary btn-sm"
                        onClick={() => setIsModalOpen(true)}
                        style={{ width: '100%' }}
                      >
                        + Open Register Modal
                      </button>
                      <button
                        className="btn btn-secondary btn-sm"
                        onClick={handleQuickRegisterSample}
                        style={{ width: '100%', marginTop: '0.4rem' }}
                      >
                        ⚡ 1-Click Sample Device
                      </button>
                    </div>
                  </div>
                </div>

                <div className="guide-step">
                  <div className="step-number">2</div>
                  <div className="step-body">
                    <h4>Click "⚡ Heartbeat" to Revive</h4>
                    <p>
                      New devices start in <span className="badge badge-offline">OFFLINE</span> state because no heartbeat has arrived yet.
                    </p>
                    <p style={{ marginTop: '0.35rem' }}>
                      Click the <strong>⚡ Heartbeat</strong> button in the device row to transmit a live telemetry ping.
                    </p>
                  </div>
                </div>

                <div className="guide-step">
                  <div className="step-number">3</div>
                  <div className="step-body">
                    <h4>30-Second Timeout Window</h4>
                    <p>
                      The device immediately turns <span className="badge badge-online">ONLINE</span>. A real-time countdown begins from <strong>30s down to 0s</strong>.
                    </p>
                    <p style={{ marginTop: '0.35rem' }}>
                      If 30 seconds pass without a new heartbeat, it flips back to <span className="badge badge-offline">OFFLINE</span> automatically.
                    </p>
                  </div>
                </div>
              </div>
            )}

            {/* Tab 2: What is Live Polling */}
            {activeSideTab === 'polling' && (
              <div className="side-tab-content">
                <div className="info-box">
                  <div className="info-box-header">
                    <span className="info-box-icon">🔄</span>
                    <h4>What is Live Polling?</h4>
                  </div>
                  <p>
                    <strong>Live Polling</strong> is an automated background sync loop running in the frontend.
                  </p>
                  <p style={{ marginTop: '0.4rem' }}>
                    Every <strong>2 seconds (2000ms)</strong>, the dashboard queries:
                  </p>
                  <ul className="guide-list">
                    <li><code>GET /summary</code>: Online/Offline/Total counts</li>
                    <li><code>GET /devices</code>: Device list with newest backend timestamps</li>
                  </ul>
                </div>

                <div className="info-box" style={{ marginTop: '0.8rem' }}>
                  <h4>Why is Polling Needed?</h4>
                  <p>
                    When multiple clients, background simulators, or other operators trigger heartbeats concurrently, live polling keeps your browser display 100% updated in real-time.
                  </p>
                </div>

                <div className="step-action-box" style={{ marginTop: '0.8rem' }}>
                  <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '0.5rem' }}>
                    <span style={{ fontSize: '0.82rem', fontWeight: '600' }}>Live Polling:</span>
                    <span className={`badge ${autoRefresh ? 'badge-online' : 'badge-offline'}`}>
                      {autoRefresh ? 'ACTIVE (2s)' : 'PAUSED'}
                    </span>
                  </div>
                  <button
                    className="btn btn-secondary btn-sm"
                    style={{ width: '100%' }}
                    onClick={() => setAutoRefresh(!autoRefresh)}
                  >
                    {autoRefresh ? 'Pause 2s Polling' : 'Resume 2s Polling'}
                  </button>
                </div>
              </div>
            )}

            {/* Tab 3: Heartbeat Mechanics */}
            {activeSideTab === 'heartbeat' && (
              <div className="side-tab-content">
                <div className="info-box">
                  <div className="info-box-header">
                    <span className="info-box-icon">⚡</span>
                    <h4>Heartbeat Telemetry</h4>
                  </div>
                  <p>
                    Each device sends periodic health pings to:
                  </p>
                  <pre className="code-snippet">POST /devices/:id/heartbeat</pre>
                  <p style={{ marginTop: '0.5rem' }}>
                    The payload includes ISO-8601 timestamps, CPU usage %, and WiFi/signal strength.
                  </p>
                </div>

                <div className="rule-card">
                  <h4>The 30-Second Rule</h4>
                  <div className="rule-item">
                    <span className="pulsing-dot online" />
                    <div>
                      <strong>ONLINE:</strong> Heartbeat received &le; 30 seconds ago.
                    </div>
                  </div>
                  <div className="rule-item" style={{ marginTop: '0.5rem' }}>
                    <span className="pulsing-dot offline" />
                    <div>
                      <strong>OFFLINE:</strong> No heartbeat received for &gt; 30 seconds.
                    </div>
                  </div>
                </div>

                <div className="info-box" style={{ marginTop: '0.8rem' }}>
                  <h4>Independent Per-Device Timers</h4>
                  <p>
                    Clicking <strong>⚡ Heartbeat</strong> only refreshes that single device. All other devices countdown strictly on their own independent timestamps!
                  </p>
                </div>
              </div>
            )}

            {/* Tab 4: Backend & Architecture */}
            {activeSideTab === 'arch' && (
              <div className="side-tab-content">
                <div className="info-box">
                  <div className="info-box-header">
                    <span className="info-box-icon">⚙️</span>
                    <h4>Go Concurrency Model</h4>
                  </div>
                  <p>
                    The Go server uses <code>sync.RWMutex</code> to guard the in-memory device registry:
                  </p>
                  <ul className="guide-list">
                    <li><strong><code>RLock()</code>:</strong> High-throughput concurrent reads for <code>/summary</code> and <code>/devices</code>.</li>
                    <li><strong><code>Lock()</code>:</strong> Thread-safe writes for registration and heartbeat updates.</li>
                  </ul>
                </div>

                <div className="info-box" style={{ marginTop: '0.8rem' }}>
                  <h4>Concurrent Simulator</h4>
                  <p>
                    Test 10 devices sending heartbeats concurrently:
                  </p>
                  <pre className="code-snippet">go run ./cmd/simulator</pre>
                </div>
              </div>
            )}
          </aside>
        )}
      </div>

      {/* Register Device Modal */}
      {isModalOpen && (
        <div className="modal-overlay" onClick={() => setIsModalOpen(false)}>
          <div className="modal" onClick={(e) => e.stopPropagation()}>
            <div className="modal-header">
              <h3 className="modal-title">Register New Device</h3>
              <button className="close-btn" onClick={() => setIsModalOpen(false)}>
                ✕
              </button>
            </div>

            {errorMsg && (
              <div className="form-error">
                {errorMsg}
              </div>
            )}

            <form onSubmit={handleRegister}>
              <div className="form-group">
                <label className="form-label">Device Identifier (ID)</label>
                <input
                  type="text"
                  className="form-input"
                  placeholder="e.g. device-06"
                  value={newDevice.id}
                  onChange={(e) => setNewDevice({ ...newDevice, id: e.target.value })}
                  required
                />
              </div>

              <div className="form-group">
                <label className="form-label">Device Name / Label</label>
                <input
                  type="text"
                  className="form-input"
                  placeholder="e.g. Backup Gateway North"
                  value={newDevice.name}
                  onChange={(e) => setNewDevice({ ...newDevice, name: e.target.value })}
                  required
                />
              </div>

              <div className="modal-actions">
                <button
                  type="button"
                  className="btn btn-secondary"
                  onClick={() => setIsModalOpen(false)}
                >
                  Cancel
                </button>
                <button type="submit" className="btn btn-primary">
                  Register Device
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  )
}

export default App
