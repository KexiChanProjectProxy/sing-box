package group

import (
	"errors"
	"hash/fnv"
	"math/rand"
	"sort"
	"strconv"

	"github.com/sagernet/sing-box/adapter"
)

type Candidate struct {
	Tag     string
	Outbound adapter.Outbound
	Latency uint16
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

func SelectFromSnapshot(snapshot *CandidateSnapshot, key string, onEmptyKey string, virtualNodes int, keySalt string) (Candidate, error) {
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

	ring := buildHashRing(snapshot.Candidates, virtualNodes, keySalt)

	keyHash := hashString(keySalt + key)

	idx := sort.Search(len(ring), func(i int) bool {
		return ring[i].hash >= keyHash
	})

	if idx >= len(ring) {
		idx = 0
	}

	return ring[idx].candidate, nil
}

type ringEntry struct {
	hash      uint32
	candidate Candidate
}

func buildHashRing(candidates []Candidate, virtualNodes int, keySalt string) []ringEntry {
	var ring []ringEntry
	for _, c := range candidates {
		for i := 0; i < virtualNodes; i++ {
			nodeKey := keySalt + c.Tag + ":" + strconv.Itoa(i)
			h := hashString(nodeKey)
			ring = append(ring, ringEntry{hash: h, candidate: c})
		}
	}

	sort.Slice(ring, func(i, j int) bool {
		return ring[i].hash < ring[j].hash
	})

	return ring
}

func hashString(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}