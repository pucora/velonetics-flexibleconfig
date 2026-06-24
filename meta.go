package flexibleconfig

import (
	"os"
	"path/filepath"
	"time"
)

type Meta struct {
	BaseDir     string            `json:"base_dir"`
	WorkingDir  string            `json:"working_dir"`
	ConfigFiles []string          `json:"config_files"`
	Vars        map[string]interface{} `json:"vars"`
	Timestamp   string            `json:"timestamp"`
	Env         map[string]string `json:"env"`
}

func NewMeta(baseDir string, configFiles []string, vars map[string]interface{}) *Meta {
	wd, _ := os.Getwd()

	env := make(map[string]string)
	for _, key := range []string{"FC_ENABLE", "FC_SETTINGS", "FC_PARTIALS", "FC_TEMPLATES", "FC_CONFIG"} {
		if val := os.Getenv(key); val != "" {
			env[key] = val
		}
	}

	return &Meta{
		BaseDir:     baseDir,
		WorkingDir:  wd,
		ConfigFiles: configFiles,
		Vars:        vars,
		Timestamp:   time.Now().Format(time.RFC3339),
		Env:         env,
	}
}

func (m *Meta) AddConfigFile(path string) {
	for _, f := range m.ConfigFiles {
		if f == path {
			return
		}
	}
	m.ConfigFiles = append(m.ConfigFiles, path)
}

func (m *Meta) AddVar(key string, value interface{}) {
	if m.Vars == nil {
		m.Vars = make(map[string]interface{})
	}
	m.Vars[key] = value
}

func (m *Meta) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"base_dir":     m.BaseDir,
		"working_dir":  m.WorkingDir,
		"config_files": m.ConfigFiles,
		"vars":         m.Vars,
		"timestamp":    m.Timestamp,
		"env":          m.Env,
	}
}

func (m *Meta) ResolveTemplates() error {
	return nil
}

type DebugOutput struct {
	Meta       *Meta                  `json:"meta"`
	Rendered   string                 `json:"rendered"`
	Resolved   map[string]interface{} `json:"resolved,omitempty"`
	FilesRead  []string               `json:"files_read,omitempty"`
	FilesWrite []string               `json:"files_write,omitempty"`
}

func NewDebugOutput(meta *Meta) *DebugOutput {
	return &DebugOutput{
		Meta:       meta,
		FilesRead:  []string{},
		FilesWrite: []string{},
	}
}

func (d *DebugOutput) AddFileRead(path string) {
	for _, f := range d.FilesRead {
		if f == path {
			return
		}
	}
	d.FilesRead = append(d.FilesRead, path)
}

func (d *DebugOutput) AddFileWrite(path string) {
	for _, f := range d.FilesWrite {
		if f == path {
			return
		}
	}
	d.FilesWrite = append(d.FilesWrite, path)
}

func resolveMetaPath(baseDir, refPath string) string {
	if filepath.IsAbs(refPath) {
		return refPath
	}
	return filepath.Join(baseDir, refPath)
}