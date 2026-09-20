package domain

import "testing"

func TestProvenanceString(t *testing.T) {
	cases := []struct {
		p    Provenance
		want string
	}{
		{ProvenanceUnavailable, "unavailable"},
		{ProvenanceObserved, "observed"},
		{ProvenanceDerived, "derived"},
		{ProvenanceEstimated, "estimated"},
		{ProvenanceInferred, "inferred"},
	}
	for _, c := range cases {
		if got := c.p.String(); got != c.want {
			t.Errorf("Provenance(%d).String() = %q, want %q", uint8(c.p), got, c.want)
		}
	}
}

func TestProvenanceJSONRoundTrip(t *testing.T) {
	for _, p := range []Provenance{
		ProvenanceUnavailable,
		ProvenanceObserved,
		ProvenanceDerived,
		ProvenanceEstimated,
		ProvenanceInferred,
	} {
		data, err := p.MarshalJSON()
		if err != nil {
			t.Fatalf("MarshalJSON(%s): %v", p, err)
		}
		want := `"` + p.String() + `"`
		if string(data) != want {
			t.Errorf("MarshalJSON(%s) = %s, want %s", p, data, want)
		}
		var got Provenance
		if err := got.UnmarshalJSON(data); err != nil {
			t.Fatalf("UnmarshalJSON(%s): %v", data, err)
		}
		if got != p {
			t.Errorf("round-trip: got %s, want %s", got, p)
		}
	}
}

func TestProvenanceUnknownJSON(t *testing.T) {
	var p Provenance
	if err := p.UnmarshalJSON([]byte(`"bogus"`)); err == nil {
		t.Error("UnmarshalJSON of unknown provenance should error")
	}
	if _, err := Provenance(200).MarshalJSON(); err == nil {
		t.Error("MarshalJSON of unknown provenance should error")
	}
}
