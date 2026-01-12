package hash

import (
	"crypto/md5"
	"encoding/binary"
	"fmt"
	"sort"
	"sync"

	"github.com/sagernet/sing-box/adapter"
)

// HashRing implements consistent hashing with virtual nodes
type HashRing struct {
	mu           sync.RWMutex
	ring         map[uint32]string // hash -> node name
	sortedHashes []uint32          // sorted hash values for binary search
	virtualNodes int               // number of virtual nodes per real node
	nodes        map[string]bool   // track which nodes are in the ring
}

// NewHashRing creates a new consistent hash ring
func NewHashRing(virtualNodes int) *HashRing {
	if virtualNodes <= 0 {
		virtualNodes = 10 // default to 10 virtual nodes
	}

	return &HashRing{
		ring:         make(map[uint32]string),
		sortedHashes: make([]uint32, 0),
		virtualNodes: virtualNodes,
		nodes:        make(map[string]bool),
	}
}

// Add adds a node to the hash ring
func (h *HashRing) Add(node string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.nodes[node] {
		return // already exists
	}

	h.nodes[node] = true

	// Add virtual nodes
	for i := 0; i < h.virtualNodes; i++ {
		virtualKey := fmt.Sprintf("%s#%d", node, i)
		hash := hashKey(virtualKey)
		h.ring[hash] = node
		h.sortedHashes = append(h.sortedHashes, hash)
	}

	// Re-sort the hashes
	sort.Slice(h.sortedHashes, func(i, j int) bool {
		return h.sortedHashes[i] < h.sortedHashes[j]
	})
}

// Remove removes a node from the hash ring
func (h *HashRing) Remove(node string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if !h.nodes[node] {
		return // doesn't exist
	}

	delete(h.nodes, node)

	// Remove virtual nodes
	newSortedHashes := make([]uint32, 0, len(h.sortedHashes)-h.virtualNodes)
	for _, hash := range h.sortedHashes {
		if h.ring[hash] == node {
			delete(h.ring, hash)
		} else {
			newSortedHashes = append(newSortedHashes, hash)
		}
	}
	h.sortedHashes = newSortedHashes
}

// Get returns the node for a given key using consistent hashing
func (h *HashRing) Get(key string) (string, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if len(h.sortedHashes) == 0 {
		return "", false
	}

	hash := hashKey(key)

	// Binary search to find the first hash >= key's hash
	idx := sort.Search(len(h.sortedHashes), func(i int) bool {
		return h.sortedHashes[i] >= hash
	})

	// Wrap around if we're past the end
	if idx >= len(h.sortedHashes) {
		idx = 0
	}

	return h.ring[h.sortedHashes[idx]], true
}

// Nodes returns all nodes in the ring
func (h *HashRing) Nodes() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	nodes := make([]string, 0, len(h.nodes))
	for node := range h.nodes {
		nodes = append(nodes, node)
	}
	return nodes
}

// Size returns the number of nodes in the ring
func (h *HashRing) Size() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.nodes)
}

// hashKey generates a hash value for a key
func hashKey(key string) uint32 {
	hash := md5.Sum([]byte(key))
	return binary.BigEndian.Uint32(hash[:4])
}

// BuildHashKey builds a composite hash key from connection metadata based on specified key parts
// Supported key parts:
//   - src_ip: Source IP address
//   - dst_ip: Destination IP address
//   - dst_port: Destination port number
//   - network: Network type (tcp/udp)
//   - domain: Full destination domain
//   - inbound_tag: Inbound connection tag
//   - matched_ruleset: Matched ruleset tag
//   - etld_plus_one: Top-level domain extraction
//   - matched_ruleset_or_etld: Smart fallback (ruleset if matched, else eTLD+1)
//   - salt: Custom randomization string
func BuildHashKey(metadata *adapter.InboundContext, keyParts []string, salt string) string {
	if len(keyParts) == 0 {
		return ""
	}

	parts := make([]string, 0, len(keyParts))

	for _, part := range keyParts {
		switch part {
		case "src_ip":
			if metadata.Source.IsValid() {
				parts = append(parts, metadata.Source.Addr.String())
			} else {
				parts = append(parts, "-")
			}

		case "dst_ip":
			if metadata.Destination.IsValid() {
				parts = append(parts, metadata.Destination.Addr.String())
			} else {
				parts = append(parts, "-")
			}

		case "dst_port":
			if metadata.Destination.IsValid() && metadata.Destination.Port != 0 {
				parts = append(parts, fmt.Sprintf("%d", metadata.Destination.Port))
			} else {
				parts = append(parts, "-")
			}

		case "network":
			if metadata.Network != "" {
				parts = append(parts, metadata.Network)
			} else {
				parts = append(parts, "-")
			}

		case "domain":
			if metadata.Domain != "" {
				parts = append(parts, metadata.Domain)
			} else {
				parts = append(parts, "-")
			}

		case "inbound_tag":
			if metadata.Inbound != "" {
				parts = append(parts, metadata.Inbound)
			} else {
				parts = append(parts, "-")
			}

		case "matched_ruleset":
			if metadata.MatchedRuleSet != "" {
				parts = append(parts, metadata.MatchedRuleSet)
			} else {
				parts = append(parts, "-")
			}

		case "etld_plus_one":
			if metadata.Domain != "" {
				etld := ExtractETLDPlusOne(metadata.Domain)
				parts = append(parts, etld)
			} else {
				parts = append(parts, "-")
			}

		case "matched_ruleset_or_etld":
			// Smart fallback: use ruleset if matched, else eTLD+1
			if metadata.MatchedRuleSet != "" {
				parts = append(parts, metadata.MatchedRuleSet)
			} else if metadata.Domain != "" {
				etld := ExtractETLDPlusOne(metadata.Domain)
				parts = append(parts, etld)
			} else {
				parts = append(parts, "-")
			}

		case "salt":
			if salt != "" {
				parts = append(parts, salt)
			}
		}
	}

	// Join all parts with a delimiter
	key := ""
	for i, part := range parts {
		if i > 0 {
			key += "|"
		}
		key += part
	}

	return key
}
