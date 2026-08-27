package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"

	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/json"
)

func defaultConfigPath() string {
	for _, name := range []string{"config.json", "config.yaml", "config.yml"} {
		if _, err := os.Stat(name); err == nil {
			return name
		}
	}
	return "config.json"
}

func isConfigFileName(name string) bool {
	return strings.HasSuffix(name, ".json") || strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".yml")
}

func isYAMLConfigPath(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".yaml" || ext == ".yml"
}

func (e *OptionsEntry) writeYAML() bool {
	if isYAMLConfigPath(e.path) {
		return true
	}
	if e.path != "stdin" && filepath.Ext(e.path) != "" {
		return false
	}
	return option.LooksLikeYAML(e.content)
}

func encodeConfig(options option.Options, asYAML bool) ([]byte, error) {
	buffer := new(bytes.Buffer)
	encoder := json.NewEncoder(buffer)
	encoder.SetIndent("", "  ")
	err := encoder.Encode(options)
	if err != nil {
		return nil, E.Cause(err, "encode config")
	}
	if !asYAML {
		return buffer.Bytes(), nil
	}
	yamlContent, err := option.JSONToYAML(buffer.Bytes())
	if err != nil {
		return nil, E.Cause(err, "encode yaml config")
	}
	return yamlContent, nil
}
