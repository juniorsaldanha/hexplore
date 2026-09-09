package components

var sparkLevels = []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

// Sparkline renders series as a single line of block characters scaled to
// its own min/max, downsampled (or repeated) to fit width. Hand-rolled
// rather than pulling in a charting library — this is the entire feature.
func Sparkline(series []float64, width int) string {
	if width <= 0 || len(series) == 0 {
		return ""
	}
	buckets := resample(series, width)

	min, max := buckets[0], buckets[0]
	for _, v := range buckets {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}
	span := max - min

	out := make([]rune, len(buckets))
	for i, v := range buckets {
		if span == 0 {
			out[i] = sparkLevels[0]
			continue
		}
		idx := int((v - min) / span * float64(len(sparkLevels)-1))
		out[i] = sparkLevels[idx]
	}
	return string(out)
}

// resample maps series (any length) onto exactly n points by averaging
// contiguous chunks, or repeating points when series is shorter than n.
func resample(series []float64, n int) []float64 {
	if len(series) == n {
		return series
	}
	out := make([]float64, n)
	for i := range n {
		lo := i * len(series) / n
		hi := (i + 1) * len(series) / n
		if hi <= lo {
			hi = lo + 1
		}
		if hi > len(series) {
			hi = len(series)
		}
		var sum float64
		for _, v := range series[lo:hi] {
			sum += v
		}
		out[i] = sum / float64(hi-lo)
	}
	return out
}
