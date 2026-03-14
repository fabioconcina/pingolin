package tui

type ConnectionStatus int

const (
	StatusHealthy ConnectionStatus = iota
	StatusDegraded
	StatusDown
)

func (s ConnectionStatus) String() string {
	switch s {
	case StatusHealthy:
		return statusHealthy.Render("● HEALTHY")
	case StatusDegraded:
		return statusDegraded.Render("● DEGRADED")
	case StatusDown:
		return statusDown.Render("● DOWN")
	default:
		return "● UNKNOWN"
	}
}
