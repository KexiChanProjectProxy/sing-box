package service

import (
	"context"
	"testing"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/log"

	"github.com/stretchr/testify/require"
)

// Nothing would start or clean up a service created after the manager
// started, so creation is refused instead of silently registering it.
func TestCreateAfterStart(t *testing.T) {
	t.Parallel()
	logger := log.NewNOPFactory().Logger()
	manager := NewManager(NewRegistry())
	scope := adapter.NewScope(context.Background(), logger)
	defer scope.Close()
	require.NoError(t, manager.Start(adapter.StartStateInitialize, scope))
	err := manager.Create(context.Background(), logger, "late", "any", nil)
	require.ErrorContains(t, err, "cannot create service after start")
}
