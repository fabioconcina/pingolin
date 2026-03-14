package tui

import (
	"math"
	"strings"
)

var sparkBlocks = []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

// RenderSparkline converts a slice of values into a sparkline string.
// nil values are rendered as gaps (space). avgBaseline is used for color thresholds.
func RenderSparkline(values []*float64, width int, avgBaseline float64) string {
	if len(values) == 0 {
		return strings.Repeat(" ", width)
	}

	// Take last `width` values
	if len(values) > width {
		values = values[len(values)-width:]
	}

	// Find min/max of non-nil values
	minVal := math.MaxFloat64
	maxVal := -math.MaxFloat64
	for _, v := range values {
		if v != nil {
			if *v < minVal {
				minVal = *v
			}
			if *v > maxVal {
				maxVal = *v
			}
		}
	}

	if minVal == math.MaxFloat64 {
		return strings.Repeat(" ", width)
	}

	var sb strings.Builder
	spread := maxVal - minVal
	if spread == 0 {
		spread = 1
	}

	for _, v := range values {
		if v == nil {
			sb.WriteString(sparkGray.Render(" "))
			continue
		}

		// Map to 0-7 index
		idx := int((*v - minVal) / spread * 7)
		if idx > 7 {
			idx = 7
		}
		if idx < 0 {
			idx = 0
		}

		ch := string(sparkBlocks[idx])

		// Color based on baseline
		switch {
		case avgBaseline > 0 && *v > 5*avgBaseline:
			sb.WriteString(sparkRed.Render(ch))
		case avgBaseline > 0 && *v > 2*avgBaseline:
			sb.WriteString(sparkYellow.Render(ch))
		default:
			sb.WriteString(sparkGreen.Render(ch))
		}
	}

	// Pad if shorter than width
	for sb.Len() < width {
		sb.WriteString(" ")
	}

	return sb.String()
}

// RenderLossBar renders a packet loss bar (filled = loss).
func RenderLossBar(losses []bool, width int) string {
	if len(losses) == 0 {
		return strings.Repeat("░", width)
	}

	if len(losses) > width {
		losses = losses[len(losses)-width:]
	}

	var sb strings.Builder
	for _, lost := range losses {
		if lost {
			sb.WriteString(sparkRed.Render("█"))
		} else {
			sb.WriteString(sparkGreen.Render("░"))
		}
	}

	for sb.Len() < width {
		sb.WriteString("░")
	}

	return sb.String()
}
