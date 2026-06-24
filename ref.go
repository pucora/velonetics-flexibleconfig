package flexibleconfig

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"
)

const defaultRefKey = "$ref"

type RefResolver struct {
	baseDir     string
	funcMap     template.FuncMap
	resolved    map[string]bool
	currentPath string
	refKey      string
}

func NewRefResolver(baseDir string) *RefResolver {
	return &RefResolver{
		baseDir:  baseDir,
		funcMap:  template.FuncMap{},
		resolved: make(map[string]bool),
		refKey:   defaultRefKey,
	}
}

func (r *RefResolver) AddFunc(name string, f interface{}) {
	r.funcMap[name] = f
}

type refValue struct {
	Ref string `json:"$ref"`
}

var dataPointerRegex = regexp.MustCompile(`^([^#]+)?(#/.*)?$`)

func ParseRef(ref string) (filePath, dataPointer string, err error) {
	if ref == "" {
		return "", "", &RefError{Message: "empty $ref"}
	}

	matches := dataPointerRegex.FindStringSubmatch(ref)
	if matches == nil {
		return "", "", &RefError{Message: "invalid $ref format: " + ref}
	}
	return matches[1], matches[2], nil
}

func (r *RefResolver) resolveDataPointer(data interface{}, pointer string) (interface{}, error) {
	if pointer == "" || pointer == "#" {
		return data, nil
	}

	if !strings.HasPrefix(pointer, "#/") {
		return nil, &RefError{Message: "invalid data pointer format: " + pointer}
	}

	parts := strings.Split(pointer[2:], "/")
	current := data

	for _, part := range parts {
		part = strings.ReplaceAll(part, "~1", "/")
		part = strings.ReplaceAll(part, "~0", "~")

		switch v := current.(type) {
		case map[string]interface{}:
			val, ok := v[part]
			if !ok {
				return nil, &RefError{Message: "data pointer path not found: " + pointer}
			}
			current = val
		case []interface{}:
			idx, err := parseArrayIndex(part)
			if err != nil || idx < 0 || idx >= len(v) {
				return nil, &RefError{Message: "data pointer array index out of bounds: " + part}
			}
			current = v[idx]
		default:
			return nil, &RefError{Message: "cannot traverse non-object/non-array at: " + part}
		}
	}

	return current, nil
}

func parseArrayIndex(s string) (int, error) {
	var idx int
	negative := false
	if s == "-" {
		return -1, nil
	}
	if s[0] == '-' {
		negative = true
		s = s[1:]
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, &RefError{Message: "invalid array index: " + s}
		}
		idx = idx*10 + int(c-'0')
	}
	if negative {
		idx = -idx
	}
	return idx, nil
}

func (r *RefResolver) resolveRef(ref string, vars map[string]interface{}) (interface{}, error) {
	filePath, dataPointer, err := ParseRef(ref)
	if err != nil {
		return nil, err
	}

	resolvedPath := filePath
	if resolvedPath == "" {
		resolvedPath = r.currentPath
	}

	if strings.Contains(resolvedPath, "{{") {
		tmpl, err := template.New("ref").Funcs(r.funcMap).Parse(resolvedPath)
		if err != nil {
			return nil, &RefError{Message: "invalid template in $ref: " + resolvedPath, Err: err}
		}
		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, vars); err != nil {
			return nil, &RefError{Message: "failed to execute $ref template", Err: err}
		}
		resolvedPath = buf.String()
	}

	if !filepath.IsAbs(resolvedPath) {
		resolvedPath = filepath.Join(r.baseDir, resolvedPath)
	}

	resolvedPath = filepath.Clean(resolvedPath)

	absPath, err := filepath.Abs(resolvedPath)
	if err != nil {
		return nil, &RefError{Message: "failed to resolve absolute path", Err: err}
	}

	absPath = filepath.ToSlash(absPath)

	if r.resolved[absPath] {
		return nil, &RefError{Message: "circular reference detected: " + absPath}
	}
	r.resolved[absPath] = true
	r.currentPath = absPath

	data, err := os.ReadFile(absPath)
	if err != nil {
		return nil, &RefError{Message: "failed to read referenced file: " + absPath, Err: err}
	}

	var result interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, &RefError{Message: "failed to parse referenced JSON: " + absPath, Err: err}
	}

	if dataPointer != "" {
		result, err = r.resolveDataPointer(result, dataPointer)
		if err != nil {
			return nil, &RefError{Message: "failed to resolve data pointer in " + absPath, Err: err}
		}
	}

	return result, nil
}

func (r *RefResolver) resolveValue(val interface{}, vars map[string]interface{}) (interface{}, error) {
	switch v := val.(type) {
	case map[string]interface{}:
		if ref, ok := v[r.refKey].(string); ok && len(v) == 1 {
			return r.resolveRef(ref, vars)
		}

		result := make(map[string]interface{})
		for key, value := range v {
			resolved, err := r.resolveValue(value, vars)
			if err != nil {
				return nil, err
			}
			result[key] = resolved
		}
		return result, nil

	case []interface{}:
		result := make([]interface{}, len(v))
		for i, item := range v {
			resolved, err := r.resolveValue(item, vars)
			if err != nil {
				return nil, err
			}
			result[i] = resolved
		}
		return result, nil

	case string:
		if strings.Contains(v, "{{") {
			tmpl, err := template.New("value").Funcs(r.funcMap).Parse(v)
			if err != nil {
				return v, nil
			}
			var buf bytes.Buffer
			if err := tmpl.Execute(&buf, vars); err != nil {
				return v, nil
			}
			return buf.String(), nil
		}
		return v, nil

	default:
		return v, nil
	}
}

func (r *RefResolver) Resolve(data interface{}, vars map[string]interface{}) (interface{}, error) {
	r.resolved = make(map[string]bool)
	r.currentPath = r.baseDir
	return r.resolveValue(data, vars)
}

type RefError struct {
	Message string
	Err     error
}

func (e *RefError) Error() string {
	if e.Err != nil {
		return e.Message + ": " + e.Err.Error()
	}
	return e.Message
}

func (e *RefError) Unwrap() error {
	return e.Err
}