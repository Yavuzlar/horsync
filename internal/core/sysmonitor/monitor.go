package sysmonitor

import (
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"

	"horsync/internal/models"
)

type Monitor struct {
	mu           sync.RWMutex
	currentStats models.Stats
	perfHistory  []models.PerformanceData
	stopCh       chan struct{}
}

var instance *Monitor
var once sync.Once

// GetInstance returns the singleton instance of the system monitor.
func GetInstance() *Monitor {
	once.Do(func() {
		instance = &Monitor{
			perfHistory: make([]models.PerformanceData, 0, 60), // Keep up to 60 ticks
			stopCh:      make(chan struct{}),
		}
	})
	return instance
}

// Start begins the background monitoring task.
// Start begins the background monitoring task.
func (m *Monitor) Start() {
	go m.loop()
}

// Stop halts the background monitoring task.
// Stop halts the background monitoring task.
func (m *Monitor) Stop() {
	close(m.stopCh)
}

func (m *Monitor) loop() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	// Initial fetch
	m.collect()

	for {
		select {
		case <-ticker.C:
			m.collect()
		case <-m.stopCh:
			return
		}
	}
}

func (m *Monitor) collect() {
	v, _ := mem.VirtualMemory()
	c, _ := cpu.Percent(0, false)
	
	diskPath := "/"
	if runtime.GOOS == "windows" {
		diskPath = "C:"
	}
	d, _ := disk.Usage(diskPath)
	h, _ := host.Info()

	var cpuPct float64
	if len(c) > 0 {
		cpuPct = c[0]
	}

	uptimeDuration := time.Duration(h.Uptime) * time.Second
	days := uptimeDuration / (24 * time.Hour)
	uptimeDuration -= days * 24 * time.Hour
	hours := uptimeDuration / time.Hour
	uptimeDuration -= hours * time.Hour
	minutes := uptimeDuration / time.Minute

	uptimeStr := fmt.Sprintf("%dD %02dH %02dM", days, hours, minutes)

	throughput := "0.0 MB/s"

	var storageStr string
	usedBytes := float64(d.Used)
	if usedBytes >= 1024*1024*1024*1024 {
		storageStr = fmt.Sprintf("%.2f TB", usedBytes/(1024*1024*1024*1024))
	} else {
		storageStr = fmt.Sprintf("%.1f GB", usedBytes/(1024*1024*1024))
	}

	newStats := models.Stats{
		CPU:        fmt.Sprintf("%.1f%%", cpuPct),
		RAM:        fmt.Sprintf("%.1f GB", float64(v.Used)/1024/1024/1024),
		Storage:    storageStr,
		Throughput: throughput,
		Uptime:     uptimeStr,
		Status:     "OPTIMAL",
	}

	now := time.Now()
	timeStr := now.Format("15:04")

	newPerf := models.PerformanceData{
		Time:  timeStr,
		Speed: int(cpuPct),
		RAM:   float64(v.Used) / 1024 / 1024 / 1024,
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.currentStats = newStats

	m.perfHistory = append(m.perfHistory, newPerf)
	if len(m.perfHistory) > 60 {
		// Keep window size manageable for UI
		m.perfHistory = m.perfHistory[1:]
	}
}

// GetStats returns the latest hardware statistics in a thread-safe manner.
func (m *Monitor) GetStats() models.Stats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.currentStats
}

// GetPerformanceHistory returns the timeline of performance data.
func (m *Monitor) GetPerformanceHistory() []models.PerformanceData {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Return a copy to prevent race conditions during JSON serialization
	copyHistory := make([]models.PerformanceData, len(m.perfHistory))
	copy(copyHistory, m.perfHistory)
	return copyHistory
}

