package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/fabioconcina/pingolin/internal/store"
)

var timeRanges = []time.Duration{
	1 * time.Hour,
	6 * time.Hour,
	24 * time.Hour,
	7 * 24 * time.Hour,
	30 * 24 * time.Hour,
}

var timeRangeLabels = []string{"1h", "6h", "24h", "7d", "30d"}

type tickMsg time.Time
type splashDoneMsg struct{}

type Model struct {
	store          *store.Store
	targets        []string
	version        string
	timeRangeIdx   int
	width          int
	height         int
	startTime      time.Time
	outageScroll   int
	showDetail     bool
	splash         bool

	// Cached data
	latestPings  map[string]*store.PingResult
	pingData     map[string][]store.PingResult
	avgRTT       map[string]float64
	lossPercent  map[string]float64
	latestDNS    *store.DNSResult
	avgDNS       float64
	latestHTTP   *store.HTTPResult
	outages      []store.Outage
	status       ConnectionStatus
}

func NewModel(s *store.Store, targets []string, defaultTimeRange string, version string) Model {
	idx := 0
	for i, label := range timeRangeLabels {
		if label == defaultTimeRange {
			idx = i
			break
		}
	}
	return Model{
		store:        s,
		targets:      targets,
		version:      version,
		timeRangeIdx: idx,
		startTime:    time.Now(),
		splash:       true,
		latestPings:  make(map[string]*store.PingResult),
		pingData:     make(map[string][]store.PingResult),
		avgRTT:       make(map[string]float64),
		lossPercent:  make(map[string]float64),
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		tickCmd(),
		tea.WindowSize(),
		tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
			return splashDoneMsg{}
		}),
	)
}

func tickCmd() tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case splashDoneMsg:
		m.splash = false
		return m, nil

	case tea.KeyMsg:
		if m.splash {
			m.splash = false
			return m, nil
		}
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "t":
			m.timeRangeIdx = (m.timeRangeIdx + 1) % len(timeRanges)
			m.refreshData()
			return m, nil
		case "d":
			m.showDetail = !m.showDetail
			return m, nil
		case "up":
			if m.outageScroll > 0 {
				m.outageScroll--
			}
			return m, nil
		case "down":
			m.outageScroll++
			return m, nil
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.refreshData()
		return m, nil

	case tickMsg:
		m.refreshData()
		return m, tickCmd()
	}

	return m, nil
}

func (m *Model) refreshData() {
	now := time.Now()
	since := now.Add(-timeRanges[m.timeRangeIdx]).UnixMilli()
	until := now.UnixMilli()

	for _, target := range m.targets {
		if latest, err := m.store.LatestPing(target); err == nil {
			m.latestPings[target] = latest
		}
		if pings, err := m.store.QueryPings(target, since, until); err == nil {
			m.pingData[target] = pings
		}
		if avg, count, lossCount, err := m.store.PingStats(target, since); err == nil && count > 0 {
			m.avgRTT[target] = avg
			m.lossPercent[target] = float64(lossCount) / float64(count) * 100
		}
	}

	if dns, err := m.store.LatestDNS(); err == nil {
		m.latestDNS = dns
	}
	if avg, _, err := m.store.DNSStats(since); err == nil {
		m.avgDNS = avg
	}
	if http, err := m.store.LatestHTTP(); err == nil {
		m.latestHTTP = http
	}
	if outages, err := m.store.RecentOutages(20); err == nil {
		m.outages = outages
	}

	m.status = m.determineStatus()
}

func (m *Model) determineStatus() ConnectionStatus {
	// Check for open outage
	if open, err := m.store.OpenOutage(); err == nil && open != nil {
		return StatusDown
	}

	// Check for degraded conditions
	for _, target := range m.targets {
		if loss, ok := m.lossPercent[target]; ok && loss > 5 {
			return StatusDegraded
		}
		if latest, ok := m.latestPings[target]; ok && latest != nil && latest.PacketLoss {
			return StatusDegraded
		}
		if avg, ok := m.avgRTT[target]; ok {
			if latest, ok := m.latestPings[target]; ok && latest != nil && latest.RTTMs != nil {
				if *latest.RTTMs > 3*avg && avg > 0 {
					return StatusDegraded
				}
			}
		}
	}

	if m.latestDNS != nil && !m.latestDNS.Success {
		return StatusDegraded
	}
	if m.latestHTTP != nil && !m.latestHTTP.Success {
		return StatusDegraded
	}

	return StatusHealthy
}

func (m Model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	if m.splash {
		return renderSplash(m)
	}

	sparkWidth := m.width - 8
	if sparkWidth > 60 {
		sparkWidth = 60
	}
	if sparkWidth < 20 {
		sparkWidth = 20
	}

	var sb strings.Builder

	// Title
	sb.WriteString(titleStyle.Render(" pingolin "))
	sb.WriteString("\n\n")

	// Status + uptime
	uptime := time.Since(m.startTime).Truncate(time.Second)
	sb.WriteString(fmt.Sprintf("  STATUS: %s%suptime: %s\n\n",
		m.status.String(),
		strings.Repeat(" ", max(2, 30-len("STATUS: ● HEALTHY"))),
		formatDuration(uptime),
	))

	// Latency per target
	for _, target := range m.targets {
		lastStr := "--"
		avgStr := "--"
		if latest, ok := m.latestPings[target]; ok && latest != nil && latest.RTTMs != nil {
			lastStr = fmt.Sprintf("%.1fms", *latest.RTTMs)
		}
		if avg, ok := m.avgRTT[target]; ok && avg > 0 {
			avgStr = fmt.Sprintf("%.1fms", avg)
		}

		sb.WriteString(fmt.Sprintf("  %s  last: %s  avg: %s\n",
			labelStyle.Render(fmt.Sprintf("LATENCY (%s)", target)),
			valueStyle.Render(lastStr),
			valueStyle.Render(avgStr),
		))

		// Sparkline
		avg := m.avgRTT[target]
		values := extractRTTs(m.pingData[target])
		sb.WriteString("  ")
		sb.WriteString(RenderSparkline(values, sparkWidth, avg))
		sb.WriteString("\n\n")
	}

	// Packet loss
	currentLoss := 0.0
	avgLoss := 0.0
	totalCount := 0
	for _, target := range m.targets {
		if latest, ok := m.latestPings[target]; ok && latest != nil && latest.PacketLoss {
			currentLoss += 100.0 / float64(len(m.targets))
		}
		if loss, ok := m.lossPercent[target]; ok {
			avgLoss += loss
			totalCount++
		}
	}
	if totalCount > 0 {
		avgLoss /= float64(totalCount)
	}

	sb.WriteString(fmt.Sprintf("  %s  current: %s   %s avg: %s\n",
		labelStyle.Render("PACKET LOSS (all targets)"),
		valueStyle.Render(fmt.Sprintf("%.0f%%", currentLoss)),
		timeRangeLabels[m.timeRangeIdx],
		valueStyle.Render(fmt.Sprintf("%.1f%%", avgLoss)),
	))

	losses := extractLosses(m.targets, m.pingData)
	sb.WriteString("  ")
	sb.WriteString(RenderLossBar(losses, sparkWidth))
	sb.WriteString("\n\n")

	// DNS
	dnsLastStr := "--"
	if m.latestDNS != nil && m.latestDNS.ResolveMs != nil {
		dnsLastStr = fmt.Sprintf("%.0fms", *m.latestDNS.ResolveMs)
	}
	sb.WriteString(fmt.Sprintf("  %s  last: %s    avg: %s\n",
		labelStyle.Render("DNS RESOLUTION"),
		valueStyle.Render(dnsLastStr),
		valueStyle.Render(fmt.Sprintf("%.0fms", m.avgDNS)),
	))

	// HTTP
	httpLastStr := "--"
	httpStatusStr := "--"
	if m.latestHTTP != nil {
		if m.latestHTTP.TotalMs != nil {
			httpLastStr = fmt.Sprintf("%.0fms", *m.latestHTTP.TotalMs)
		}
		if m.latestHTTP.StatusCode != nil {
			httpStatusStr = fmt.Sprintf("%d", *m.latestHTTP.StatusCode)
		}
	}
	sb.WriteString(fmt.Sprintf("  %s  last: %s   status: %s\n",
		labelStyle.Render("HTTP PROBE"),
		valueStyle.Render(httpLastStr),
		valueStyle.Render(httpStatusStr),
	))
	sb.WriteString("\n")

	// Outages
	sb.WriteString(fmt.Sprintf("  %s\n", labelStyle.Render("RECENT OUTAGES")))
	if len(m.outages) == 0 {
		sb.WriteString(fmt.Sprintf("  %s\n", dimStyle.Render("No outages recorded")))
	} else {
		maxShow := 5
		start := m.outageScroll
		if start >= len(m.outages) {
			start = len(m.outages) - 1
		}
		end := start + maxShow
		if end > len(m.outages) {
			end = len(m.outages)
		}

		for i := start; i < end; i++ {
			o := m.outages[i]
			prefix := "├─"
			if i == end-1 {
				prefix = "└─"
			}
			ts := time.UnixMilli(o.StartedAt).Format("2006-01-02 15:04")
			dur := "--"
			if o.DurationMs != nil {
				dur = formatDuration(time.Duration(*o.DurationMs) * time.Millisecond)
			} else {
				dur = "ongoing"
			}
			sb.WriteString(fmt.Sprintf("  %s %s  duration: %s  (%s)\n",
				outageStyle.Render(prefix),
				outageStyle.Render(ts),
				outageStyle.Render(dur),
				outageStyle.Render(o.Cause),
			))
		}
	}
	sb.WriteString("\n")

	// Help
	sb.WriteString(fmt.Sprintf("  %s  time range: %s\n",
		helpStyle.Render("[q]uit  [t]ime range  [d]etail  ↑/↓ scroll"),
		labelStyle.Render(timeRangeLabels[m.timeRangeIdx]),
	))

	return borderStyle.Render(sb.String())
}

func extractRTTs(pings []store.PingResult) []*float64 {
	values := make([]*float64, len(pings))
	for i, p := range pings {
		values[i] = p.RTTMs
	}
	return values
}

func extractLosses(targets []string, pingData map[string][]store.PingResult) []bool {
	// Merge all targets' loss data by timestamp order
	type entry struct {
		ts   int64
		loss bool
	}
	var all []entry
	for _, target := range targets {
		for _, p := range pingData[target] {
			all = append(all, entry{ts: p.Timestamp, loss: p.PacketLoss})
		}
	}
	losses := make([]bool, len(all))
	for i, e := range all {
		losses[i] = e.loss
	}
	return losses
}

func renderSplash(m Model) string {
	logo := splashLogoStyle.Render(
		"█▀█ █ █▄ █ █▀▀ █▀█ █   █ █▄ █\n" +
			"█▀▀ █ █ ▀█ █▄█ █▄█ █▄▄ █ █ ▀█")

	sep := splashSubStyle.Render(strings.Repeat("─", 32))

	subtitle := splashSubStyle.Render("Internet connection monitor  ·  " + m.version)

	content := fmt.Sprintf("%s\n%s\n%s", logo, sep, subtitle)

	return lipgloss.Place(m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		content)
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm %ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
	}
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	return fmt.Sprintf("%dd %dh", days, hours)
}
