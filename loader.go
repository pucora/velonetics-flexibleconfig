package flexibleconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pucora/pucora-flexibleconfig/v2/parsers"
)

type Loader struct {
	flexCfg     *FlexibleConfig
	baseDir     string
	configFiles []string
	meta        *Meta
}

func NewLoader(configPath string) (*Loader, error) {
	flexCfg, baseDir, err := LoadFlexibleConfig(configPath)
	if err != nil {
		return nil, err
	}

	if flexCfg == nil {
		flexCfg = &FlexibleConfig{}
		baseDir, _ = os.Getwd()
	}

	loader := &Loader{
		flexCfg:     flexCfg,
		baseDir:     baseDir,
		configFiles: []string{},
		meta:        NewMeta(baseDir, []string{}, nil),
	}

	if configPath != "" {
		loader.configFiles = append(loader.configFiles, configPath)
	}

	return loader, nil
}

func NewLoaderWithConfig(flexCfg *FlexibleConfig, baseDir string) *Loader {
	if flexCfg == nil {
		flexCfg = &FlexibleConfig{}
	}

	if baseDir == "" {
		baseDir, _ = os.Getwd()
	}

	return &Loader{
		flexCfg:     flexCfg,
		baseDir:     baseDir,
		configFiles: []string{},
		meta:        NewMeta(baseDir, []string{}, nil),
	}
}

func (l *Loader) FlexibleConfig() *FlexibleConfig {
	return l.flexCfg
}

func (l *Loader) BaseDir() string {
	return l.baseDir
}

func (l *Loader) Meta() *Meta {
	return l.meta
}

func (l *Loader) IsEnabled() bool {
	return l.flexCfg.IsEnabled() || os.Getenv(EnvFCEnable) != ""
}

func (l *Loader) IsDebug() bool {
	return l.flexCfg.IsDebug()
}

func (l *Loader) LoadSettings() (map[string]interface{}, error) {
	settingsPaths := l.flexCfg.GetSettingsPaths(l.baseDir)

	if len(settingsPaths) == 0 {
		settingsEnv := os.Getenv(EnvFCSettings)
		if settingsEnv != "" {
			settingsPaths = append(settingsPaths, settingsEnv)
		}
	}

	if len(settingsPaths) == 0 {
		return make(map[string]interface{}), nil
	}

	var allSettings []map[string]interface{}

	for _, settingsPath := range settingsPaths {
		absPath, err := filepath.Abs(settingsPath)
		if err == nil {
			l.meta.AddConfigFile(absPath)
		}

		stat, err := os.Stat(settingsPath)
		if err != nil {
			continue
		}

		if stat.IsDir() {
			settings, err := l.loadSettingsRecursive(settingsPath, "")
			if err == nil && settings != nil {
				allSettings = append(allSettings, settings)
			}
		} else {
			settings, err := l.loadSettingsFile(settingsPath)
			if err == nil {
				allSettings = append(allSettings, settings)
			}
		}
	}

	mergedSettings, err := l.flexCfg.MergeSettings(allSettings)
	if err != nil {
		return nil, err
	}

	if l.IsDebug() {
		l.printSettingsTree(mergedSettings)
	}

	return mergedSettings, nil
}

func (l *Loader) loadSettingsRecursive(dirPath string, prefix string) (map[string]interface{}, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}

	allowedSuffixes := l.flexCfg.GetAllowedSuffixes()
	allowOverwrite := l.flexCfg.GetAllowOverwrite()
	dirPrefix := l.flexCfg.GetDirFieldPrefix()

	result := make(map[string]interface{})
	var dirSettings []map[string]interface{}

	for _, entry := range entries {
		if entry.IsDir() {
			newPrefix := entry.Name()
			if prefix != "" {
				if dirPrefix != "" {
					newPrefix = prefix + dirPrefix + "_" + entry.Name()
				} else {
					newPrefix = prefix + "_" + entry.Name()
				}
			}

			subSettings, err := l.loadSettingsRecursive(filepath.Join(dirPath, entry.Name()), newPrefix)
			if err != nil {
				continue
			}

			if subSettings != nil {
				dirSettings = append(dirSettings, subSettings)
			}
			continue
		}

		if !isAllowedSuffix(entry.Name(), allowedSuffixes) {
			continue
		}

		filePath := filepath.Join(dirPath, entry.Name())
		settings, err := l.loadSettingsFile(filePath)
		if err != nil {
			continue
		}

		dirSettings = append(dirSettings, settings)
	}

	for _, settings := range dirSettings {
		for key, val := range settings {
			if _, exists := result[key]; exists && !allowOverwrite {
				continue
			}
			result[key] = val
		}
	}

	return result, nil
}

func (l *Loader) loadSettingsFile(filePath string) (map[string]interface{}, error) {
	l.meta.AddConfigFile(filePath)

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	settings, err := parsers.DetectAndParse(data, filePath)
	if err != nil {
		return nil, err
	}

	return settings, nil
}

func (l *Loader) printSettingsTree(settings map[string]interface{}) {
	fmt.Println("=== Flexible Config Settings Tree ===")
	printMap(settings, "", 0)
	fmt.Println("=====================================")
}

func printMap(m map[string]interface{}, prefix string, depth int) {
	indent := strings.Repeat("  ", depth)
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sortStrings(keys)

	for _, k := range keys {
		v := m[k]
		switch val := v.(type) {
		case map[string]interface{}:
			fmt.Printf("%s%s:\n", indent, k)
			printMap(val, prefix, depth+1)
		default:
			fmt.Printf("%s%s: %v\n", indent, k, val)
		}
	}
}

func sortStrings(s []string) {
	for i := 0; i < len(s)-1; i++ {
		for j := i + 1; j < len(s); j++ {
			if s[i] > s[j] {
				s[i], s[j] = s[j], s[i]
			}
		}
	}
}

func (l *Loader) LoadPartials() (map[string]string, error) {
	partialsPaths := l.flexCfg.GetPartialsPaths(l.baseDir)

	if len(partialsPaths) == 0 {
		partialsEnv := os.Getenv(EnvFCPartials)
		if partialsEnv != "" {
			partialsPaths = append(partialsPaths, partialsEnv)
		}
	}

	partials := make(map[string]string)

	for _, partialsPath := range partialsPaths {
		absPath, err := filepath.Abs(partialsPath)
		if err == nil {
			l.meta.AddConfigFile(absPath)
		}

		err = l.loadPartialsRecursive(partialsPath, "", partials)
		if err != nil {
			continue
		}
	}

	return partials, nil
}

func (l *Loader) loadPartialsRecursive(dirPath, prefix string, partials map[string]string) error {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			newPrefix := entry.Name()
			if prefix != "" {
				newPrefix = prefix + "/" + entry.Name()
			}
			if err := l.loadPartialsRecursive(filepath.Join(dirPath, entry.Name()), newPrefix, partials); err != nil {
				continue
			}
			continue
		}

		filePath := filepath.Join(dirPath, entry.Name())
		content, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		name := entry.Name()
		if prefix != "" {
			name = prefix + "/" + name
		}

		partials[name] = string(content)
	}

	return nil
}

func (l *Loader) LoadTemplates() ([]string, error) {
	templatesPaths := l.flexCfg.GetTemplatesPaths(l.baseDir)

	if len(templatesPaths) == 0 {
		templatesEnv := os.Getenv(EnvFCTemplates)
		if templatesEnv != "" {
			templatesPaths = append(templatesPaths, templatesEnv)
		}
	}

	var templates []string

	for _, templatesPath := range templatesPaths {
		absPath, err := filepath.Abs(templatesPath)
		if err == nil {
			l.meta.AddConfigFile(absPath)
		}

		found, err := l.loadTemplatesRecursive(templatesPath, &templates)
		if err != nil {
			continue
		}
		if found {
			continue
		}
	}

	return templates, nil
}

func (l *Loader) loadTemplatesRecursive(dirPath string, templates *[]string) (bool, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return false, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			l.loadTemplatesRecursive(filepath.Join(dirPath, entry.Name()), templates)
			continue
		}

		if filepath.Ext(entry.Name()) != ".tmpl" {
			continue
		}

		*templates = append(*templates, filepath.Join(dirPath, entry.Name()))
	}

	return true, nil
}

func (l *Loader) ResolveRefs(data interface{}, vars map[string]interface{}) (interface{}, error) {
	refKey := l.flexCfg.GetRefKey()
	resolver := NewRefResolver(l.baseDir)
	resolver.refKey = refKey

	return resolver.Resolve(data, vars)
}

func (l *Loader) WriteOutput(content string, outputPath string) error {
	if outputPath == "" {
		outputPath = l.flexCfg.Out
	}

	if outputPath == "" {
		return nil
	}

	absPath, err := filepath.Abs(outputPath)
	if err != nil {
		return err
	}

	if err := os.WriteFile(absPath, []byte(content), 0644); err != nil {
		return err
	}

	l.meta.AddConfigFile(absPath)
	return nil
}

func (l *Loader) GetDebugOutput() *DebugOutput {
	return NewDebugOutput(l.meta)
}

type SettingsMerger struct {
	allowOverwrite bool
	dirFieldPrefix string
	keyRules       string
}

func NewSettingsMerger(fc *FlexibleConfig) *SettingsMerger {
	if fc == nil {
		return &SettingsMerger{}
	}
	return &SettingsMerger{
		allowOverwrite: fc.GetAllowOverwrite(),
		dirFieldPrefix:  fc.GetDirFieldPrefix(),
		keyRules:        fc.GetKeysNamingRules(),
	}
}

func (m *SettingsMerger) Merge(allSettings []map[string]interface{}) (map[string]interface{}, error) {
	result := make(map[string]interface{})

	for _, settings := range allSettings {
		if settings == nil {
			continue
		}

		flat, err := m.flatten("", settings)
		if err != nil {
			return nil, err
		}

		for key, val := range flat {
			if m.keyRules == "strict" && !isValidStrictKey(key) {
				return nil, fmt.Errorf("key contains invalid characters (strict mode): %s", key)
			}

			if _, exists := result[key]; exists && !m.allowOverwrite {
				continue
			}
			result[key] = val
		}
	}

	return result, nil
}

func (m *SettingsMerger) flatten(prefix string, settings map[string]interface{}) (map[string]interface{}, error) {
	result := make(map[string]interface{})

	for key, val := range settings {
		fullKey := key
		if prefix != "" {
			if m.dirFieldPrefix != "" {
				fullKey = prefix + m.dirFieldPrefix + "_" + key
			} else {
				fullKey = prefix + "_" + key
			}
		}

		switch v := val.(type) {
		case map[string]interface{}:
			nested, err := m.flatten(fullKey, v)
			if err != nil {
				return nil, err
			}
			for k, v := range nested {
				result[k] = v
			}
		default:
			result[fullKey] = v
		}
	}

	return result, nil
}

func isValidStrictKey(key string) bool {
	for _, c := range key {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_') {
			return false
		}
	}
	return true
}

func (l *Loader) ValidateKeys(settings map[string]interface{}) error {
	keyRules := l.flexCfg.GetKeysNamingRules()
	if keyRules != "strict" {
		return nil
	}

	return validateKeysStrict("", settings)
}

func validateKeysStrict(prefix string, settings map[string]interface{}) error {
	for key, val := range settings {
		fullKey := key
		if prefix != "" {
			fullKey = prefix + "_" + key
		}

		if !isValidStrictKey(key) {
			return fmt.Errorf("key contains invalid characters (strict mode): %s", fullKey)
		}

		if nested, ok := val.(map[string]interface{}); ok {
			if err := validateKeysStrict(fullKey, nested); err != nil {
				return err
			}
		}
	}

	return nil
}