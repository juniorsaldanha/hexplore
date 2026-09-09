package views

import (
	"fmt"
	"math"
	"strconv"
	"time"
)

func formatAge(d time.Duration) string {
	if d <= 0 {
		return "0s"
	}
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
	default:
		days := int(d.Hours()) / 24
		return fmt.Sprintf("%dd%dh", days, int(d.Hours())%24)
	}
}

// formatInt adds thousands separators — every count on the dashboard is
// large enough that reading digit groups matters.
func formatInt(n int) string {
	s := strconv.Itoa(n)
	neg := ""
	if len(s) > 0 && s[0] == '-' {
		neg, s = "-", s[1:]
	}
	if len(s) <= 3 {
		return neg + s
	}
	var out []byte
	for i, c := range []byte(s) {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, c)
	}
	return neg + string(out)
}

func formatFloat(f float64, decimals int) string {
	return strconv.FormatFloat(f, 'f', decimals, 64)
}

// formatCompact renders a large number with a K/M/B/T suffix — difficulty
// numbers run into the hundreds of trillions and are unreadable in full.
func formatCompact(f float64) string {
	abs := math.Abs(f)
	switch {
	case abs >= 1e12:
		return formatFloat(f/1e12, 2) + "T"
	case abs >= 1e9:
		return formatFloat(f/1e9, 2) + "B"
	case abs >= 1e6:
		return formatFloat(f/1e6, 2) + "M"
	case abs >= 1e3:
		return formatFloat(f/1e3, 2) + "K"
	default:
		return formatFloat(f, 2)
	}
}

func minMax(series []float64) (lo, hi float64) {
	if len(series) == 0 {
		return 0, 0
	}
	lo, hi = series[0], series[0]
	for _, v := range series {
		if v < lo {
			lo = v
		}
		if v > hi {
			hi = v
		}
	}
	return lo, hi
}
