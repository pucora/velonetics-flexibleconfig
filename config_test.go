package flexibleconfig

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFlexibleConfig(t *testing.T) {
	t.Run("load from explicit path", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "flexible_config.json")

		fc := FlexibleConfig{
			Settings: SettingsConfig{
				Paths: []string{"./settings"},
			},
			Partials: PathsConfig{
				Paths: []string{"./partials"},
			},
		}
		data, _ := json.Marshal(fc)
		os.WriteFile(configPath, data, 0644)

		loaded, baseDir, err := LoadFlexibleConfig(configPath)
		if err != nil {
			t.Fatalf("LoadFlexibleConfig() error = %v", err)
		}
		if loaded == nil {
			t.Fatal("LoadFlexibleConfig() returned nil")
		}
		if len(loaded.Settings.Paths) != 1 || loaded.Settings.Paths[0] != "./settings" {
			t.Errorf("Settings paths = %v, want [./settings]", loaded.Settings.Paths)
		}
		if baseDir != tmpDir {
			t.Errorf("BaseDir = %v, want %v", baseDir, tmpDir)
		}
	})

	t.Run("auto-detect flexible_config.json", func(t *testing.T) {
		tmpDir := t.TempDir()
		oldCwd, _ := os.Getwd()
		defer os.Chdir(oldCwd)
		os.Chdir(tmpDir)

		fc := FlexibleConfig{
			Settings: SettingsConfig{
				Paths: []string{"./settings"},
			},
		}
		data, _ := json.Marshal(fc)
		os.WriteFile(filepath.Join(tmpDir, "flexible_config.json"), data, 0644)

		loaded, _, err := LoadFlexibleConfig("")
		if err != nil {
			t.Fatalf("LoadFlexibleConfig() error = %v", err)
		}
		if loaded == nil {
			t.Fatal("LoadFlexibleConfig() returned nil for auto-detect")
		}
	})

	t.Run("no config file returns nil", func(t *testing.T) {
		tmpDir := t.TempDir()
		oldCwd, _ := os.Getwd()
		defer os.Chdir(oldCwd)
		os.Chdir(tmpDir)

		loaded, baseDir, err := LoadFlexibleConfig("")
		if err != nil {
			t.Fatalf("LoadFlexibleConfig() error = %v", err)
		}
		if loaded != nil {
			t.Error("LoadFlexibleConfig() should return nil when no config exists")
		}
		if baseDir != "" {
			t.Errorf("BaseDir = %v, want empty string", baseDir)
		}
	})

	t.Run("invalid JSON returns error", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "flexible_config.json")
		os.WriteFile(configPath, []byte("invalid json"), 0644)

		_, _, err := LoadFlexibleConfig(configPath)
		if err == nil {
			t.Error("LoadFlexibleConfig() should return error for invalid JSON")
		}
	})
}

func TestFlexibleConfig_IsEnabled(t *testing.T) {
	t.Run("nil config checks env var", func(t *testing.T) {
		os.Setenv(EnvFCEnable, "1")
		defer os.Unsetenv(EnvFCEnable)

		fc := (*FlexibleConfig)(nil)
		if !fc.IsEnabled() {
			t.Error("IsEnabled() should return true when FC_ENABLE is set")
		}
	})

	t.Run("non-nil config returns true", func(t *testing.T) {
		os.Unsetenv(EnvFCEnable)
		fc := &FlexibleConfig{}
		if !fc.IsEnabled() {
			t.Error("IsEnabled() should return true for non-nil config")
		}
	})
}

func TestFlexibleConfig_GetSettingsPaths(t *testing.T) {
	t.Run("resolves relative paths", func(t *testing.T) {
		tmpDir := t.TempDir()
		fc := &FlexibleConfig{
			Settings: SettingsConfig{
				Paths: []string{"./settings"},
			},
		}

		paths := fc.GetSettingsPaths(tmpDir)
		if len(paths) != 1 {
			t.Fatalf("GetSettingsPaths() returned %d paths, want 1", len(paths))
		}
		expected := filepath.Join(tmpDir, "settings")
		if paths[0] != expected {
			t.Errorf("GetSettingsPaths()[0] = %v, want %v", paths[0], expected)
		}
	})

	t.Run("falls back to env var when no paths", func(t *testing.T) {
		tmpDir := t.TempDir()
		os.Setenv(EnvFCSettings, tmpDir)
		defer os.Unsetenv(EnvFCSettings)

		fc := &FlexibleConfig{}
		paths := fc.GetSettingsPaths("/some/base")
		if len(paths) != 1 || paths[0] != tmpDir {
			t.Errorf("GetSettingsPaths() = %v, want [%v]", paths, tmpDir)
		}
	})
}

func TestConfigError(t *testing.T) {
	t.Run("error with wrapped error", func(t *testing.T) {
		err := &ConfigError{Message: "test error", Err: os.ErrNotExist}
		expected := "test error: file does not exist"
		if err.Error() != expected {
			t.Errorf("ConfigError.Error() = %v, want %v", err.Error(), expected)
		}
	})

	t.Run("error without wrapped error", func(t *testing.T) {
		err := &ConfigError{Message: "simple error"}
		expected := "simple error"
		if err.Error() != expected {
			t.Errorf("ConfigError.Error() = %v, want %v", err.Error(), expected)
		}
	})

	t.Run("unwrap", func(t *testing.T) {
		err := &ConfigError{Message: "test", Err: os.ErrNotExist}
		if err.Unwrap() != os.ErrNotExist {
			t.Error("Unwrap() did not return wrapped error")
		}
	})
}