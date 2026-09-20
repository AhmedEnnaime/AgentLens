package domain

import (
	"encoding/json"
	"fmt"
)

type Provenance uint8

const (
	ProvenanceUnavailable Provenance = iota
	ProvenanceObserved
	ProvenanceDerived
	ProvenanceEstimated
	ProvenanceInferred
)

func (p Provenance) String() string {
	switch p {
	case ProvenanceUnavailable:
		return "unavailable"
	case ProvenanceObserved:
		return "observed"
	case ProvenanceDerived:
		return "derived"
	case ProvenanceEstimated:
		return "estimated"
	case ProvenanceInferred:
		return "inferred"
	default:
		return fmt.Sprintf("provenance(%d)", uint8(p))
	}
}

func (p Provenance) known() bool {
	switch p {
	case ProvenanceUnavailable, ProvenanceObserved, ProvenanceDerived, ProvenanceEstimated, ProvenanceInferred:
		return true
	}
	return false
}

func (p Provenance) MarshalJSON() ([]byte, error) {
	if !p.known() {
		return nil, fmt.Errorf("cannot marshal unknown provenance %d", uint8(p))
	}
	return json.Marshal(p.String())
}

func (p *Provenance) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	for v := ProvenanceUnavailable; v <= ProvenanceInferred; v++ {
		if v.String() == s {
			*p = v
			return nil
		}
	}
	return fmt.Errorf("unknown provenance %q", s)
}
