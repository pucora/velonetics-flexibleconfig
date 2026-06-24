package parsers

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/pelletier/go-toml/v2"
	"github.com/zclconf/go-cty/cty"
	"gopkg.in/ini.v1"
	"gopkg.in/yaml.v3"
)

type Parser func([]byte) (map[string]interface{}, error)

var(
	JSONParser Parser = parseJSON
	YAMLParser Parser = parseYAML
	TOMLParser Parser = parseTOML
	ENVParser Parser = parseENV
	INI       Parser = parseINI
	PropsParser Parser = parseProperties
	HCLParser  Parser = parseHCL
)

func GetParser(filename string) Parser {
	ext := strings.ToLower(filepath.Ext(filename))
	name := strings.ToLower(filepath.Base(filename))

	switch ext {
	case ".json":
		return JSONParser
	case ".yaml", ".yml":
		return YAMLParser
	case ".toml":
		return TOMLParser
	case ".env", ".dotenv":
		return ENVParser
	case ".ini", ".tml":
		return INI
	case ".properties", ".prop", ".props":
		return PropsParser
	case ".hcl":
		return HCLParser
	}

	if name == ".env" || name == ".dotenv" {
		return ENVParser
	}

	return nil
}

func parseJSON(data []byte) (map[string]interface{}, error) {
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("json parse error: %w", err)
	}
	return result, nil
}

func parseYAML(data []byte) (map[string]interface{}, error) {
	var result map[string]interface{}
	if err := yaml.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("yaml parse error: %w", err)
	}
	flattened := flattenYAML(result)
	if m, ok := flattened.(map[string]interface{}); ok {
		return m, nil
	}
	return result, nil
}

func flattenYAML(v interface{}) interface{} {
	switch val := v.(type) {
	case map[string]interface{}:
		result := make(map[string]interface{})
		for k, v := range val {
			result[k] = flattenYAML(v)
		}
		return result
	case map[interface{}]interface{}:
		result := make(map[string]interface{})
		for k, v := range val {
			key, ok := k.(string)
			if !ok {
				key = fmt.Sprintf("%v", k)
			}
			result[key] = flattenYAML(v)
		}
		return result
	case []interface{}:
		result := make([]interface{}, len(val))
		for i, v := range val {
			result[i] = flattenYAML(v)
		}
		return result
	default:
		return val
	}
}

func parseTOML(data []byte) (map[string]interface{}, error) {
	var result map[string]interface{}
	if err := toml.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("toml parse error: %w", err)
	}
	return result, nil
}

func parseENV(data []byte) (map[string]interface{}, error) {
	result := make(map[string]interface{})
	lines := strings.Split(string(data), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if idx := strings.IndexByte(line, '='); idx != -1 {
			key := strings.TrimSpace(line[:idx])
			value := strings.TrimSpace(line[idx+1:])

			value = strings.Trim(value, "\"")

			value = strings.TrimPrefix(value, "'")
			value = strings.TrimSuffix(value, "'")

			result[key] = value
		}
	}

	return result, nil
}

func parseINI(data []byte) (map[string]interface{}, error) {
	cfg, err := ini.Load(data)
	if err != nil {
		return nil, fmt.Errorf("ini parse error: %w", err)
	}

	result := make(map[string]interface{})

	for _, section := range cfg.Sections() {
		sectionName := section.Name()
		if sectionName == "DEFAULT" || sectionName == "" {
			for _, key := range section.Keys() {
				result[key.Name()] = key.Value()
			}
		} else {
			sectionMap := make(map[string]interface{})
			for _, key := range section.Keys() {
				sectionMap[key.Name()] = key.Value()
			}
			result[sectionName] = sectionMap
		}
	}

	return result, nil
}

func parseProperties(data []byte) (map[string]interface{}, error) {
	result := make(map[string]interface{})
	lines := strings.Split(string(data), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "!") {
			continue
		}

		if idx := strings.IndexByte(line, '='); idx != -1 {
			key := strings.TrimSpace(line[:idx])
			value := strings.TrimSpace(line[idx+1:])

			value = unescapePropertiesValue(value)

			result[key] = value
		} else if idx := strings.IndexByte(line, ':'); idx != -1 {
			key := strings.TrimSpace(line[:idx])
			value := strings.TrimSpace(line[idx+1:])

			value = unescapePropertiesValue(value)

			result[key] = value
		}
	}

	return result, nil
}

func unescapePropertiesValue(value string) string {
	value = strings.ReplaceAll(value, "\\n", "\n")
	value = strings.ReplaceAll(value, "\\t", "\t")
	value = strings.ReplaceAll(value, "\\r", "\r")
	value = strings.ReplaceAll(value, "\\\\", "\\")
	return value
}

func parseHCL(data []byte) (map[string]interface{}, error) {
	parser := hclparse.NewParser()

	file, diag := parser.ParseHCL(data, "")
	if diag != nil && diag.HasErrors() {
		return nil, fmt.Errorf("hcl parse error: %s", diag.Error())
	}

	if file == nil {
		return nil, fmt.Errorf("hcl: no file parsed")
	}

	result := make(map[string]interface{})

	body := file.Body

	content, _ := body.Content(&hcl.BodySchema{
		Attributes: []hcl.AttributeSchema{},
		Blocks:     []hcl.BlockHeaderSchema{},
	})

	for _, attr := range content.Attributes {
		value, diag := attr.Expr.Value(nil)
		if diag == nil || !diag.HasErrors() {
			val, _ := ctyToGo(value)
			result[attr.Name] = val
		}
	}

	for _, block := range content.Blocks {
		blockContent, blockDiags := block.Body.Content(&hcl.BodySchema{
			Attributes: []hcl.AttributeSchema{},
			Blocks:     []hcl.BlockHeaderSchema{},
		})
		if blockDiags.HasErrors() {
			continue
		}

		blockMap := make(map[string]interface{})

		for _, attr := range blockContent.Attributes {
			value, diag := attr.Expr.Value(nil)
			if diag == nil || !diag.HasErrors() {
				val, _ := ctyToGo(value)
				blockMap[attr.Name] = val
			}
		}

		for _, nestedBlock := range blockContent.Blocks {
			nestedContent, nestedDiags := nestedBlock.Body.Content(&hcl.BodySchema{
				Attributes: []hcl.AttributeSchema{},
				Blocks:     []hcl.BlockHeaderSchema{},
			})
			if nestedDiags.HasErrors() {
				continue
			}

			nestedMap := make(map[string]interface{})

			for _, attr := range nestedContent.Attributes {
				value, diag := attr.Expr.Value(nil)
				if diag == nil || !diag.HasErrors() {
					val, _ := ctyToGo(value)
					nestedMap[attr.Name] = val
				}
			}

			if len(nestedMap) > 0 {
				blockMap[nestedBlock.Type] = nestedMap
			}
		}

		if len(blockMap) > 0 {
			result[block.Type] = blockMap
		}
	}

	return result, nil
}

func ctyToGo(v cty.Value) (interface{}, error) {
	if v.IsNull() {
		return nil, nil
	}

	ty := v.Type()

	if ty.IsTupleType() {
		elems := v.AsValueSlice()
		result := make([]interface{}, len(elems))
		for i, elem := range elems {
			r, _ := ctyToGo(elem)
			result[i] = r
		}
		return result, nil
	}

	if ty.IsObjectType() {
		vals := v.AsValueMap()
		result := make(map[string]interface{})
		for k, val := range vals {
			r, _ := ctyToGo(val)
			result[k] = r
		}
		return result, nil
	}

	if ty.IsPrimitiveType() {
		if ty.Equals(cty.String) {
			return v.AsString(), nil
		}
		if ty.Equals(cty.Number) {
			return v.AsBigFloat().String(), nil
		}
		if ty.Equals(cty.Bool) {
			return v.True, nil
		}
	}

	if ty.Equals(cty.DynamicPseudoType) {
		return nil, nil
	}

	return v.AsValueSlice(), nil
}

func ParseFile(filename string) (map[string]interface{}, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", filename, err)
	}

	parser := GetParser(filename)
	if parser == nil {
		return nil, fmt.Errorf("unsupported file format: %s", filename)
	}

	return parser(data)
}

func Parse(r io.Reader, format string) (map[string]interface{}, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read data: %w", err)
	}

	var parser Parser
	switch strings.ToLower(format) {
	case "json":
		parser = JSONParser
	case "yaml", "yml":
		parser = YAMLParser
	case "toml":
		parser = TOMLParser
	case "env", "dotenv":
		parser = ENVParser
	case "ini", "tml":
		parser = INI
	case "properties", "prop", "props":
		parser = PropsParser
	default:
		return nil, fmt.Errorf("unsupported format: %s", format)
	}

	return parser(data)
}

func DetectAndParse(data []byte, filename string) (map[string]interface{}, error) {
	if parser := GetParser(filename); parser != nil {
		return parser(data)
	}

	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err == nil {
		return m, nil
	}

	if err := yaml.Unmarshal(data, &m); err == nil {
		flattened := flattenYAML(m)
		if result, ok := flattened.(map[string]interface{}); ok {
			return result, nil
		}
		return m, nil
	}

	return nil, fmt.Errorf("unable to parse data as any supported format")
}

func ValidSuffixes() []string {
	return []string{
		".json",
		".yaml", ".yml",
		".toml",
		".env", ".dotenv",
		".ini", ".tml",
		".properties", ".prop", ".props",
		".hcl",
	}
}

func IsValidSuffix(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	name := strings.ToLower(filepath.Base(filename))

	for _, suffix := range ValidSuffixes() {
		if ext == suffix {
			return true
		}
	}

	return name == ".env" || name == ".dotenv"
}

func ConvertToJSON(v map[string]interface{}) ([]byte, error) {
	return json.MarshalIndent(v, "", "  ")
}

func MustParse(filename string) map[string]interface{} {
	result, err := ParseFile(filename)
	if err != nil {
		panic(err)
	}
	return result
}

type multiFormatParser struct {
	filename string
}

func NewParser(filename string) *multiFormatParser {
	return &multiFormatParser{filename: filename}
}

func (p *multiFormatParser) Parse(data []byte) (map[string]interface{}, error) {
	return DetectAndParse(data, p.filename)
}

func FromJSON(data []byte) (map[string]interface{}, error) {
	return JSONParser(data)
}

func ToJSON(v map[string]interface{}) ([]byte, error) {
	return json.MarshalIndent(v, "", "  ")
}