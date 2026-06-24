package flexibleconfig

import (
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Masterminds/sprig/v3"
	"github.com/pucora/pucora-flexibleconfig/v2/parsers"
)

type ExtendedFuncs struct {
	baseDir       string
	allowMissing  bool
	undefinedVars string
	flexCfg      *FlexibleConfig
}

func NewExtendedFuncs(baseDir string) *ExtendedFuncs {
	return &ExtendedFuncs{
		baseDir:       baseDir,
		allowMissing:  false,
		undefinedVars: "error",
	}
}

func NewExtendedFuncsWithConfig(baseDir string, fc *FlexibleConfig) *ExtendedFuncs {
	ef := &ExtendedFuncs{
		baseDir:       baseDir,
		allowMissing:  false,
		undefinedVars: "error",
		flexCfg:      fc,
	}

	if fc != nil {
		ef.undefinedVars = fc.GetUndefinedVars()
	}

	return ef
}

func (f *ExtendedFuncs) SetUndefinedVars(mode string) {
	f.undefinedVars = mode
}

func (f *ExtendedFuncs) SetBaseDir(baseDir string) {
	f.baseDir = baseDir
}

func (f *ExtendedFuncs) FuncMap() template.FuncMap {
	fm := sprig.GenericFuncMap()

	fm["keys"] = f.keys
	fm["index"] = f.index
	fm["include"] = f.include
	fm["includeErr"] = f.includeErr
	fm["template"] = f.template
	fm["exists"] = f.exists
	fm["merge"] = f.merge
	fm["toJson"] = f.toJson
	fm["fromJson"] = f.fromJson

	return fm
}

func (f *ExtendedFuncs) keys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func (f *ExtendedFuncs) index(m map[string]interface{}, key string) (interface{}, error) {
	v, ok := m[key]
	if !ok {
		switch f.undefinedVars {
		case "error":
			return nil, fmt.Errorf("key not found: %s", key)
		case "zero":
			return nil, nil
		case "invalid":
			return "<no value>", nil
		}
	}
	return v, nil
}

func (f *ExtendedFuncs) include(name string, data interface{}) string {
	result, _ := f.includeErr(name, data)
	return result
}

func (f *ExtendedFuncs) includeErr(name string, data interface{}) (string, error) {
	partialsPaths := f.getPartialsPaths()

	var content []byte
	var err error

	for _, basePath := range partialsPaths {
		filePath := filepath.Join(basePath, name)
		content, err = os.ReadFile(filePath)
		if err == nil {
			break
		}
	}

	if err != nil {
		return "", &IncludeError{File: name, Err: err}
	}

	tmpl, err := template.New(name).Funcs(f.FuncMap()).Parse(string(content))
	if err != nil {
		return "", &IncludeError{File: name, Err: err}
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", &IncludeError{File: name, Err: err}
	}

	return buf.String(), nil
}

func (f *ExtendedFuncs) template(name string, data interface{}) (string, error) {
	templatesPaths := f.getTemplatesPaths()

	var content []byte
	var err error

	for _, basePath := range templatesPaths {
		filePath := filepath.Join(basePath, name)
		content, err = os.ReadFile(filePath)
		if err == nil {
			break
		}
	}

	if err != nil {
		return "", &TemplateError{File: name, Err: err}
	}

	tmpl, err := template.New(name).Funcs(f.FuncMap()).Parse(string(content))
	if err != nil {
		return "", &TemplateError{File: name, Err: err}
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", &TemplateError{File: name, Err: err}
	}

	return buf.String(), nil
}

func (f *ExtendedFuncs) getTemplatesPaths() []string {
	paths := []string{f.baseDir}

	if f.flexCfg != nil {
		for _, p := range f.flexCfg.Templates.Paths {
			if !filepath.IsAbs(p) {
				p = filepath.Join(f.baseDir, p)
			}
			paths = append(paths, p)
		}
	}

	templatesEnv := os.Getenv(EnvFCTemplates)
	if templatesEnv != "" {
		paths = append(paths, templatesEnv)
	}

	return paths
}

func (f *ExtendedFuncs) getPartialsPaths() []string {
	paths := []string{f.baseDir}

	if f.flexCfg != nil {
		for _, p := range f.flexCfg.Partials.Paths {
			if !filepath.IsAbs(p) {
				p = filepath.Join(f.baseDir, p)
			}
			paths = append(paths, p)
		}
	}

	partialsEnv := os.Getenv(EnvFCPartials)
	if partialsEnv != "" {
		paths = append(paths, partialsEnv)
	}

	return paths
}

func (f *ExtendedFuncs) exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func (f *ExtendedFuncs) merge(dest, src map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range dest {
		result[k] = v
	}
	for k, v := range src {
		result[k] = v
	}
	return result
}

func (f *ExtendedFuncs) toJson(v interface{}) (string, error) {
	m, ok := v.(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("toJson: expected map[string]interface{}, got %T", v)
	}
	data, err := parsers.ToJSON(m)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (f *ExtendedFuncs) fromJson(data string) (map[string]interface{}, error) {
	return parsers.FromJSON([]byte(data))
}

type IncludeError struct {
	File string
	Err  error
}

func (e *IncludeError) Error() string {
	return fmt.Sprintf("failed to include %q: %s", e.File, e.Err.Error())
}

func (e *IncludeError) Unwrap() error {
	return e.Err
}

type TemplateError struct {
	File string
	Err  error
}

func (e *TemplateError) Error() string {
	return fmt.Sprintf("failed to render template %q: %s", e.File, e.Err.Error())
}

func (e *TemplateError) Unwrap() error {
	return e.Err
}

func filterBySuffix(files []os.DirEntry, suffixes []string) []os.DirEntry {
	if len(suffixes) == 0 {
		return files
	}

	var filtered []os.DirEntry
	for _, f := range files {
		name := f.Name()
		if parsers.IsValidSuffix(name) {
			for _, suffix := range suffixes {
				ext := filepath.Ext(name)
				if ext == suffix || name == suffix {
					filtered = append(filtered, f)
					break
				}
			}
		}
	}
	return filtered
}

func isAllowedSuffix(filename string, allowedSuffixes []string) bool {
	if len(allowedSuffixes) == 0 {
		return true
	}

	ext := filepath.Ext(filename)
	for _, suffix := range allowedSuffixes {
		if ext == suffix || filename == suffix {
			return true
		}
	}
	return false
}

func mergeSettings(dirSettings []map[string]interface{}, allowOverwrite bool) map[string]interface{} {
	result := make(map[string]interface{})

	for _, settings := range dirSettings {
		for key, val := range settings {
			if _, exists := result[key]; exists && !allowOverwrite {
				continue
			}
			result[key] = val
		}
	}

	return result
}

func collectSettingsFromDir(dirPath string, suffixes []string, prefix string, allowOverwrite bool) (map[string]interface{}, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}

	entries = filterBySuffix(entries, suffixes)

	var dirSettings []map[string]interface{}
	for _, sf := range entries {
		if sf.IsDir() {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dirPath, sf.Name()))
		if err != nil {
			continue
		}
		v, err := parsers.DetectAndParse(b, sf.Name())
		if err != nil {
			continue
		}
		dirSettings = append(dirSettings, v)
	}

	return mergeSettings(dirSettings, allowOverwrite), nil
}