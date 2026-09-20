package domain

import "fmt"

type Status uint8

const (
	StatusUnset Status = iota
	StatusOk
	StatusError
)

func (s Status) String() string {
	switch s {
	case StatusUnset:
		return "unset"
	case StatusOk:
		return "ok"
	case StatusError:
		return "error"
	default:
		return fmt.Sprintf("status(%d)", uint8(s))
	}
}

func (s Status) known() bool {
	switch s {
	case StatusUnset, StatusOk, StatusError:
		return true
	}
	return false
}
