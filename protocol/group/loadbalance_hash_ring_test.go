package group

import (
	"hash/fnv"
	"math/rand"
	"sort"
	"strconv"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

// legacySelectFromSnapshot is SelectFromSnapshot as released up to 1.14.4,
// which rebuilt the ring for every connection, kept verbatim so that the
// cached ring can be checked to pick the same member for every key.
func legacySelectFromSnapshot(snapshot *CandidateSnapshot, key string, onEmptyKey string, virtualNodes int, keySalt string) (Candidate, error) {
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

	ring := legacyBuildHashRing(snapshot.Candidates, virtualNodes, keySalt)

	keyHash := legacyHashString(keySalt + key)

	idx := sort.Search(len(ring), func(i int) bool {
		return ring[i].hash >= keyHash
	})

	if idx >= len(ring) {
		idx = 0
	}

	return ring[idx].candidate, nil
}

type legacyRingEntry struct {
	hash      uint32
	candidate Candidate
}

func legacyBuildHashRing(candidates []Candidate, virtualNodes int, keySalt string) []legacyRingEntry {
	var ring []legacyRingEntry
	for _, c := range candidates {
		for i := 0; i < virtualNodes; i++ {
			nodeKey := keySalt + c.Tag + ":" + strconv.Itoa(i)
			h := legacyHashString(nodeKey)
			ring = append(ring, legacyRingEntry{hash: h, candidate: c})
		}
	}

	sort.Slice(ring, func(i, j int) bool {
		return ring[i].hash < ring[j].hash
	})

	return ring
}

func legacyHashString(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}

func ringTestSnapshot(tags []string) *CandidateSnapshot {
	candidates := make([]Candidate, len(tags))
	for i, tag := range tags {
		candidates[i] = Candidate{
			Tag:       tag,
			Latency:   uint16(10 + i),
			Offset:    int32(i),
			Score:     float64(100 + i),
			IsPrimary: i%2 == 0,
		}
	}
	return &CandidateSnapshot{Candidates: candidates, Generation: 1}
}

func ringTestKeys(random *rand.Rand, count int) []string {
	keys := make([]string, 0, count+3)
	for i := 0; i < count; i++ {
		switch i % 3 {
		case 0:
			keys = append(keys, "10."+strconv.Itoa(random.Intn(256))+"."+strconv.Itoa(random.Intn(256))+"."+strconv.Itoa(random.Intn(256)))
		case 1:
			keys = append(keys, "192.0.2."+strconv.Itoa(random.Intn(256))+"|geosite-"+strconv.Itoa(random.Intn(1000)))
		default:
			keys = append(keys, "2001:db8::"+strconv.FormatInt(random.Int63(), 16)+"|example"+strconv.Itoa(i)+".com")
		}
	}
	return append(keys, "a", "\x00", "user@example.com")
}

// legacyPick is the lookup half of legacySelectFromSnapshot over a ring built
// once, so that a test can check thousands of keys without rebuilding it.
func legacyPick(ring []legacyRingEntry, keySalt string, key string) Candidate {
	keyHash := legacyHashString(keySalt + key)
	idx := sort.Search(len(ring), func(i int) bool {
		return ring[i].hash >= keyHash
	})
	if idx >= len(ring) {
		idx = 0
	}
	return ring[idx].candidate
}

// requireSameAsLegacy checks every key through cache, and a few keys through
// the full legacy and uncached paths, which rebuild the ring for each key.
func requireSameAsLegacy(t *testing.T, cache *hashRingCache, snapshot *CandidateSnapshot, keys []string, virtualNodes int, keySalt string) {
	t.Helper()
	for i, key := range keys {
		if i%200 != 0 {
			continue
		}
		want, wantErr := legacySelectFromSnapshot(snapshot, key, "error", virtualNodes, keySalt)
		got, gotErr := SelectFromSnapshot(snapshot, key, "error", virtualNodes, keySalt)
		require.NoError(t, wantErr)
		require.NoError(t, gotErr)
		require.Equal(t, want, got, "uncached: key %q, %d virtual nodes, salt %q", key, virtualNodes, keySalt)
	}
	legacyNodes := virtualNodes
	if legacyNodes <= 0 {
		legacyNodes = defaultVirtualNodes
	}
	ring := legacyBuildHashRing(snapshot.Candidates, legacyNodes, keySalt)
	for _, key := range keys {
		want := legacyPick(ring, keySalt, key)
		got, err := selectFromSnapshot(cache, snapshot, key, "error", virtualNodes, keySalt)
		require.NoError(t, err)
		require.Equal(t, want, got, "cached: key %q, %d virtual nodes, salt %q", key, virtualNodes, keySalt)
	}
}

// TestHashRingMatchesLegacy checks, for both the uncached and the cached
// path, that every key picks the very candidate (every field) the per-call
// ring of earlier releases picked.
func TestHashRingMatchesLegacy(t *testing.T) {
	random := rand.New(rand.NewSource(1))
	keys := ringTestKeys(random, 3000)
	memberSets := [][]string{
		{"solo"},
		{"dmit-1", "dmit-2"},
		{"hk-01", "hk-02", "jp-01", "jp-02", "sg-01", "us-01"},
		{"lb.dmit", "direct-out", "warp", "ucr"},
	}
	for n := 0; n < 4; n++ {
		tags := make([]string, 3+random.Intn(20))
		for i := range tags {
			tags[i] = "node-" + strconv.FormatInt(random.Int63(), 36)
		}
		memberSets = append(memberSets, tags)
	}
	for _, tags := range memberSets {
		snapshot := ringTestSnapshot(tags)
		for _, virtualNodes := range []int{-1, 0, 1, 2, 7, 100, 1000} {
			for _, keySalt := range []string{"", "salt", "中文:"} {
				requireSameAsLegacy(t, &hashRingCache{}, snapshot, keys, virtualNodes, keySalt)
			}
		}
	}
}

// TestHashRingMatchesLegacyOnNodeCollision covers two nodes of different
// members with equal hashes, where only the unstable sort decides which member
// owns the keys at that point. The key is the colliding node key itself, so
// it lands exactly on the tie.
func TestHashRingMatchesLegacyOnNodeCollision(t *testing.T) {
	seen := make(map[uint32]string)
	var first, second string
	for i := 0; first == ""; i++ {
		tag := "m" + strconv.Itoa(i)
		h := legacyHashString(tag + ":0")
		if other, ok := seen[h]; ok {
			first, second = other, tag
		}
		seen[h] = tag
	}
	keys := []string{first + ":0", second + ":0"}
	require.Equal(t, legacyHashString(keys[0]), legacyHashString(keys[1]))
	for _, tags := range [][]string{
		{first, second},
		{second, first},
		{"x", first, "y", second, "z"},
		{second, "x", "y", "z", first},
	} {
		snapshot := ringTestSnapshot(tags)
		for _, virtualNodes := range []int{1, 2, 100, 1000} {
			requireSameAsLegacy(t, &hashRingCache{}, snapshot, keys, virtualNodes, "")
		}
	}
}

// TestHashRingCacheFollowsChanges runs one cache through member changes,
// reorders, parameter changes and fresh latency values, as a load balancer's
// cache sees them over its life, and checks each pick against the legacy ring
// for that snapshot.
func TestHashRingCacheFollowsChanges(t *testing.T) {
	random := rand.New(rand.NewSource(2))
	keys := ringTestKeys(random, 500)
	cache := &hashRingCache{}
	steps := []struct {
		tags         []string
		virtualNodes int
		keySalt      string
	}{
		{[]string{"a", "b", "c"}, 100, ""},
		{[]string{"a", "b", "c"}, 100, ""},
		{[]string{"a", "b"}, 100, ""},
		{[]string{"b", "a"}, 100, ""},
		{[]string{"a", "b", "c", "d"}, 100, ""},
		{[]string{"a", "b", "c", "d"}, 1000, ""},
		{[]string{"a", "b", "c", "d"}, 1000, "s"},
		{[]string{"a", "b", "c", "e"}, 1000, "s"},
		{[]string{"a", "b", "c"}, 100, ""},
	}
	for step, s := range steps {
		snapshot := ringTestSnapshot(s.tags)
		// Same members as the cached ring, new health check results: the
		// pick must carry this snapshot's values, not the cached ones.
		for i := range snapshot.Candidates {
			snapshot.Candidates[i].Latency += uint16(step)
			snapshot.Candidates[i].Score += float64(step)
		}
		requireSameAsLegacy(t, cache, snapshot, keys, s.virtualNodes, s.keySalt)
	}
}

func TestHashRingCacheReusesRing(t *testing.T) {
	cache := &hashRingCache{}
	snapshot := ringTestSnapshot([]string{"a", "b", "c"})
	first := cache.get(snapshot.Candidates, 100, "")
	require.Same(t, first, cache.get(ringTestSnapshot([]string{"a", "b", "c"}).Candidates, 100, ""))
	require.NotSame(t, first, cache.get(snapshot.Candidates, 101, ""))
	require.NotSame(t, first, cache.get(snapshot.Candidates, 100, "x"))
	require.NotSame(t, first, cache.get(ringTestSnapshot([]string{"a", "c", "b"}).Candidates, 100, ""))
}

func TestHashRingCacheConcurrent(t *testing.T) {
	cache := &hashRingCache{}
	snapshots := []*CandidateSnapshot{
		ringTestSnapshot([]string{"a", "b"}),
		ringTestSnapshot([]string{"a", "b", "c", "d", "e", "f"}),
	}
	keys := ringTestKeys(rand.New(rand.NewSource(3)), 200)
	want := make([][]Candidate, len(snapshots))
	for i, snapshot := range snapshots {
		for _, key := range keys {
			candidate, err := legacySelectFromSnapshot(snapshot, key, "error", 100, "")
			require.NoError(t, err)
			want[i] = append(want[i], candidate)
		}
	}
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for round := 0; round < 20; round++ {
				i := (g + round) % len(snapshots)
				for k, key := range keys {
					got, err := selectFromSnapshot(cache, snapshots[i], key, "error", 100, "")
					if err != nil || got != want[i][k] {
						t.Errorf("snapshot %d key %q: got %v (%v), want %v", i, key, got, err, want[i][k])
						return
					}
				}
			}
		}(g)
	}
	wg.Wait()
}

func benchmarkHashSelect(b *testing.B, members int, virtualNodes int, selectFn func(*CandidateSnapshot, string) (Candidate, error)) {
	tags := make([]string, members)
	for i := range tags {
		tags[i] = "lb.dmit-" + strconv.Itoa(i)
	}
	snapshot := ringTestSnapshot(tags)
	keys := ringTestKeys(rand.New(rand.NewSource(4)), 1024)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := selectFn(snapshot, keys[i%len(keys)]); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkConsistentHashSelect(b *testing.B) {
	for _, members := range []int{2, 6} {
		for _, virtualNodes := range []int{100, 1000} {
			name := strconv.Itoa(members) + "members/" + strconv.Itoa(virtualNodes) + "vnodes"
			b.Run(name+"/legacy", func(b *testing.B) {
				benchmarkHashSelect(b, members, virtualNodes, func(snapshot *CandidateSnapshot, key string) (Candidate, error) {
					return legacySelectFromSnapshot(snapshot, key, "error", virtualNodes, "")
				})
			})
			b.Run(name+"/uncached", func(b *testing.B) {
				benchmarkHashSelect(b, members, virtualNodes, func(snapshot *CandidateSnapshot, key string) (Candidate, error) {
					return SelectFromSnapshot(snapshot, key, "error", virtualNodes, "")
				})
			})
			b.Run(name+"/cached", func(b *testing.B) {
				cache := &hashRingCache{}
				benchmarkHashSelect(b, members, virtualNodes, func(snapshot *CandidateSnapshot, key string) (Candidate, error) {
					return selectFromSnapshot(cache, snapshot, key, "error", virtualNodes, "")
				})
			})
		}
	}
}
