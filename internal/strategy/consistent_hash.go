package strategy

import (
	"hash/crc32"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/angeloszaimis/load-balancer/internal/backend"
)

type consistentHashStrategy struct {
	virtualNodes int
	ring         atomic.Value
	mutex        sync.Mutex
	hashKey      atomic.Uint32
}

type ringSnapshot struct {
	positions []uint32
	owners    map[uint32]*backend.Backend
}

func buildRing(backends []*backend.Backend, vnodes int) *ringSnapshot {
	rs := &ringSnapshot{
		positions: make([]uint32, 0, len(backends)*vnodes),
		owners:    make(map[uint32]*backend.Backend),
	}

	for _, b := range backends {
		for i := 0; i < vnodes; i++ {
			key := b.URL().String() + "#" + strconv.Itoa(i)
			hash := crc32.ChecksumIEEE([]byte(key))

			rs.positions = append(rs.positions, hash)
			rs.owners[hash] = b
		}
	}

	sort.Slice(rs.positions, func(i, j int) bool { return rs.positions[i] < rs.positions[j] })
	return rs
}

func (r *ringSnapshot) lookup(hash uint32) *backend.Backend {
	if r == nil || len(r.positions) == 0 {
		return nil
	}

	idx := sort.Search(len(r.positions), func(i int) bool {
		return r.positions[i] >= hash
	})

	if idx == len(r.positions) {
		idx = 0
	}

	return r.owners[r.positions[idx]]
}

func (s *consistentHashStrategy) SelectBackend(backends []*backend.Backend) *backend.Backend {
	val := s.ring.Load()
	rs, _ := val.(*ringSnapshot)

	if rs == nil || len(rs.positions) == 0 {
		s.mutex.Lock()
		defer s.mutex.Unlock()

		val := s.ring.Load()
		rs, _ = val.(*ringSnapshot)
		if rs == nil || len(rs.positions) == 0 {
			rs = buildRing(backends, s.virtualNodes)
			s.ring.Store(rs)
		}
	}

	return rs.lookup(s.hashKey.Load())
}

func (s *consistentHashStrategy) SetKey(key string) {
	hash := crc32.ChecksumIEEE([]byte(key))
	s.hashKey.Store(hash)
}

func NewConsistentHashStrategy(virtualNodes int) Strategy {
	if virtualNodes <= 0 {
		virtualNodes = 100
	}

	ipHashStrategy := &consistentHashStrategy{virtualNodes: virtualNodes}

	ipHashStrategy.ring.Store(&ringSnapshot{
		positions: nil,
		owners:    nil,
	})

	return ipHashStrategy
}

func (s *consistentHashStrategy) Rebuild(backends []*backend.Backend) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	rs := buildRing(backends, s.virtualNodes)
	s.ring.Store(rs)
}
