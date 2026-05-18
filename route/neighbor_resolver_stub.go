//go:build !linux && !darwin

package route

import (
	"os"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/log"
)

func newNeighborResolver(_ log.StructuredLogger, _ []string) (adapter.NeighborResolver, error) {
	return nil, os.ErrInvalid
}
