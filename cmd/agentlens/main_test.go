package main

import "testing"

func TestVersionNeverEmpty(t *testing.T) {
	if version == "" {
		t.Error("version must never be empty (default is dev)")
	}
}
