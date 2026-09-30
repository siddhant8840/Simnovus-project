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

  // Handle manual heartbeat trigger
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
        fetchData()
      }
    } catch (err) {
      console.error('Error sending heartbeat:', err)
    }
  }

  // Calculate elapsed time and remaining seconds before timeout (30s)
  const formatHeartbeatInfo = (lastHeartbeat) => {
    if (!lastHeartbeat) {
      return { elapsedText: 'Never received', countdownText: 'OFFLINE' }
    }

    const hbDate = new Date(lastHeartbeat)
    const elapsedSeconds = Math.max(0, Math.floor((currentTime - hbDate) / 1000))

    if (elapsedSeconds <= 30) {
      const remaining = 30 - elapsedSeconds
      return {
        elapsedText: `${elapsedSeconds}s ago`,
        countdownText: `${remaining}s until timeout`
      }
    }

    return {
      elapsedText: `${elapsedSeconds}s ago`,
      countdownText: 'Timeout expired (>30s)'
    }
  }

  // Filter and search
  const filteredDevices = devices.filter((dev) => {
    const matchesFilter =
      filter === 'ALL' ||
      (filter === 'ONLINE' && dev.status === 'ONLINE') ||
      (filter === 'OFFLINE' && dev.status === 'OFFLINE')

    const matchesSearch =
      dev.id.toLowerCase().includes(search.toLowerCase()) ||
      dev.name.toLowerCase().includes(search.toLowerCase())

    return matchesFilter && matchesSearch
  })

  const healthPercent =
    summary.total > 0 ? Math.round((summary.online / summary.total) * 100) : 0

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

      {/* Metrics Fleet Summary Cards */}
      <div className="metrics-grid">
        <div className="metric-card total">
          <div className="metric-label">Total Devices</div>
          <div className="metric-value">{summary.total}</div>
          <div className="metric-sub">Registered in fleet map</div>
        </div>

        <div className="metric-card online">
          <div className="metric-label">
            Online Devices
            <span className="pulsing-dot online" />
          </div>
          <div className="metric-value" style={{ color: '#34d399' }}>
            {summary.online}
          </div>
          <div className="metric-sub">Heartbeat within ≤ 30s</div>
        </div>

        <div className="metric-card offline">
          <div className="metric-label">
            Offline Devices
            <span className="pulsing-dot offline" />
          </div>
          <div className="metric-value" style={{ color: '#fb7185' }}>
            {summary.offline}
          </div>
          <div className="metric-sub">No heartbeat for &gt; 30s</div>
        </div>

        <div className="metric-card health">
          <div className="metric-label">Fleet Availability</div>
          <div className="metric-value" style={{ color: '#c084fc' }}>
            {healthPercent}%
          </div>
          <div className="metric-sub">
            {summary.online} of {summary.total} operational
          </div>
        </div>
      </div>

      {/* Main Devices Table */}
      <div className="table-card">
        <div className="table-header-bar">
          <div className="table-title">Fleet Devices ({filteredDevices.length})</div>

          <div style={{ display: 'flex', gap: '1rem', flexWrap: 'wrap' }}>
            <div className="filter-group">
              <button
                className={`filter-btn ${filter === 'ALL' ? 'active' : ''}`}
                onClick={() => setFilter('ALL')}
              >
                All ({devices.length})
              </button>
              <button
                className={`filter-btn ${filter === 'ONLINE' ? 'active' : ''}`}
                onClick={() => setFilter('ONLINE')}
              >
                Online ({summary.online})
              </button>
              <button
                className={`filter-btn ${filter === 'OFFLINE' ? 'active' : ''}`}
                onClick={() => setFilter('OFFLINE')}
              >
                Offline ({summary.offline})
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
              Register a device or start the Go simulator (<code>go run ./cmd/simulator</code>).
            </p>
          </div>
        ) : (
          <table className="device-table">
            <thead>
              <tr>
                <th>Device ID</th>
                <th>Device Name</th>
                <th>Status</th>
                <th>Last Heartbeat</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              {filteredDevices.map((dev) => {
                const isOnline = dev.status === 'ONLINE'
                const { elapsedText, countdownText } = formatHeartbeatInfo(dev.last_heartbeat)

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
                        {dev.status}
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
              <div
                style={{
                  background: 'rgba(244, 63, 94, 0.1)',
                  color: '#fb7185',
                  padding: '0.5rem 0.75rem',
                  borderRadius: '6px',
                  marginBottom: '1rem',
                  fontSize: '0.85rem'
                }}
              >
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
