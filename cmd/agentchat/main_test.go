package main

import "testing"

func TestRequireName(t *testing.T) {
	t.Parallel()

	originalName := name
	t.Cleanup(func() {
		name = originalName
	})

	name = ""
	if err := requireName(); err == nil {
		t.Fatal("expected error when name is empty")
	}

	name = "alice"
	if err := requireName(); err != nil {
		t.Fatalf("expected no error when name is set, got %v", err)
	}
}
