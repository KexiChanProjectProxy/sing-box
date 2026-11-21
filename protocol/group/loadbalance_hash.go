package group

import (
	"errors"
	"math/rand"
	"sort"
	"strconv"
	"sync/atomic"

	"github.com/sagernet/sing-box/adapter"
)

type Candidate struct {
	Tag      string
	Outbound adapter.Outbound
	// Latency is the member's raw health check delay in milliseconds, kept for
	// health decisions, fail-over ordering and reporting.
	Latency uint16
	// Offset is the member's configured latency_offset in milliseconds. It
	// only shifts ranking: Score includes it, and rankLatency adds it to
	// Latency, but health decisions and reporting read Latency alone.
	Offset int32
	// Score is the sorter's millisecond equivalent ranking value plus Offset,
	// where lower ranks better. With no sorter configured it equals
	// Latency+Offset.
	Score     float64
	IsPrimary bool
}

type CandidateSnapshot struct {
	Candidates []Candidate
	Generation uint64
}

var (
	ErrEmptyHashKey       = errors.New("empty hash key")
	ErrEmptyCandidatePool = errors.New("empty candidate pool")
)

func SelectRandomFromSnapshot(snapshot *CandidateSnapshot) (Candidate, error) {
	if snapshot == nil || len(snapshot.Candidates) == 0 {
		return Candidate{}, ErrEmptyCandidatePool
	}
	return snapshot.Candidates[rand.Intn(len(snapshot.Candidates))], nil
}

const defaultVirtualNodes = 100

// SelectFromSnapshot picks the candidate that owns key on a consistent hash
// ring built from the snapshot. It builds the ring on every call; LoadBalance
// goes through a hashRingCache instead, which picks the same candidate.
func SelectFromSnapshot(snapshot *CandidateSnapshot, key string, onEmptyKey string, virtualNodes int, keySalt string) (Candidate, error) {
	return selectFromSnapshot(nil, snapshot, key, onEmptyKey, virtualNodes, keySalt)
}

func selectFromSnapshot(cache *hashRingCache, snapshot *CandidateSnapshot, key string, onEmptyKey string, virtualNodes int, keySalt string) (Candidate, error) {
	if len(snapshot.Candidates) == 0 {
		return Candidate{}, ErrEmptyCandidatePool
	}

	if key == "" {
		if onEmptyKey == "random" {
			return snapshot.Candidates[rand.Intn(len(snapshot.Candidates))], nil
		}
		return Candidate{}, ErrEmptyHashKey
	}

	if virtualNodes <= 0 {
		virtualNodes = defaultVirtualNodes
	}

	var ring *hashRing
	if cache != nil {
		ring = cache.get(snapshot.Candidates, virtualNodes, keySalt)
	} else {
		ring = buildHashRing(snapshot.Candidates, virtualNodes, keySalt)
	}
	// The ring holds positions, not candidates, so the pick carries the
	// latency and score fields of this snapshot rather than of the snapshot
	// the ring was built from.
	return snapshot.Candidates[ring.lookup(fnv32aAdd(fnv32aAdd(fnv32aOffset, keySalt), key))], nil
}

type ringEntry struct {
	hash uint32
	// member is the owning candidate's position in the candidate list the
	// ring was built from.
	member int32
}

// hashRing is immutable once built. It depends only on the candidates' tags,
// in order, and on virtualNodes and keySalt: the order matters because equal
// node hashes keep the order the sort leaves them in.
type hashRing struct {
	tags         []string
	virtualNodes int
	keySalt      string
	entries      []ringEntry
}

func (r *hashRing) matches(candidates []Candidate, virtualNodes int, keySalt string) bool {
	if r.virtualNodes != virtualNodes || r.keySalt != keySalt || len(r.tags) != len(candidates) {
		return false
	}
	for i := range candidates {
		if r.tags[i] != candidates[i].Tag {
			return false
		}
	}
	return true
}

// lookup returns the member owning the first node at or after keyHash,
// wrapping to the first node.
func (r *hashRing) lookup(keyHash uint32) int32 {
	low, high := 0, len(r.entries)
	for low < high {
		middle := int(uint(low+high) >> 1)
		if r.entries[middle].hash < keyHash {
			low = middle + 1
		} else {
			high = middle
		}
	}
	if low >= len(r.entries) {
		low = 0
	}
	return r.entries[low].member
}

// hashRingCache keeps the last ring a load balancer built, so that the ring is
// rebuilt only when the member list, the virtual node count or the salt
// changes, not on every connection. It is safe for concurrent use: rings are
// immutable, and concurrent rebuilds after a change each build an identical
// ring and the last store wins.
type hashRingCache struct {
	ring atomic.Pointer[hashRing]
}

func (c *hashRingCache) get(candidates []Candidate, virtualNodes int, keySalt string) *hashRing {
	if ring := c.ring.Load(); ring != nil && ring.matches(candidates, virtualNodes, keySalt) {
		return ring
	}
	ring := buildHashRing(candidates, virtualNodes, keySalt)
	c.ring.Store(ring)
	return ring
}

// buildHashRing places virtualNodes nodes per candidate, node i of a candidate
// hashed as keySalt + tag + ":" + i. The sort must stay sort.Slice over this
// entry list: it is not stable, and keeping the same algorithm and input order
// keeps equal hashes in the order earlier releases put them in, so that every
// key keeps its member across upgrades.
func buildHashRing(candidates []Candidate, virtualNodes int, keySalt string) *hashRing {
	tags := make([]string, len(candidates))
	entries := make([]ringEntry, 0, len(candidates)*virtualNodes)
	saltHash := fnv32aAdd(fnv32aOffset, keySalt)
	var buffer [20]byte
	for member, c := range candidates {
		tags[member] = c.Tag
		prefix := fnv32aAdd(fnv32aAdd(saltHash, c.Tag), ":")
		for i := 0; i < virtualNodes; i++ {
			h := fnv32aAddBytes(prefix, strconv.AppendInt(buffer[:0], int64(i), 10))
			entries = append(entries, ringEntry{hash: h, member: int32(member)})
		}
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].hash < entries[j].hash
	})

	return &hashRing{
		tags:         tags,
		virtualNodes: virtualNodes,
		keySalt:      keySalt,
		entries:      entries,
	}
}

// FNV-1a 32, as hash/fnv.New32a, fed incrementally so that hashing a
// concatenation needs neither the concatenated string nor a hash.Hash32.
const (
	fnv32aOffset uint32 = 2166136261
	fnv32aPrime  uint32 = 16777619
)

func fnv32aAdd(h uint32, s string) uint32 {
	for i := 0; i < len(s); i++ {
		h ^= uint32(s[i])
		h *= fnv32aPrime
	}
	return h
}

func fnv32aAddBytes(h uint32, b []byte) uint32 {
	for _, c := range b {
		h ^= uint32(c)
		h *= fnv32aPrime
	}
	return h
}
