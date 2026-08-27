package option

import (
	"bytes"
	"context"
	stdjson "encoding/json"
	"testing"

	"github.com/sagernet/sing/common/json"
	"github.com/stretchr/testify/require"
)

func TestLooksLikeJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		json  bool
	}{
		{name: "object", input: "{\n  \"log\": {}\n}", json: true},
		{name: "array", input: "[1, 2]", json: true},
		{name: "jsonc_line", input: "// comment\n{\"log\":{}}", json: true},
		{name: "jsonc_hash", input: "# comment\n{\"log\":{}}", json: true},
		{name: "jsonc_block", input: "/* comment */\n{\"log\":{}}", json: true},
		{name: "bom_object", input: "\uFEFF{\"log\":{}}", json: true},
		{name: "empty", input: "", json: true},
		{name: "yaml_mapping", input: "log:\n  level: info\n", json: false},
		{name: "yaml_comment", input: "# comment\nlog:\n  level: info\n", json: false},
		{name: "yaml_doc", input: "---\nlog:\n  level: info\n", json: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.json, looksLikeJSON([]byte(tt.input)))
			require.Equal(t, !tt.json, LooksLikeYAML([]byte(tt.input)))
		})
	}
}

func TestYAMLToJSONBasic(t *testing.T) {
	t.Parallel()

	raw, err := YAMLToJSON([]byte(`
log:
  level: info
  timestamp: true
inbounds:
  - type: mixed
    tag: mixed-in
    listen: 127.0.0.1
    listen_port: 1080
`))
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(raw, &decoded))
	logOptions := decoded["log"].(map[string]any)
	require.Equal(t, "info", logOptions["level"])
	require.Equal(t, true, logOptions["timestamp"])
	inbounds := decoded["inbounds"].([]any)
	require.Len(t, inbounds, 1)
	inbound := inbounds[0].(map[string]any)
	require.Equal(t, "mixed", inbound["type"])
	require.Equal(t, "mixed-in", inbound["tag"])
	require.Equal(t, "127.0.0.1", inbound["listen"])
	require.Equal(t, float64(1080), inbound["listen_port"])
}

func TestYAMLToJSONAnchorsAndMerge(t *testing.T) {
	t.Parallel()

	raw, err := YAMLToJSON([]byte(`
inbounds:
  - <<: &listen
      listen: 127.0.0.1
      listen_port: 1080
    type: mixed
    tag: mixed-in
  - <<: *listen
    type: mixed
    tag: mixed-in-2
    listen_port: 1081
`))
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(raw, &decoded))
	inbounds := decoded["inbounds"].([]any)
	require.Len(t, inbounds, 2)
	first := inbounds[0].(map[string]any)
	second := inbounds[1].(map[string]any)
	require.Equal(t, "127.0.0.1", first["listen"])
	require.Equal(t, float64(1080), first["listen_port"])
	require.Equal(t, "mixed-in", first["tag"])
	require.Equal(t, "127.0.0.1", second["listen"])
	require.Equal(t, float64(1081), second["listen_port"])
	require.Equal(t, "mixed-in-2", second["tag"])
}

func TestYAMLToJSONRejectsMultipleDocuments(t *testing.T) {
	t.Parallel()

	_, err := YAMLToJSON([]byte("log:\n  level: info\n---\nlog:\n  level: debug\n"))
	require.Error(t, err)
	require.Contains(t, err.Error(), "single document")
}

func TestYAMLToJSONRejectsEmpty(t *testing.T) {
	t.Parallel()

	_, err := YAMLToJSON(nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "empty YAML document")
}

func TestJSONToYAMLPreservesKeyOrder(t *testing.T) {
	t.Parallel()

	yamlContent, err := JSONToYAML([]byte(`{
  "log": {
    "level": "info",
    "timestamp": true
  }
}`))
	require.NoError(t, err)
	require.Equal(t, "log:\n    level: info\n    timestamp: true\n", string(yamlContent))
}

func TestOptionsUnmarshalYAML(t *testing.T) {
	t.Parallel()

	const yamlConfig = `
# yaml comment
log:
  level: info
  timestamp: true
`
	const jsonConfig = `{
  "log": {
    "level": "info",
    "timestamp": true
  }
}`
	yamlOptions, err := UnmarshalContext(context.Background(), []byte(yamlConfig))
	require.NoError(t, err)
	var jsonOptions Options
	require.NoError(t, json.UnmarshalContext(context.Background(), []byte(jsonConfig), &jsonOptions))

	require.NotNil(t, yamlOptions.Log)
	require.Equal(t, jsonOptions.Log.Level, yamlOptions.Log.Level)
	require.Equal(t, jsonOptions.Log.Timestamp, yamlOptions.Log.Timestamp)
	require.True(t, stdjson.Valid(yamlOptions.RawMessage))
}

func TestOptionsUnmarshalJSONComments(t *testing.T) {
	t.Parallel()

	var options Options
	err := json.UnmarshalContext(context.Background(), []byte(`{
  // keep jsonc
  "log": {
    "level": "debug"
  }
}`), &options)
	require.NoError(t, err)
	require.Equal(t, "debug", options.Log.Level)
	require.NotNil(t, options.Comments())
}

func TestOptionsUnmarshalJSONCHashThenObject(t *testing.T) {
	t.Parallel()

	var options Options
	err := json.UnmarshalContext(context.Background(), []byte(`# hash
{
  "log": { "level": "warn" }
}`), &options)
	require.NoError(t, err)
	require.Equal(t, "warn", options.Log.Level)
}

func TestOptionsUnmarshalYAMLFlowFallback(t *testing.T) {
	t.Parallel()

	options, err := UnmarshalContext(context.Background(), []byte("{log: {level: error}}"))
	require.NoError(t, err)
	require.Equal(t, "error", options.Log.Level)
}

func TestOptionsUnmarshalYAMLUnknownField(t *testing.T) {
	t.Parallel()

	_, err := UnmarshalContext(context.Background(), []byte("not_a_field: true\n"))
	require.Error(t, err)
}

func TestOptionsUnmarshalJSONUnknownField(t *testing.T) {
	t.Parallel()

	var options Options
	err := json.UnmarshalContext(context.Background(), []byte(`{"not_a_field": true}`), &options)
	require.Error(t, err)
}

func TestOptionsUnmarshalYAMLRoundTrip(t *testing.T) {
	t.Parallel()

	options, err := UnmarshalContext(context.Background(), []byte("log:\n  level: info\n"))
	require.NoError(t, err)
	encoded, err := json.MarshalContext(context.Background(), options)
	require.NoError(t, err)
	yamlContent, err := JSONToYAML(encoded)
	require.NoError(t, err)
	roundTrip, err := UnmarshalContext(context.Background(), yamlContent)
	require.NoError(t, err)

	require.Equal(t, options.Log.Level, roundTrip.Log.Level)
	require.True(t, bytes.Contains(yamlContent, []byte("level: info")))
}

func TestOptionsInvalidJSONStillErrors(t *testing.T) {
	t.Parallel()

	var options Options
	err := json.UnmarshalContext(context.Background(), []byte(`{"log":`), &options)
	require.Error(t, err)
}
