package noisyshuttle

import (
	"encoding/binary"
	"sync"
	"time"

	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
)

const (
	UDPFragmentNone = 0x00
)

func EncodeUDPdatagram(address Address, port uint16, payload []byte) ([]byte, error) {
	addrBytes, err := EncodeAddress(address)
	if err != nil {
		return nil, err
	}
	totalLen := 1 + len(addrBytes) + 2 + 2 + len(payload)
	result := make([]byte, totalLen)
	offset := 0
	result[offset] = UDPFragmentNone
	offset++
	copy(result[offset:], addrBytes)
	offset += len(addrBytes)
	binary.BigEndian.PutUint16(result[offset:], port)
	offset += 2
	binary.BigEndian.PutUint16(result[offset:], uint16(len(payload)))
	offset += 2
	copy(result[offset:], payload)
	return result, nil
}

func DecodeUDPdatagram(payload []byte) (Address, uint16, []byte, error) {
	if len(payload) < 5 {
		return Address{}, 0, nil, E.New("truncated UDP datagram")
	}
	offset := 0
	frag := payload[offset]
	offset++
	if frag != UDPFragmentNone {
		return Address{}, 0, nil, E.New("unsupported fragmentation: ", frag)
	}
	addr, consumed, err := DecodeAddress(payload[offset:])
	if err != nil {
		return Address{}, 0, nil, E.Cause(err, "decode UDP address")
	}
	offset += consumed
	if len(payload) < offset+4 {
		return Address{}, 0, nil, E.New("truncated UDP datagram header after address")
	}
	port := binary.BigEndian.Uint16(payload[offset:])
	offset += 2
	dataLen := binary.BigEndian.Uint16(payload[offset:])
	offset += 2
	if int(dataLen) > len(payload)-offset {
		return Address{}, 0, nil, E.New("UDP data length mismatch: expected ", dataLen, " but only ", len(payload)-offset, " bytes available")
	}
	data := payload[offset : offset+int(dataLen)]
	return addr, port, data, nil
}

type NATEntry struct {
	SessionID    uint32
	StreamID     uint32
	Destination  M.Socksaddr
	Created      time.Time
	LastActivity time.Time
	UDPConn      *udpPacketConn
}

type NATManager struct {
	mu       sync.RWMutex
	mappings map[uint32]*NATEntry
	config   *NATManagerConfig
}

type NATManagerConfig struct {
	MaxMappings   int
	IdleTimeout   time.Duration
	MaxPacketSize int
}

func NewNATManager(config *NATManagerConfig) *NATManager {
	if config.MaxMappings <= 0 {
		config.MaxMappings = 16
	}
	if config.IdleTimeout <= 0 {
		config.IdleTimeout = 60 * time.Second
	}
	if config.MaxPacketSize <= 0 {
		config.MaxPacketSize = 1500
	}
	return &NATManager{
		mappings: make(map[uint32]*NATEntry),
		config:   config,
	}
}

func (m *NATManager) Get(streamID uint32) *NATEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.mappings[streamID]
}

func (m *NATManager) Put(streamID uint32, entry *NATEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.mappings) >= m.config.MaxMappings {
		return E.New("max UDP mappings reached")
	}
	entry.Created = time.Now()
	entry.LastActivity = entry.Created
	m.mappings[streamID] = entry
	return nil
}

func (m *NATManager) Remove(streamID uint32) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if entry, ok := m.mappings[streamID]; ok {
		if entry.UDPConn != nil {
			entry.UDPConn.Close()
		}
		delete(m.mappings, streamID)
	}
}

func (m *NATManager) RemoveAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for streamID, entry := range m.mappings {
		if entry.UDPConn != nil {
			entry.UDPConn.Close()
		}
		delete(m.mappings, streamID)
	}
}

func (m *NATManager) TouchActivity(streamID uint32) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if entry, ok := m.mappings[streamID]; ok {
		entry.LastActivity = time.Now()
	}
}

func (m *NATManager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.mappings)
}

func (m *NATManager) IsFull() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.mappings) >= m.config.MaxMappings
}

func (m *NATManager) CleanExpired() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	expired := 0
	for streamID, entry := range m.mappings {
		if now.Sub(entry.LastActivity) > m.config.IdleTimeout {
			if entry.UDPConn != nil {
				entry.UDPConn.Close()
			}
			delete(m.mappings, streamID)
			expired++
		}
	}
	return expired
}

func (m *NATManager) MaxPacketSize() int {
	return m.config.MaxPacketSize
}

func (m *NATManager) IdleTimeout() time.Duration {
	return m.config.IdleTimeout
}

type udpPacketConn struct {
}

func (c *udpPacketConn) Close() error {
	return nil
}
