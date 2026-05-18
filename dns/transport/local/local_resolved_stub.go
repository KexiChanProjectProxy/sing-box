//go:build !linux

//nolint:unused
package local

import (
	"context"
	"os"

	"github.com/sagernet/sing-box/log"
)

func isSystemdResolvedManaged() bool {
	return false
}

func NewResolvedResolver(ctx context.Context, logger log.StructuredLogger) (ResolvedResolver, error) {
	return nil, os.ErrInvalid
}
