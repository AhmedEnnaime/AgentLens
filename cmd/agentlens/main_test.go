package main

import "testing"

func TestVersionOutput(t *testing.T) {
	if version == "" {
		t.Error("version must never be empty (default is \"dev\")")
	}
}
