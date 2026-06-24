package parsers

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestGetParser(t *testing.T) {
	tests := []struct {
		filename string
		want     string
	}{
		{"config.json", "json"},
		{"data.yaml", "yaml"},
		{"data.yml", "yaml"},
		{"settings.toml", "toml"},
		{".env", "env"},
		{".dotenv", "dotenv"},
		{"config.ini", "ini"},
		{"config.tml", "ini"},
		{"app.properties", "properties"},
		{"app.prop", "properties"},
		{"app.props", "properties"},
		{"unknown.xyz", ""},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			parser := GetParser(tt.filename)
			if tt.want == "" {
				if parser != nil {
					t.Errorf("GetParser(%q) = %v, want nil", tt.filename, parser)
				}
			} else {
				if parser == nil {
					t.Errorf("GetParser(%q) = nil, want non-nil", tt.filename)
				}
			}
		})
	}
}

func TestParseJSON(t *testing.T) {
	data := []byte(`{"key": "value", "number": 42}`)
	result, err := JSONParser(data)
	if err != nil {
		t.Fatalf("JSONParser() error = %v", err)
	}
	if result["key"] != "value" {
		t.Errorf("result[key] = %v, want value", result["key"])
	}
	if result["number"] != float64(42) {
		t.Errorf("result[number] = %v, want 42", result["number"])
	}
}

func TestParseYAML(t *testing.T) {
	data := []byte(`key: value
number: 42
nested:
  inner: test`)
	result, err := YAMLParser(data)
	if err != nil {
		t.Fatalf("YAMLParser() error = %v", err)
	}
	if result["key"] != "value" {
		t.Errorf("result[key] = %v, want value", result["key"])
	}
	num := result["number"]
	numInt, isInt := num.(int)
	if isInt && numInt == 42 {
		// yaml.v3 parses integers as int
	} else if numFloat, isFloat := num.(float64); !isFloat || numFloat != 42 {
		t.Errorf("result[number] = %v (type %T), want 42", num, num)
	}
	nested, ok := result["nested"].(map[string]interface{})
	if !ok {
		t.Fatalf("result[nested] is not map[string]interface{}")
	}
	if nested["inner"] != "test" {
		t.Errorf("result[nested][inner] = %v, want test", nested["inner"])
	}
}

func TestParseENV(t *testing.T) {
	data := []byte(`KEY=value
NUMBER=42
EMPTY=
# This is a comment
ANOTHER=test`)
	result, err := ENVParser(data)
	if err != nil {
		t.Fatalf("ENVParser() error = %v", err)
	}
	if result["KEY"] != "value" {
		t.Errorf("result[KEY] = %v, want value", result["KEY"])
	}
	if result["NUMBER"] != "42" {
		t.Errorf("result[NUMBER] = %v, want 42", result["NUMBER"])
	}
	if result["EMPTY"] != "" {
		t.Errorf("result[EMPTY] = %v, want empty", result["EMPTY"])
	}
	if _, exists := result["#"]; exists {
		t.Error("lines starting with # should be treated as comments and ignored")
	}
	if result["ANOTHER"] != "test" {
		t.Errorf("result[ANOTHER] = %v, want test", result["ANOTHER"])
	}
}

func TestParseINI(t *testing.T) {
	data := []byte(`key = value
number = 42

[section]
inner = test`)
	result, err := INI(data)
	if err != nil {
		t.Fatalf("INI() error = %v", err)
	}
	if result["key"] != "value" {
		t.Errorf("result[key] = %v, want value", result["key"])
	}
	section, ok := result["section"].(map[string]interface{})
	if !ok {
		t.Fatalf("result[section] is not map[string]interface{}")
	}
	if section["inner"] != "test" {
		t.Errorf("result[section][inner] = %v, want test", section["inner"])
	}
}

func TestParseProperties(t *testing.T) {
	data := []byte(`key=value
number=42
escaped=line1\nline2`)
	result, err := PropsParser(data)
	if err != nil {
		t.Fatalf("PropsParser() error = %v", err)
	}
	if result["key"] != "value" {
		t.Errorf("result[key] = %v, want value", result["key"])
	}
	if result["number"] != "42" {
		t.Errorf("result[number] = %v, want 42", result["number"])
	}
	if result["escaped"] != "line1\nline2" {
		t.Errorf("result[escaped] = %v, want line1\\nline2", result["escaped"])
	}
}

func TestParseFile(t *testing.T) {
	tmpDir := t.TempDir()

	jsonFile := filepath.Join(tmpDir, "test.json")
	os.WriteFile(jsonFile, []byte(`{"json": true}`), 0644)

	result, err := ParseFile(jsonFile)
	if err != nil {
		t.Fatalf("ParseFile() error = %v", err)
	}
	if result["json"] != true {
		t.Errorf("result[json] = %v, want true", result["json"])
	}
}

func TestValidSuffixes(t *testing.T) {
	suffixes := ValidSuffixes()
	if len(suffixes) == 0 {
		t.Error("ValidSuffixes() returned empty slice")
	}

	expected := []string{".json", ".yaml", ".yml", ".toml", ".env", ".dotenv", ".ini", ".tml", ".properties", ".prop", ".props", ".hcl"}
	if len(suffixes) != len(expected) {
		t.Errorf("ValidSuffixes() returned %d suffixes, want %d", len(suffixes), len(expected))
	}
}

func TestIsValidSuffix(t *testing.T) {
	tests := []struct {
		filename string
		want     bool
	}{
		{"test.json", true},
		{"test.yaml", true},
		{"test.yml", true},
		{"test.toml", true},
		{"test.env", true},
		{"test.dotenv", true},
		{"test.ini", true},
		{"test.tml", true},
		{"test.properties", true},
		{"test.xyz", false},
		{"test", false},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			if got := IsValidSuffix(tt.filename); got != tt.want {
				t.Errorf("IsValidSuffix(%q) = %v, want %v", tt.filename, got, tt.want)
			}
		})
	}
}

func TestDetectAndParse(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		data     []byte
	}{
		{"json", "test.json", []byte(`{"key": "value"}`)},
		{"yaml", "test.yaml", []byte(`key: value`)},
		{"yaml alt", "test.yml", []byte(`key: value`)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := DetectAndParse(tt.data, tt.filename)
			if err != nil {
				t.Fatalf("DetectAndParse() error = %v", err)
			}
			if result["key"] != "value" {
				t.Errorf("result[key] = %v, want value", result["key"])
			}
		})
	}
}

func TestToJSON(t *testing.T) {
	m := map[string]interface{}{
		"key":   "value",
		"number": 42,
	}

	data, err := ToJSON(m)
	if err != nil {
		t.Fatalf("ToJSON() error = %v", err)
	}

	if len(data) == 0 {
		t.Error("ToJSON() returned empty data")
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	if parsed["key"] != "value" {
		t.Errorf("parsed[key] = %v, want value", parsed["key"])
	}
}

func TestFromJSON(t *testing.T) {
	data := []byte(`{"key": "value", "number": 42}`)
	m, err := FromJSON(data)
	if err != nil {
		t.Fatalf("FromJSON() error = %v", err)
	}
	if m["key"] != "value" {
		t.Errorf("m[key] = %v, want value", m["key"])
	}
}