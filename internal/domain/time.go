package domain

import "time"

func FromEpochMillis(ms int64) time.Time {
	return time.Unix(ms/1000, (ms%1000)*int64(time.Millisecond)).UTC()
}

func ToEpochMillis(t time.Time) int64 {
	return t.UnixNano() / int64(time.Millisecond)
}
