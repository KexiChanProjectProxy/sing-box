package option

import (
	"bytes"
	"context"
	stdjson "encoding/json"
	"fmt"
	"io"
	"time"

	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/json"
	"gopkg.in/yaml.v3"
)

var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

func stripBOM(content []byte) []byte {
	return bytes.TrimPrefix(content, utf8BOM)
}

// LooksLikeYAML reports whether content should be parsed as YAML rather than
// JSON/JSONC. JSON objects, arrays, and files that start with JSONC comments
// then `{`/`[` are treated as JSON.
func LooksLikeYAML(content []byte) bool {
	return !looksLikeJSON(content)
}

func looksLikeJSON(content []byte) bool {
	s := skipJSONPreamble(content)
	if len(s) == 0 {
		return true
	}
	return s[0] == '{' || s[0] == '['
}

func skipJSONPreamble(content []byte) []byte {
	s := stripBOM(content)
	for {
		s = bytes.TrimLeft(s, " \t\r\n")
		if len(s) == 0 {
			return s
		}
		switch s[0] {
		case '#':
			if i := bytes.IndexByte(s, '\n'); i >= 0 {
				s = s[i+1:]
				continue
			}
			return nil
		case '/':
			if len(s) > 1 && s[1] == '/' {
				if i := bytes.IndexByte(s, '\n'); i >= 0 {
					s = s[i+1:]
					continue
				}
				return nil
			}
			if len(s) > 1 && s[1] == '*' {
				i := bytes.Index(s, []byte("*/"))
				if i < 0 {
					return nil
				}
				s = s[i+2:]
				continue
			}
			return s
		default:
			return s
		}
	}
}

// UnmarshalContext decodes JSON, JSONC, or YAML configuration.
func UnmarshalContext(ctx context.Context, content []byte) (Options, error) {
	content = stripBOM(content)
	if !looksLikeJSON(content) {
		jsonContent, err := YAMLToJSON(content)
		if err != nil {
			return Options{}, err
		}
		return json.UnmarshalExtendedContext[Options](ctx, jsonContent)
	}
	options, err := json.UnmarshalExtendedContext[Options](ctx, content)
	if err == nil {
		return options, nil
	}
	jsonContent, yamlErr := YAMLToJSON(content)
	if yamlErr != nil {
		return Options{}, err
	}
	yamlOptions, yamlErr := json.UnmarshalExtendedContext[Options](ctx, jsonContent)
	if yamlErr != nil {
		return Options{}, err
	}
	return yamlOptions, nil
}

// YAMLToJSON converts a single YAML document to JSON.
func YAMLToJSON(content []byte) ([]byte, error) {
	decoder := yaml.NewDecoder(bytes.NewReader(stripBOM(content)))
	var doc any
	err := decoder.Decode(&doc)
	if err != nil {
		if err == io.EOF {
			return nil, E.New("empty YAML document")
		}
		return nil, err
	}
	var extra any
	extraErr := decoder.Decode(&extra)
	if extraErr == nil {
		return nil, E.New("YAML configuration must contain a single document")
	}
	if extraErr != io.EOF {
		return nil, extraErr
	}
	converted, err := convertYAMLValue(doc)
	if err != nil {
		return nil, err
	}
	return marshalConvertedJSON(converted)
}

// JSONToYAML converts JSON to YAML, preserving object key order.
func JSONToYAML(content []byte) ([]byte, error) {
	var node yaml.Node
	err := yaml.Unmarshal(content, &node)
	if err != nil {
		return nil, err
	}
	setYAMLBlockStyle(&node)
	return yaml.Marshal(&node)
}

func setYAMLBlockStyle(node *yaml.Node) {
	if node == nil {
		return
	}
	switch node.Kind {
	case yaml.DocumentNode, yaml.MappingNode, yaml.SequenceNode, yaml.ScalarNode:
		node.Style = 0
	}
	for _, child := range node.Content {
		setYAMLBlockStyle(child)
	}
}

func marshalConvertedJSON(value any) ([]byte, error) {
	var buffer bytes.Buffer
	encoder := stdjson.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	err := encoder.Encode(value)
	if err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(buffer.Bytes(), []byte{'\n'}), nil
}

func convertYAMLValue(value any) (any, error) {
	switch typed := value.(type) {
	case map[string]any:
		converted := make(map[string]any, len(typed))
		for key, item := range typed {
			next, err := convertYAMLValue(item)
			if err != nil {
				return nil, err
			}
			converted[key] = next
		}
		return converted, nil
	case map[any]any:
		converted := make(map[string]any, len(typed))
		for key, item := range typed {
			next, err := convertYAMLValue(item)
			if err != nil {
				return nil, err
			}
			converted[fmt.Sprint(key)] = next
		}
		return converted, nil
	case []any:
		converted := make([]any, len(typed))
		for i, item := range typed {
			next, err := convertYAMLValue(item)
			if err != nil {
				return nil, err
			}
			converted[i] = next
		}
		return converted, nil
	case time.Time:
		return typed.Format(time.RFC3339Nano), nil
	default:
		return value, nil
	}
}
