package domain

import (
	"encoding/json"
	"testing"
)

func TestZeroValueIsUnavailable(t *testing.T) {
	var v Value[int64]
	if v.Provenance != ProvenanceUnavailable {
		t.Errorf("zero Value provenance = %s, want unavailable", v.Provenance)
	}
	if err := v.Validate(); err != nil {
		t.Errorf("zero Value should be valid: %v", err)
	}
}

func TestConstructors(t *testing.T) {
	if v := Observed(5, "r1"); v.V != 5 || v.Provenance != ProvenanceObserved || v.SourceRef != "r1" {
		t.Errorf("Observed = %+v, want V=5 observed ref=r1", v)
	}
	if v := Derived(7); v.V != 7 || v.Provenance != ProvenanceDerived || v.SourceRef != "" {
		t.Errorf("Derived = %+v, want V=7 derived empty ref", v)
	}
	if v := Estimated(2.5, "pricing-v1"); v.Provenance != ProvenanceEstimated || v.SourceRef != "pricing-v1" {
		t.Errorf("Estimated = %+v, want estimated ref=pricing-v1", v)
	}
	if v := Inferred(9, "run-42"); v.Provenance != ProvenanceInferred || v.SourceRef != "run-42" {
		t.Errorf("Inferred = %+v, want inferred ref=run-42", v)
	}
	if v := Unavailable[int64](); v.Provenance != ProvenanceUnavailable || v.SourceRef != "" {
		t.Errorf("Unavailable = %+v, want unavailable empty ref", v)
	}
}

func TestValidateSourceRefRules(t *testing.T) {
	cases := []struct {
		name string
		v    Value[int]
		want bool
	}{
		{"observed empty ref", Observed(1, ""), false},
		{"observed with ref", Observed(1, "r"), true},
		{"derived empty ref", Derived(1), true},
		{"estimated empty ref", Estimated(1, ""), false},
		{"estimated with ref", Estimated(1, "r"), true},
		{"inferred empty ref", Inferred(1, ""), false},
		{"inferred with ref", Inferred(1, "r"), true},
		{"unavailable empty ref", Unavailable[int](), true},
		{"unavailable with ref", Value[int]{Provenance: ProvenanceUnavailable, SourceRef: "r"}, false},
		{"unknown provenance", Value[int]{Provenance: 200}, false},
	}
	for _, c := range cases {
		err := c.v.Validate()
		if (err == nil) != c.want {
			t.Errorf("%s: Validate() = %v, want valid=%v", c.name, err, c.want)
		}
	}
}

func TestValueJSONShape(t *testing.T) {
	data, err := json.Marshal(Observed(42, "src-1"))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := m["provenance"]; !ok {
		t.Errorf("provenance key missing from %s", data)
	}
	if _, ok := m["source_ref"]; !ok {
		t.Errorf("source_ref key missing from %s", data)
	}
	if _, ok := m["value"]; !ok {
		t.Errorf("value key missing from %s", data)
	}
	want := map[string]any{"value": float64(42), "provenance": "observed", "source_ref": "src-1"}
	for k, wv := range want {
		if m[k] != wv {
			t.Errorf("key %q = %v, want %v", k, m[k], wv)
		}
	}
}

func TestUnavailableJSONNullValue(t *testing.T) {
	data, err := json.Marshal(Unavailable[int64]())
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if v, ok := m["value"]; !ok || v != nil {
		t.Errorf("unavailable value = %v (present=%v), want null", v, ok)
	}
	if m["provenance"] != "unavailable" {
		t.Errorf("unavailable provenance = %v, want unavailable", m["provenance"])
	}
}

func TestValueJSONRoundTrip(t *testing.T) {
	orig := Observed(int64(12345), "ref-x")
	data, err := json.Marshal(orig)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got Value[int64]
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got != orig {
		t.Errorf("round-trip: got %+v, want %+v", got, orig)
	}

	unav := Unavailable[string]()
	data, err = json.Marshal(unav)
	if err != nil {
		t.Fatalf("marshal unavailable: %v", err)
	}
	var gotU Value[string]
	if err := json.Unmarshal(data, &gotU); err != nil {
		t.Fatalf("unmarshal unavailable: %v", err)
	}
	if gotU.Provenance != ProvenanceUnavailable {
		t.Errorf("unavailable round-trip provenance = %s, want unavailable", gotU.Provenance)
	}
}
