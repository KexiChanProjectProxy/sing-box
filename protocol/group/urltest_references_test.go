package group

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// In session latency mode any member may carry the next connection, so all
// of them stay referenced and keep their idle connections.
func TestURLTestSessionModeReferencesAllMembers(t *testing.T) {
	t.Parallel()
	urlTest := &URLTest{tags: []string{"a", "b", "c"}, sessionLatency: true, group: &URLTestGroup{}}
	require.Equal(t, []string{"a", "b", "c"}, urlTest.References())
}
