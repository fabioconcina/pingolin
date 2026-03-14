package prober

import (
	"math"
	"testing"
)

func TestJitterCalculator_SingleSample(t *testing.T) {
	jc := NewJitterCalculator()
	result := jc.Add(10.0)
	if result != nil {
		t.Errorf("expected nil for single sample, got %v", *result)
	}
}

func TestJitterCalculator_TwoSamples(t *testing.T) {
	jc := NewJitterCalculator()
	jc.Add(10.0)
	result := jc.Add(20.0)
	if result == nil {
		t.Fatal("expected non-nil result for two samples")
	}
	if math.Abs(*result-10.0) > 0.001 {
		t.Errorf("expected jitter=10.0, got %f", *result)
	}
}

func TestJitterCalculator_SteadyStream(t *testing.T) {
	jc := NewJitterCalculator()
	for i := 0; i < 10; i++ {
		jc.Add(15.0)
	}
	result := jc.Add(15.0)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if *result != 0.0 {
		t.Errorf("expected jitter=0 for steady stream, got %f", *result)
	}
}

func TestJitterCalculator_WindowSize(t *testing.T) {
	jc := NewJitterCalculator()
	// Fill beyond window size
	for i := 0; i < 30; i++ {
		jc.Add(float64(i))
	}
	if len(jc.samples) != jitterWindowSize {
		t.Errorf("expected window size %d, got %d", jitterWindowSize, len(jc.samples))
	}
}

func TestJitterCalculator_Reset(t *testing.T) {
	jc := NewJitterCalculator()
	jc.Add(10.0)
	jc.Add(20.0)
	jc.Reset()
	result := jc.Add(10.0)
	if result != nil {
		t.Errorf("expected nil after reset, got %v", *result)
	}
}
