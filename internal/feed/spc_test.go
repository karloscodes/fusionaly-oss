package feed

import (
	"testing"
	"time"
)

func TestSPCIsSpike(t *testing.T) {
	t.Run("returns critical when above 3 sigma", func(t *testing.T) {
		isSpike, severity := SPCIsSpike(10.0, 2.0, 2.0) // z = (10-2)/2 = 4

		if !isSpike {
			t.Error("expected spike to be detected")
		}
		if severity != "critical" {
			t.Errorf("expected critical, got %s", severity)
		}
	})

	t.Run("returns warning when between 2 and 3 sigma", func(t *testing.T) {
		isSpike, severity := SPCIsSpike(6.5, 2.0, 2.0) // z = (6.5-2)/2 = 2.25

		if !isSpike {
			t.Error("expected spike to be detected")
		}
		if severity != "warning" {
			t.Errorf("expected warning, got %s", severity)
		}
	})

	t.Run("returns false when within normal range", func(t *testing.T) {
		isSpike, _ := SPCIsSpike(3.0, 2.0, 2.0) // z = (3-2)/2 = 0.5

		if isSpike {
			t.Error("expected no spike")
		}
	})

	t.Run("handles zero stddev", func(t *testing.T) {
		isSpike, severity := SPCIsSpike(5.0, 2.0, 0.0) // Should use 1.0

		if !isSpike {
			t.Error("expected spike with zero stddev fallback")
		}
		if severity != "critical" {
			t.Errorf("expected critical, got %s", severity)
		}
	})
}

func TestHourOfWeek(t *testing.T) {
	t.Run("monday midnight is 0", func(t *testing.T) {
		// 2026-02-09 is a Monday
		monday := time.Date(2026, 2, 9, 0, 0, 0, 0, time.UTC)

		if got := HourOfWeek(monday); got != 0 {
			t.Errorf("expected 0, got %d", got)
		}
	})

	t.Run("monday 12:00 is 12", func(t *testing.T) {
		monday := time.Date(2026, 2, 9, 12, 0, 0, 0, time.UTC)

		if got := HourOfWeek(monday); got != 12 {
			t.Errorf("expected 12, got %d", got)
		}
	})

	t.Run("tuesday midnight is 24", func(t *testing.T) {
		tuesday := time.Date(2026, 2, 10, 0, 0, 0, 0, time.UTC)

		if got := HourOfWeek(tuesday); got != 24 {
			t.Errorf("expected 24, got %d", got)
		}
	})

	t.Run("sunday 23:00 is 167", func(t *testing.T) {
		// 2026-02-15 is a Sunday
		sunday := time.Date(2026, 2, 15, 23, 0, 0, 0, time.UTC)

		if got := HourOfWeek(sunday); got != 167 {
			t.Errorf("expected 167, got %d", got)
		}
	})

	t.Run("saturday noon is 156", func(t *testing.T) {
		// Saturday = day 5 (0-indexed from Monday), hour 12
		// 5 * 24 + 12 = 132
		saturday := time.Date(2026, 2, 14, 12, 0, 0, 0, time.UTC)

		if got := HourOfWeek(saturday); got != 132 {
			t.Errorf("expected 132, got %d", got)
		}
	})
}

func TestZScore(t *testing.T) {
	t.Run("calculates positive z-score", func(t *testing.T) {
		z := ZScore(150.0, 100.0, 25.0) // (150-100)/25 = 2

		if z != 2.0 {
			t.Errorf("expected 2.0, got %f", z)
		}
	})

	t.Run("calculates negative z-score", func(t *testing.T) {
		z := ZScore(50.0, 100.0, 25.0) // (50-100)/25 = -2

		if z != -2.0 {
			t.Errorf("expected -2.0, got %f", z)
		}
	})

	t.Run("handles zero stddev", func(t *testing.T) {
		z := ZScore(5.0, 2.0, 0.0) // Should use stddev=1

		if z != 3.0 {
			t.Errorf("expected 3.0, got %f", z)
		}
	})
}

// =============================================================================
// SCENARIO TESTS: Cold Start vs Learned Baseline
// =============================================================================

func TestSPC_ColdStartScenario(t *testing.T) {
	// Scenario: New website with insufficient data
	// System should use defaults (mean=100, stddev=25)
	// This calibration matches original 50% spike/drop thresholds

	t.Run("uses conservative defaults during cold start", func(t *testing.T) {
		baseline := &FeedBaseline{
			Mean:        500.0, // Website's actual mean, but we don't trust it yet
			StdDev:      50.0,
			SampleCount: 5, // Below MinSamplesForBaseline (20)
		}

		mean, stddev := GetEffectiveBaseline(baseline)

		if mean != DefaultMean {
			t.Errorf("cold start should use default mean %f, got %f", DefaultMean, mean)
		}
		if stddev != DefaultStdDev {
			t.Errorf("cold start should use default stddev %f, got %f", DefaultStdDev, stddev)
		}
	})

	t.Run("200 visitors triggers spike during cold start", func(t *testing.T) {
		// With defaults (mean=100, stddev=25), 200 gives z-score = (200-100)/25 = 4
		// This should be critical (> 3 sigma)
		baseline := &FeedBaseline{SampleCount: 5}
		mean, stddev := GetEffectiveBaseline(baseline)

		isSpike, severity := SPCIsSpike(200.0, mean, stddev)

		if !isSpike || severity != "critical" {
			t.Errorf("200 visitors during cold start should be critical, got spike=%v severity=%s", isSpike, severity)
		}
	})

	t.Run("140 visitors is within normal range during cold start", func(t *testing.T) {
		// With defaults (mean=100, stddev=25), 140 gives z-score = (140-100)/25 = 1.6
		// This is below warning threshold (2 sigma)
		baseline := &FeedBaseline{SampleCount: 5}
		mean, stddev := GetEffectiveBaseline(baseline)

		isSpike, _ := SPCIsSpike(140.0, mean, stddev)

		if isSpike {
			t.Error("140 visitors during cold start should NOT trigger spike (z=1.6 < 2)")
		}
	})
}

func TestSPC_LearnedBaselineScenario(t *testing.T) {
	// Scenario: Website with enough data to trust baseline
	// Website normally gets 500 visitors ± 50

	t.Run("uses learned values after enough samples", func(t *testing.T) {
		baseline := &FeedBaseline{
			Mean:        500.0,
			StdDev:      50.0,
			SampleCount: 30,
		}

		mean, stddev := GetEffectiveBaseline(baseline)

		if mean != 500.0 {
			t.Errorf("should use learned mean 500.0, got %f", mean)
		}
		if stddev != 50.0 {
			t.Errorf("should use learned stddev 50.0, got %f", stddev)
		}
	})

	t.Run("650 visitors is critical for stable website", func(t *testing.T) {
		// Website normally at 500 ± 50
		// 650 gives z-score = (650-500)/50 = 3, at critical threshold
		baseline := &FeedBaseline{
			Mean:        500.0,
			StdDev:      50.0,
			SampleCount: 30,
		}
		mean, stddev := GetEffectiveBaseline(baseline)

		isSpike, severity := SPCIsSpike(651.0, mean, stddev)

		if !isSpike || severity != "critical" {
			t.Errorf("651 visitors for 500±50 website should be critical, got spike=%v severity=%s", isSpike, severity)
		}
	})

	t.Run("600 visitors is warning for stable website", func(t *testing.T) {
		// Website normally at 500 ± 50
		// 600 gives z-score = (600-500)/50 = 2, at warning threshold
		baseline := &FeedBaseline{
			Mean:        500.0,
			StdDev:      50.0,
			SampleCount: 30,
		}
		mean, stddev := GetEffectiveBaseline(baseline)

		isSpike, severity := SPCIsSpike(601.0, mean, stddev)

		if !isSpike || severity != "warning" {
			t.Errorf("601 visitors for 500±50 website should be warning, got spike=%v severity=%s", isSpike, severity)
		}
	})

	t.Run("550 visitors is normal for stable website", func(t *testing.T) {
		// Website normally at 500 ± 50
		// 550 gives z-score = (550-500)/50 = 1, within normal range
		baseline := &FeedBaseline{
			Mean:        500.0,
			StdDev:      50.0,
			SampleCount: 30,
		}
		mean, stddev := GetEffectiveBaseline(baseline)

		isSpike, _ := SPCIsSpike(550.0, mean, stddev)

		if isSpike {
			t.Error("550 visitors for 500±50 website should be normal (z=1)")
		}
	})
}

func TestSPC_HighVarianceWebsite(t *testing.T) {
	// Scenario: Website with naturally high variance
	// E-commerce site with mean 1000 and stddev 300

	t.Run("1500 visitors is normal for high-variance site", func(t *testing.T) {
		// 1000 ± 300, so 1500 is z-score = (1500-1000)/300 = 1.67
		baseline := &FeedBaseline{
			Mean:        1000.0,
			StdDev:      300.0,
			SampleCount: 50,
		}
		mean, stddev := GetEffectiveBaseline(baseline)

		isSpike, _ := SPCIsSpike(1500.0, mean, stddev)

		if isSpike {
			t.Error("1500 visitors for 1000±300 site should be normal (z<2)")
		}
	})

	t.Run("2000 visitors triggers critical for high-variance site", func(t *testing.T) {
		// 1000 ± 300, so 2000 is z-score = (2000-1000)/300 = 3.33, critical
		baseline := &FeedBaseline{
			Mean:        1000.0,
			StdDev:      300.0,
			SampleCount: 50,
		}
		mean, stddev := GetEffectiveBaseline(baseline)

		isSpike, severity := SPCIsSpike(2000.0, mean, stddev)

		if !isSpike || severity != "critical" {
			t.Errorf("2000 visitors for 1000±300 site should be critical, got spike=%v severity=%s", isSpike, severity)
		}
	})
}
