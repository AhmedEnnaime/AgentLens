package domain

import (
	"encoding/json"
	"fmt"
)

func (k SpanKind) MarshalJSON() ([]byte, error) {
	if !k.known() {
		return nil, fmt.Errorf("cannot marshal unknown span kind %d", uint8(k))
	}
	return json.Marshal(k.String())
}

func (k *SpanKind) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	for v := KindSession; v <= KindToolCall; v++ {
		if v.String() == s {
			*k = v
			return nil
		}
	}
	return fmt.Errorf("unknown span kind %q", s)
}

func (k EventKind) MarshalJSON() ([]byte, error) {
	if !k.known() {
		return nil, fmt.Errorf("cannot marshal unknown event kind %d", uint8(k))
	}
	return json.Marshal(k.String())
}

func (k *EventKind) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	for v := EventFileEdit; v <= EventTaskMarker; v++ {
		if v.String() == s {
			*k = v
			return nil
		}
	}
	return fmt.Errorf("unknown event kind %q", s)
}

func (s Status) MarshalJSON() ([]byte, error) {
	if !s.known() {
		return nil, fmt.Errorf("cannot marshal unknown status %d", uint8(s))
	}
	return json.Marshal(s.String())
}

func (s *Status) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}
	for v := StatusUnset; v <= StatusError; v++ {
		if v.String() == str {
			*s = v
			return nil
		}
	}
	return fmt.Errorf("unknown status %q", str)
}
