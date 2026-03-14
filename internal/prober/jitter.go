package prober

import "math"

const jitterWindowSize = 20

// JitterCalculator computes inter-packet delay variation over a sliding window.
type JitterCalculator struct {
	samples []float64
}

func NewJitterCalculator() *JitterCalculator {
	return &JitterCalculator{
		samples: make([]float64, 0, jitterWindowSize),
	}
}

// Add adds an RTT sample and returns the current jitter value.
// Returns nil if there are fewer than 2 samples.
func (j *JitterCalculator) Add(rttMs float64) *float64 {
	j.samples = append(j.samples, rttMs)
	if len(j.samples) > jitterWindowSize {
		j.samples = j.samples[1:]
	}
	if len(j.samples) < 2 {
		return nil
	}

	// Jitter = mean absolute difference between consecutive samples
	var sum float64
	for i := 1; i < len(j.samples); i++ {
		sum += math.Abs(j.samples[i] - j.samples[i-1])
	}
	jitter := sum / float64(len(j.samples)-1)
	return &jitter
}

// Reset clears the jitter window.
func (j *JitterCalculator) Reset() {
	j.samples = j.samples[:0]
}
