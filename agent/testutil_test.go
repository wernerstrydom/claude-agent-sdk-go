package agent

import (
	"io"
	"os"
	"testing"
)

// mustClose closes an io.Closer and fails the test if an error occurs.
// This is safe to use in defer statements.
func mustClose(t *testing.T, c io.Closer) {
	t.Helper()
	if err := c.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
}

// mustWriteFile writes a file and fails the test if an error occurs.
// The permission argument requires 0755 for executable scripts in tests.
//
//nolint:gosec // G306: Test scripts need executable permissions
func mustWriteFile(t *testing.T, name string, data []byte, perm os.FileMode) {
	t.Helper()
	if err := os.WriteFile(name, data, perm); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", name, err)
	}
}

//nolint:unused // Keep for future test use
func mustMkdir(t *testing.T, name string, perm os.FileMode) {
	t.Helper()
	if err := os.Mkdir(name, perm); err != nil { //nolint:gosec // G301: Test directories may need specific permissions
		t.Fatalf("Mkdir(%q) error = %v", name, err)
	}
}

// mustMkdirAll creates a directory and all parents, failing the test if an error occurs.
//
//nolint:gosec // G301: Test directories may need specific permissions
func mustMkdirAll(t *testing.T, name string, perm os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(name, perm); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", name, err)
	}
}

//nolint:unused // Keep for future test use
func mustRemoveAll(t *testing.T, path string) {
	t.Helper()
	if err := os.RemoveAll(path); err != nil {
		t.Errorf("RemoveAll(%q) error = %v", path, err)
	}
}

// mustReadFile reads a file and fails the test if an error occurs.
//
//nolint:gosec // G304: Test file paths are controlled by tests
func mustReadFile(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", name, err)
	}
	return data
}
