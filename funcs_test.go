package flexibleconfig

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExtendedFuncs_keys(t *testing.T) {
	ef := NewExtendedFuncs("/tmp")

	t.Run("keys returns sorted keys", func(t *testing.T) {
		m := map[string]interface{}{
			"zebra": 1,
			"apple": 2,
			"mango": 3,
		}

		keys := ef.keys(m)
		expected := []string{"apple", "mango", "zebra"}
		if len(keys) != len(expected) {
			t.Errorf("keys() returned %d keys, want %d", len(keys), len(expected))
		}
		for i, k := range keys {
			if k != expected[i] {
				t.Errorf("keys()[%d] = %v, want %v", i, k, expected[i])
			}
		}
	})

	t.Run("keys with empty map", func(t *testing.T) {
		m := map[string]interface{}{}
		keys := ef.keys(m)
		if len(keys) != 0 {
			t.Errorf("keys() returned %d keys, want 0", len(keys))
		}
	})
}

func TestExtendedFuncs_index(t *testing.T) {
	ef := NewExtendedFuncs("/tmp")

	t.Run("index returns value for existing key", func(t *testing.T) {
		m := map[string]interface{}{
			"foo": "bar",
			"num": 42,
		}

		val, err := ef.index(m, "foo")
		if err != nil {
			t.Errorf("index() error = %v", err)
		}
		if val != "bar" {
			t.Errorf("index() = %v, want bar", val)
		}

		val, err = ef.index(m, "num")
		if err != nil {
			t.Errorf("index() error = %v", err)
		}
		if val != 42 {
			t.Errorf("index() = %v, want 42", val)
		}
	})

	t.Run("index returns nil for missing key with zero mode", func(t *testing.T) {
		ef.undefinedVars = "zero"
		m := map[string]interface{}{"foo": "bar"}

		val, err := ef.index(m, "missing")
		if err != nil {
			t.Errorf("index() error = %v", err)
		}
		if val != nil {
			t.Errorf("index() = %v, want nil", val)
		}
	})

	t.Run("index returns error for missing key with error mode", func(t *testing.T) {
		ef.undefinedVars = "error"
		m := map[string]interface{}{"foo": "bar"}

		_, err := ef.index(m, "missing")
		if err == nil {
			t.Error("index() should return error for missing key in error mode")
		}
	})

	t.Run("index returns <no value> string for missing key with invalid mode", func(t *testing.T) {
		ef.undefinedVars = "invalid"
		m := map[string]interface{}{"foo": "bar"}

		val, err := ef.index(m, "missing")
		if err != nil {
			t.Errorf("index() error = %v, want nil", err)
		}
		if val != "<no value>" {
			t.Errorf("index() = %v, want <no value>", val)
		}
	})
}

func TestExtendedFuncs_exists(t *testing.T) {
	ef := NewExtendedFuncs(os.TempDir())

	t.Run("exists returns true for existing file", func(t *testing.T) {
		if !ef.exists(os.TempDir()) {
			t.Error("exists() should return true for temp dir")
		}
	})

	t.Run("exists returns false for non-existent file", func(t *testing.T) {
		if ef.exists(filepath.Join(os.TempDir(), "nonexistent_file_path_12345")) {
			t.Error("exists() should return false for non-existent path")
		}
	})
}

func TestExtendedFuncs_merge(t *testing.T) {
	ef := NewExtendedFuncs("/tmp")

	t.Run("merge combines two maps", func(t *testing.T) {
		dest := map[string]interface{}{
			"a": 1,
			"b": 2,
		}
		src := map[string]interface{}{
			"b": 3,
			"c": 4,
		}

		result := ef.merge(dest, src)

		if result["a"] != 1 {
			t.Errorf("merge() a = %v, want 1", result["a"])
		}
		if result["b"] != 3 {
			t.Errorf("merge() b = %v, want 3", result["b"])
		}
		if result["c"] != 4 {
			t.Errorf("merge() c = %v, want 4", result["c"])
		}
	})

	t.Run("merge does not modify original dest", func(t *testing.T) {
		dest := map[string]interface{}{"a": 1}
		src := map[string]interface{}{"b": 2}

		ef.merge(dest, src)

		if dest["b"] != nil {
			t.Error("merge() should not modify original dest")
		}
	})
}

func TestIncludeError(t *testing.T) {
	err := &IncludeError{File: "test.txt", Err: ErrMock}
	expected := `failed to include "test.txt": mock error`
	if err.Error() != expected {
		t.Errorf("IncludeError.Error() = %v, want %v", err.Error(), expected)
	}
	if err.Unwrap() != ErrMock {
		t.Error("Unwrap() did not return wrapped error")
	}
}

var ErrMock = &mockError{}

type mockError struct{}

func (e *mockError) Error() string { return "mock error" }
func (e *mockError) Unwrap() error  { return nil }

func TestFilterBySuffix(t *testing.T) {
	t.Run("filter by single suffix", func(t *testing.T) {
		files := []os.DirEntry{
			&mockDirEntry{name: "test.json"},
			&mockDirEntry{name: "test.yaml"},
			&mockDirEntry{name: "test.toml"},
		}

		filtered := filterBySuffix(files, []string{".json"})
		if len(filtered) != 1 {
			t.Errorf("filterBySuffix() returned %d files, want 1", len(filtered))
		}
	})

	t.Run("filter by multiple suffixes", func(t *testing.T) {
		files := []os.DirEntry{
			&mockDirEntry{name: "test.json"},
			&mockDirEntry{name: "test.yaml"},
			&mockDirEntry{name: "test.toml"},
		}

		filtered := filterBySuffix(files, []string{".json", ".yaml"})
		if len(filtered) != 2 {
			t.Errorf("filterBySuffix() returned %d files, want 2", len(filtered))
		}
	})

	t.Run("empty suffix returns all", func(t *testing.T) {
		files := []os.DirEntry{
			&mockDirEntry{name: "test.json"},
			&mockDirEntry{name: "test.yaml"},
		}

		filtered := filterBySuffix(files, []string{})
		if len(filtered) != 2 {
			t.Errorf("filterBySuffix() returned %d files, want 2", len(filtered))
		}
	})
}

type mockDirEntry struct {
	name string
}

func (m *mockDirEntry) Name() string       { return m.name }
func (m *mockDirEntry) IsDir() bool        { return false }
func (m *mockDirEntry) Type() os.FileMode { return 0 }
func (m *mockDirEntry) Info() (os.FileInfo, error) {
	return nil, nil
}