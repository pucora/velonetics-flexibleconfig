package flexibleconfig

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"github.com/pucora/lura/v2/config"
)

type Config struct {
	Settings  string
	Partials  string
	Templates string
	Parser    config.Parser
	Path      string
}

func NewTemplateParser(cfg Config) *TemplateParser {
	t := &TemplateParser{
		Partials:  cfg.Partials,
		Templates: []string{},
		Parser:    cfg.Parser,
		Vars:      map[string]interface{}{},
		Path:      cfg.Path,
		err:       parserError{errors: map[string]error{}},
	}

	if cfg.Settings != "" {
		files, err := os.ReadDir(cfg.Settings)
		if err != nil {
			t.err.errors[cfg.Settings] = err
			files = []os.DirEntry{}
		}
		for _, settingsFile := range files {
			if !strings.HasSuffix(settingsFile.Name(), ".json") {
				continue
			}
			b, err := os.ReadFile(filepath.Join(cfg.Settings, settingsFile.Name()))
			if err != nil {
				t.err.errors[settingsFile.Name()] = err
				continue
			}
			var v map[string]interface{}
			if err := json.Unmarshal(b, &v); err != nil {
				t.err.errors[settingsFile.Name()] = err
				continue
			}
			t.Vars[strings.TrimSuffix(filepath.Base(settingsFile.Name()), ".json")] = v
		}
	}

	if cfg.Templates != "" {
		files, err := os.ReadDir(cfg.Templates)
		if err != nil {
			t.err.errors[cfg.Templates] = err
			files = []os.DirEntry{}
		}
		for _, settingsFile := range files {
			if !strings.HasSuffix(settingsFile.Name(), ".tmpl") {
				continue
			}
			t.Templates = append(t.Templates, filepath.Join(cfg.Templates, settingsFile.Name()))
		}
	}

	t.funcMap = sprig.GenericFuncMap()
	t.funcMap["marshal"] = t.marshal
	t.funcMap["include"] = t.include

	return t
}

type TemplateParser struct {
	Vars        map[string]interface{}
	Partials    string
	Parser      config.Parser
	Templates   []string
	Path        string
	err         parserError
	funcMap     template.FuncMap
	RefKey      string
	Meta        *Meta
	flexCfg     *FlexibleConfig
	resolver    *RefResolver
	extFuncs    *ExtendedFuncs
	lastSource  []byte
	undefinedVars string
}

func (t *TemplateParser) AddFunc(name string, f interface{}) {
	t.funcMap[name] = f
}

func (t *TemplateParser) SetRefKey(key string) {
	t.RefKey = key
}

func (t *TemplateParser) SetMeta(meta *Meta) {
	t.Meta = meta
}

func (t *TemplateParser) SetFlexibleConfig(fc *FlexibleConfig) {
	t.flexCfg = fc
	if fc != nil {
		t.undefinedVars = fc.GetUndefinedVars()
	}
}

func (t *TemplateParser) SetBaseDir(baseDir string) {
	t.resolver = NewRefResolver(baseDir)
}

func (t *TemplateParser) SetExtendedFuncs(ef *ExtendedFuncs) {
	t.extFuncs = ef
	if ef != nil {
		t.undefinedVars = ef.undefinedVars
	}
}

func (t *TemplateParser) parseWithRefs(configFile string) ([]byte, error) {
	data, err := os.ReadFile(configFile)
	if err != nil {
		return nil, err
	}

	var parsed interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil, nil
	}

	if t.resolver == nil {
		t.resolver = NewRefResolver(filepath.Dir(configFile))
	}

	if t.RefKey != "" && t.RefKey != "$ref" {
		t.resolver.refKey = t.RefKey
	}

	resolved, err := t.resolver.Resolve(parsed, t.Vars)
	if err != nil {
		return data, nil
	}

	result, err := json.MarshalIndent(resolved, "", "  ")
	if err != nil {
		return data, nil
	}

	return result, nil
}

func (t *TemplateParser) Parse(configFile string) (config.ServiceConfig, error) {
	if len(t.err.errors) != 0 {
		return config.ServiceConfig{}, t.err
	}

	tmpfile, err := os.CreateTemp("", "Pucora_parsed_config_template_")
	if err != nil {
		log.Println("Couldn't create the temporary file:", err)
		return config.ServiceConfig{}, err
	}

	defer os.Remove(tmpfile.Name())

	refResolvedData, refErr := t.parseWithRefs(configFile)
	if refErr == nil && refResolvedData != nil {
		if err := t.writeOutput(string(refResolvedData)); err != nil {
			log.Println("Warning: failed to write output file:", err)
		}

		if _, err := tmpfile.Write(refResolvedData); err != nil {
			log.Println("Unable to write ref-resolved configuration:", err)
			return t.Parser.Parse(configFile)
		}
		if err := tmpfile.Close(); err != nil {
			log.Println("Unable to close temp file:", err)
			return config.ServiceConfig{}, err
		}

		filename := tmpfile.Name() + ".json"
		if t.Path != "" {
			filename = t.Path
		}
		if err := renameFile(tmpfile.Name(), filename); err != nil {
			return config.ServiceConfig{}, err
		}

		t.lastSource, _ = os.ReadFile(filename)
		cfg, err := t.Parser.Parse(filename)

		if t.Path == "" {
			os.Remove(filename)
		}

		return cfg, err
	}

	var buf bytes.Buffer

	useFuncs := t.funcMap
	if t.extFuncs != nil {
		useFuncs = t.extFuncs.FuncMap()
	}

	tmpl, err := template.New("config").Funcs(useFuncs).ParseFiles(configFile)
	if err != nil {
		log.Println("Unable to parse configuration file:", err)
		return t.Parser.Parse(configFile)
	}
	if len(t.Templates) > 0 {
		tmpl, err = tmpl.ParseFiles(t.Templates...)
		if err != nil {
			log.Println("Error parsing sub-templates:", err)
			return t.Parser.Parse(configFile)
		}
	}

	varsToUse := t.Vars
	if t.Meta != nil {
		if varsToUse == nil {
			varsToUse = make(map[string]interface{})
		}
		metaKey := "meta"
		if t.flexCfg != nil {
			metaKey = t.flexCfg.GetMetaKey()
		}
		varsToUse[metaKey] = t.Meta.ToMap()
	}

	err = tmpl.ExecuteTemplate(&buf, filepath.Base(configFile), varsToUse)
	if err != nil {
		log.Println("Found error while executing template:", err)
		return t.Parser.Parse(configFile)
	}

	if err := t.writeOutput(buf.String()); err != nil {
		log.Println("Warning: failed to write output file:", err)
	}

	if _, err = tmpfile.Write(buf.Bytes()); err != nil {
		log.Println("Unable to write the temporary configuration file:", err)
		return t.Parser.Parse(configFile)
	}
	if err = tmpfile.Close(); err != nil {
		log.Println("Unable to close the file after writing:", err)
		return config.ServiceConfig{}, err
	}

	filename := tmpfile.Name() + ".json"
	if t.Path != "" {
		filename = t.Path
	}
	if err := renameFile(tmpfile.Name(), filename); err != nil {
		return config.ServiceConfig{}, err
	}

	t.lastSource, _ = os.ReadFile(filename)
	cfg, err := t.Parser.Parse(filename)

	if t.Path == "" {
		os.Remove(filename)
	}

	return cfg, err
}

func (t *TemplateParser) LastSource() ([]byte, error) {
	if t.lastSource == nil {
		return nil, fmt.Errorf("no content")
	}
	return t.lastSource, nil
}

func (*TemplateParser) marshal(v interface{}) string {
	a, _ := json.Marshal(v)
	return string(a)
}

func (t *TemplateParser) include(v interface{}) string {
	a, _ := os.ReadFile(path.Join(t.Partials, v.(string)))
	return string(a)
}

func (t *TemplateParser) writeOutput(content string) error {
	if t.flexCfg == nil || t.flexCfg.Out == "" {
		return nil
	}

	absPath, err := filepath.Abs(t.flexCfg.Out)
	if err != nil {
		return fmt.Errorf("failed to resolve output path: %w", err)
	}

	if err := os.WriteFile(absPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write output file: %w", err)
	}

	return nil
}

type parserError struct {
	errors map[string]error
}

func (p parserError) Error() string {
	msgs := make([]string, len(p.errors))
	var j int
	for i, e := range p.errors {
		msgs[j] = fmt.Sprintf("\t- %s: %s", i, e.Error())
		j++
	}
	return "loading flexible-config settings:\n" + strings.Join(msgs, "\n")
}

func renameFile(src, dst string) (err error) {
	err = copyFile(src, dst)
	if err != nil {
		return fmt.Errorf("failed to copy source file %s to %s: %s", src, dst, err)
	}
	err = os.RemoveAll(src)
	if err != nil {
		return fmt.Errorf("failed to cleanup source file %s: %s", src, err)
	}
	return nil
}

func copyFile(src, dst string) (err error) {
	in, err := os.Open(src)
	if err != nil {
		return
	}
	defer func() { err = in.Close() }()

	out, err := os.Create(dst)
	if err != nil {
		return
	}
	defer func() {
		if e := out.Close(); e != nil {
			err = e
		}
	}()

	_, err = io.Copy(out, in)
	if err != nil {
		return
	}

	err = out.Sync()
	if err != nil {
		return
	}

	si, err := os.Stat(src)
	if err != nil {
		return
	}
	err = os.Chmod(dst, si.Mode())
	if err != nil {
		return
	}

	return
}