package flexibleconfig

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
)

type FlexibleConfig struct {
	Debug          bool              `json:"debug,omitempty"`
	MetaKey        string            `json:"meta_key,omitempty"`
	KeysNamingRules string           `json:"keys_naming_rules,omitempty"`
	Settings       SettingsConfig    `json:"settings,omitempty"`
	Partials       PathsConfig        `json:"partials,omitempty"`
	Templates      TemplatesConfig    `json:"templates,omitempty"`
	AllowedSuffixes []string        `json:"allowed_suffixes,omitempty"`
	AllowOverwrite  bool             `json:"allow_overwrite,omitempty"`
	DirFieldPrefix  string           `json:"dir_field_prefix,omitempty"`
	RefKey         string            `json:"ref_key,omitempty"`
	UndefinedVars  string            `json:"undefined_vars,omitempty"`
	Out            string            `json:"out,omitempty"`
}

type SettingsConfig struct {
	Paths            []string `json:"paths,omitempty"`
	AllowedSuffixes  []string `json:"allowed_suffixes,omitempty"`
	AllowOverwrite   bool     `json:"allow_overwrite,omitempty"`
	DirFieldPrefix   string   `json:"dir_field_prefix,omitempty"`
}

type PathsConfig struct {
	Paths  []string `json:"paths,omitempty"`
}

type TemplatesConfig struct {
	Paths         []string `json:"paths,omitempty"`
	UndefinedVars string   `json:"undefined_vars,omitempty"`
}

const (
	EnvFCConfig     = "FC_CONFIG"
	EnvFCEnable     = "FC_ENABLE"
	EnvFCSettings   = "FC_SETTINGS"
	EnvFCPartials   = "FC_PARTIALS"
	EnvFCTemplates  = "FC_TEMPLATES"
	EnvFCDebug      = "FC_DEBUG"
)

var strictKeyRegex = regexp.MustCompile(`^[a-zA-Z0-9_]+$`)

func LoadFlexibleConfig(configPath string) (*FlexibleConfig, string, error) {
	if configPath == "" {
		configPath = os.Getenv(EnvFCConfig)
	}

	if configPath == "" {
		autoPath := filepath.Join("flexible_config.json")
		if _, err := os.Stat(autoPath); err == nil {
			configPath = autoPath
		}
	}

	if configPath == "" {
		return nil, "", nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, "", &ConfigError{Message: "failed to read flexible_config.json", Err: err}
	}

	var fc FlexibleConfig
	if err := json.Unmarshal(data, &fc); err != nil {
		return nil, "", &ConfigError{Message: "failed to parse flexible_config.json", Err: err}
	}

	baseDir := filepath.Dir(configPath)
	if baseDir == "." {
		baseDir, _ = os.Getwd()
	}

	return &fc, baseDir, nil
}

func (fc *FlexibleConfig) IsEnabled() bool {
	if fc == nil {
		return os.Getenv(EnvFCEnable) != ""
	}
	return true
}

func (fc *FlexibleConfig) IsDebug() bool {
	if fc == nil {
		return os.Getenv(EnvFCDebug) == "true"
	}
	return fc.Debug
}

func (fc *FlexibleConfig) GetMetaKey() string {
	if fc != nil && fc.MetaKey != "" {
		return fc.MetaKey
	}
	return "meta"
}

func (fc *FlexibleConfig) GetKeysNamingRules() string {
	if fc != nil && fc.KeysNamingRules != "" {
		return fc.KeysNamingRules
	}
	return "freeform"
}

func (fc *FlexibleConfig) GetUndefinedVars() string {
	if fc != nil && fc.UndefinedVars != "" {
		return fc.UndefinedVars
	}
	if fc != nil && fc.Templates.UndefinedVars != "" {
		return fc.Templates.UndefinedVars
	}
	return "error"
}

func (fc *FlexibleConfig) GetSettingsPaths(baseDir string) []string {
	var paths []string

	if fc != nil {
		for _, p := range fc.Settings.Paths {
			if p == "" {
				continue
			}
			if !filepath.IsAbs(p) {
				p = filepath.Join(baseDir, p)
			}
			paths = append(paths, p)
		}
	}

	settingsEnv := os.Getenv(EnvFCSettings)
	if settingsEnv != "" && len(paths) == 0 {
		paths = append(paths, settingsEnv)
	}
	return paths
}

func (fc *FlexibleConfig) GetPartialsPaths(baseDir string) []string {
	var paths []string

	if fc != nil {
		for _, p := range fc.Partials.Paths {
			if p == "" {
				continue
			}
			if !filepath.IsAbs(p) {
				p = filepath.Join(baseDir, p)
			}
			paths = append(paths, p)
		}
	}

	partialsEnv := os.Getenv(EnvFCPartials)
	if partialsEnv != "" && len(paths) == 0 {
		paths = append(paths, partialsEnv)
	}
	return paths
}

func (fc *FlexibleConfig) GetTemplatesPaths(baseDir string) []string {
	var paths []string

	if fc != nil {
		for _, p := range fc.Templates.Paths {
			if p == "" {
				continue
			}
			if !filepath.IsAbs(p) {
				p = filepath.Join(baseDir, p)
			}
			paths = append(paths, p)
		}
	}

	templatesEnv := os.Getenv(EnvFCTemplates)
	if templatesEnv != "" && len(paths) == 0 {
		paths = append(paths, templatesEnv)
	}
	return paths
}

func (fc *FlexibleConfig) GetRefKey() string {
	if fc != nil && fc.RefKey != "" {
		return fc.RefKey
	}
	return "$ref"
}

func (fc *FlexibleConfig) GetAllowedSuffixes() []string {
	if fc != nil && len(fc.AllowedSuffixes) > 0 {
		return fc.AllowedSuffixes
	}

	if fc != nil && fc.Settings.AllowedSuffixes != nil && len(fc.Settings.AllowedSuffixes) > 0 {
		return fc.Settings.AllowedSuffixes
	}

	return []string{
		".yaml", ".yml",
		".json",
		".toml",
		".env",
		".dotenv",
		".ini",
		".tml",
		".properties", ".prop", ".props",
	}
}

func (fc *FlexibleConfig) GetDirFieldPrefix() string {
	if fc != nil && fc.DirFieldPrefix != "" {
		return fc.DirFieldPrefix
	}
	if fc != nil && fc.Settings.DirFieldPrefix != "" {
		return fc.Settings.DirFieldPrefix
	}
	return ""
}

func (fc *FlexibleConfig) GetAllowOverwrite() bool {
	if fc != nil {
		if fc.AllowOverwrite {
			return true
		}
		if fc.Settings.AllowOverwrite {
			return true
		}
	}
	return false
}

func (fc *FlexibleConfig) ValidateKeyName(key string) error {
	if fc == nil || fc.KeysNamingRules == "" {
		return nil
	}

	if fc.KeysNamingRules == "strict" && !strictKeyRegex.MatchString(key) {
		return &ConfigError{Message: "key contains non-alphanumeric characters (strict mode): " + key}
	}

	return nil
}

func (fc *FlexibleConfig) MergeSettings(dirSettings []map[string]interface{}) (map[string]interface{}, error) {
	result := make(map[string]interface{})

	for _, settings := range dirSettings {
		if settings == nil {
			continue
		}

		flat, err := fc.flattenSettings("", settings)
		if err != nil {
			return nil, err
		}

		for key, val := range flat {
			if err := fc.ValidateKeyName(key); err != nil {
				return nil, err
			}

			if _, exists := result[key]; exists && !fc.GetAllowOverwrite() {
				continue
			}
			result[key] = val
		}
	}

	return result, nil
}

func (fc *FlexibleConfig) flattenSettings(prefix string, settings map[string]interface{}) (map[string]interface{}, error) {
	result := make(map[string]interface{})
	dirPrefix := fc.GetDirFieldPrefix()

	for key, val := range settings {
		fullKey := key
		if prefix != "" {
			if dirPrefix != "" && prefix != "" {
				fullKey = prefix + dirPrefix + "_" + key
			} else {
				fullKey = prefix + "_" + key
			}
		}

		switch v := val.(type) {
		case map[string]interface{}:
			nested, err := fc.flattenSettings(fullKey, v)
			if err != nil {
				return nil, err
			}
			for k, val := range nested {
				result[k] = val
			}
		default:
			result[fullKey] = v
		}
	}

	return result, nil
}

type ConfigError struct {
	Message string
	Err     error
}

func (e *ConfigError) Error() string {
	if e.Err != nil {
		return e.Message + ": " + e.Err.Error()
	}
	return e.Message
}

func (e *ConfigError) Unwrap() error {
	return e.Err
}