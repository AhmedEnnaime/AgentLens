package domain

import "time"

func FromEpochMillis(ms int64) time.Time {
	return time.Unix(0, ms*int64(time.Millisecond)).UTC()
}

func ToEpochMillis(t time.Time) int64 {
	return t.UnixNano() / int64(time.Millisecond)
}
