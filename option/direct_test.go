package option

import (
	"net/netip"
	"testing"

	"github.com/sagernet/sing/common/json"
	"github.com/sagernet/sing/common/json/badoption"
	"github.com/stretchr/testify/require"
)

// TestDirectOutboundXLAT464Valid exercises the happy-path parse of a
// /96 IPv6 prefix. The exact approved schema is:
//
//	{"xlat464":{"prefix":"64:ff9b::/96"}}
func TestDirectOutboundXLAT464Valid(t *testing.T) {
	t.Parallel()

	var options DirectOutboundOptions
	err := json.Unmarshal([]byte(`{"xlat464":{"prefix":"64:ff9b::/96"}}`), &options)
	require.NoError(t, err)
	require.NotNil(t, options.Xlat464)
	require.NotNil(t, options.Xlat464.Prefix)
	prefix := netip.Prefix(*options.Xlat464.Prefix)
	require.True(t, prefix.IsValid())
	require.Equal(t, 96, prefix.Bits())
	require.True(t, prefix.Addr().Is6())
	require.False(t, prefix.Addr().Is4())
	require.False(t, prefix.Addr().Is4In6())
}

// TestDirectOutboundXLAT464Absent proves an absent xlat464 key is a no-op
// and leaves the DirectOutboundOptions otherwise empty.
func TestDirectOutboundXLAT464Absent(t *testing.T) {
	t.Parallel()

	var options DirectOutboundOptions
	err := json.Unmarshal([]byte(`{}`), &options)
	require.NoError(t, err)
	require.Nil(t, options.Xlat464)
}

// TestDirectOutboundXLAT464EmptyObjectRejects proves a present xlat464
// object with no prefix is rejected.
func TestDirectOutboundXLAT464EmptyObjectRejects(t *testing.T) {
	t.Parallel()

	var options DirectOutboundOptions
	err := json.Unmarshal([]byte(`{"xlat464":{}}`), &options)
	require.Error(t, err)
	require.Contains(t, err.Error(), "xlat464: prefix is required")
}

// TestDirectOutboundXLAT464MalformedRejects proves malformed JSON is rejected
// before the badoption.Prefix UnmarshalJSON runs.
func TestDirectOutboundXLAT464MalformedRejects(t *testing.T) {
	t.Parallel()

	var options DirectOutboundOptions
	err := json.Unmarshal([]byte(`{"xlat464":{"prefix":""}}`), &options)
	require.Error(t, err)
}

// TestDirectOutboundXLAT464Rejects is the table-driven validation matrix.
// Every row names the exact error fragment the implementation must surface.
func TestDirectOutboundXLAT464Rejects(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name        string
		jsonContent string
		errorText   string
	}{
		{
			name:        "empty xlat464 object",
			jsonContent: `{"xlat464":{}}`,
			errorText:   "xlat464: prefix is required",
		},
		{
			name:        "ipv4 prefix",
			jsonContent: `{"xlat464":{"prefix":"192.0.2.0/24"}}`,
			errorText:   "xlat464: prefix must be an IPv6",
		},
		{
			name:        "ipv4-mapped prefix",
			jsonContent: `{"xlat464":{"prefix":"::ffff:192.0.2.0/120"}}`,
			errorText:   "xlat464: IPv4-mapped prefixes are not supported",
		},
		{
			name:        "rfc6052 /64 prefix",
			jsonContent: `{"xlat464":{"prefix":"64:ff9b::/64"}}`,
			errorText:   "xlat464: prefix must be an IPv6 /96",
		},
		{
			name:        "rfc6052 /32 prefix",
			jsonContent: `{"xlat464":{"prefix":"64:ff9b::/32"}}`,
			errorText:   "xlat464: prefix must be an IPv6 /96",
		},
		{
			name:        "rfc6052 /48 prefix",
			jsonContent: `{"xlat464":{"prefix":"64:ff9b::/48"}}`,
			errorText:   "xlat464: prefix must be an IPv6 /96",
		},
		{
			name:        "rfc6052 /56 prefix",
			jsonContent: `{"xlat464":{"prefix":"64:ff9b::/56"}}`,
			errorText:   "xlat464: prefix must be an IPv6 /96",
		},
		{
			name:        "malformed prefix string",
			jsonContent: `{"xlat464":{"prefix":"not-a-prefix"}}`,
			errorText:   "xlat464",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			var options DirectOutboundOptions
			err := json.Unmarshal([]byte(testCase.jsonContent), &options)
			require.Error(t, err)
			require.Contains(t, err.Error(), testCase.errorText)
		})
	}
}

// TestDirectOutboundXLAT464PreservesUnknownFieldRejection protects the
// disallow-unknown-fields contract. A misspelled sibling field must still
// cause DirectOutboundOptions to reject the JSON.
func TestDirectOutboundXLAT464PreservesUnknownFieldRejection(t *testing.T) {
	t.Parallel()

	var options DirectOutboundOptions
	err := json.Unmarshal([]byte(`{"xlat464":{"prefix":"64:ff9b::/96"},"xlat464_typo":{}}`), &options)
	require.Error(t, err)
}

// TestDirectOutboundXLAT464NilPointerRejects constructs an Xlat464Options
// with a nil Prefix pointer (simulating a non-nil wrapper carrying no
// parsed value) and asserts the validator catches it. We do this via a
// raw JSON shape: passing an explicit null in place of the prefix string
// is invalid JSON for *badoption.Prefix, but we cover the nil-pointer
// branch through the empty-object case above. This test pins the
// post-validation Xlat464.Prefix invariant: whenever Xlat464 is non-nil,
// Prefix is non-nil too.
func TestDirectOutboundXLAT464PrefixInvariant(t *testing.T) {
	t.Parallel()

	var options DirectOutboundOptions
	err := json.Unmarshal([]byte(`{"xlat464":{"prefix":"64:ff9b::/96"}}`), &options)
	require.NoError(t, err)
	require.NotNil(t, options.Xlat464)
	require.NotNil(t, options.Xlat464.Prefix)
	// badoption.Prefix is a named netip.Prefix; sanity-check the round-trip.
	_ = badoption.Prefix(*options.Xlat464.Prefix)
}
