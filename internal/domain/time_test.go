package domain

import (
	"testing"
	"time"
)

func TestEpochMillisRoundTrip(t *testing.T) {
	values := []int64{0, 1, 1000, 1700000000000, 1712345678901, -1, -1700000000000}
	for _, ms := range values {
		tm := FromEpochMillis(ms)
		if got := ToEpochMillis(tm); got != ms {
			t.Errorf("round-trip %d: got %d", ms, got)
		}
	}
}

func TestFromEpochMillisUTC(t *testing.T) {
	tm := FromEpochMillis(1700000000123)
	if tm.Location() != time.UTC {
		t.Errorf("location = %v, want UTC", tm.Location())
	}
	if tm.Nanosecond()%int(time.Millisecond) != 0 {
		t.Errorf("ms precision not preserved: ns=%d", tm.Nanosecond())
	}
}

func TestFromEpochMillisPrecision(t *testing.T) {
	tm := FromEpochMillis(1700000000123)
	if tm.UnixMilli() != 1700000000123 {
		t.Errorf("UnixMilli = %d, want 1700000000123", tm.UnixMilli())
	}
}
