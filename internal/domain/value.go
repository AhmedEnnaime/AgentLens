package domain

import (
	"encoding/json"
	"fmt"
)

type Value[T any] struct {
	V          T
	Provenance Provenance
	SourceRef  string
}

func Observed[T any](v T, ref string) Value[T] {
	return Value[T]{V: v, Provenance: ProvenanceObserved, SourceRef: ref}
}

func Derived[T any](v T) Value[T] {
	return Value[T]{V: v, Provenance: ProvenanceDerived}
}

func Estimated[T any](v T, ref string) Value[T] {
	return Value[T]{V: v, Provenance: ProvenanceEstimated, SourceRef: ref}
}

func Inferred[T any](v T, ref string) Value[T] {
	return Value[T]{V: v, Provenance: ProvenanceInferred, SourceRef: ref}
}

func Unavailable[T any]() Value[T] {
	return Value[T]{Provenance: ProvenanceUnavailable}
}

func (v Value[T]) Validate() error {
	if !v.Provenance.known() {
		return fmt.Errorf("value: unknown provenance %d", uint8(v.Provenance))
	}
	switch v.Provenance {
	case ProvenanceObserved, ProvenanceEstimated, ProvenanceInferred:
		if v.SourceRef == "" {
			return fmt.Errorf("value: %s provenance requires non-empty source ref", v.Provenance)
		}
	case ProvenanceUnavailable:
		if v.SourceRef != "" {
			return fmt.Errorf("value: unavailable provenance must have empty source ref")
		}
	}
	return nil
}

func (v Value[T]) MarshalJSON() ([]byte, error) {
	value := any(v.V)
	if v.Provenance == ProvenanceUnavailable {
		value = nil
	}
	return json.Marshal(struct {
		Value      any    `json:"value"`
		Provenance string `json:"provenance"`
		SourceRef  string `json:"source_ref"`
	}{
		Value:      value,
		Provenance: v.Provenance.String(),
		SourceRef:  v.SourceRef,
	})
}

func (v *Value[T]) UnmarshalJSON(data []byte) error {
	var raw struct {
		Value      json.RawMessage `json:"value"`
		Provenance string          `json:"provenance"`
		SourceRef  string          `json:"source_ref"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	var prov Provenance
	if err := json.Unmarshal([]byte(fmt.Sprintf("%q", raw.Provenance)), &prov); err != nil {
		return err
	}
	v.Provenance = prov
	v.SourceRef = raw.SourceRef
	if prov == ProvenanceUnavailable {
		return nil
	}
	if err := json.Unmarshal(raw.Value, &v.V); err != nil {
		return err
	}
	return nil
}
