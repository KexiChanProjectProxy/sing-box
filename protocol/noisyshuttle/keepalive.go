package noisyshuttle

import (
	"encoding/binary"

	E "github.com/sagernet/sing/common/exceptions"
)

type Ping struct {
	Timestamp uint64
	Counter   uint32
	Token     uint32
}

type Pong struct {
	Counter uint32
	Token   uint32
}

func EncodePing(ping Ping) []byte {
	payload := make([]byte, 17)
	payload[0] = KeepaliveMagic
	binary.BigEndian.PutUint64(payload[1:9], ping.Timestamp)
	binary.BigEndian.PutUint32(payload[9:13], ping.Counter)
	binary.BigEndian.PutUint32(payload[13:17], ping.Token)
	return payload
}

func DecodePing(payload []byte) (Ping, error) {
	if len(payload) != 17 {
		return Ping{}, E.New("invalid ping length: ", len(payload))
	}
	if payload[0] != KeepaliveMagic {
		return Ping{}, E.New("invalid ping magic: ", payload[0])
	}
	return Ping{Timestamp: binary.BigEndian.Uint64(payload[1:9]), Counter: binary.BigEndian.Uint32(payload[9:13]), Token: binary.BigEndian.Uint32(payload[13:17])}, nil
}

func EncodePong(pong Pong) []byte {
	payload := make([]byte, 9)
	payload[0] = KeepaliveMagic
	binary.BigEndian.PutUint32(payload[1:5], pong.Counter)
	binary.BigEndian.PutUint32(payload[5:9], pong.Token)
	return payload
}

func DecodePong(payload []byte) (Pong, error) {
	if len(payload) != 9 {
		return Pong{}, E.New("invalid pong length: ", len(payload))
	}
	if payload[0] != KeepaliveMagic {
		return Pong{}, E.New("invalid pong magic: ", payload[0])
	}
	return Pong{Counter: binary.BigEndian.Uint32(payload[1:5]), Token: binary.BigEndian.Uint32(payload[5:9])}, nil
}

func ValidatePong(ping Ping, pong Pong) error {
	if ping.Counter != pong.Counter || ping.Token != pong.Token {
		return E.New("pong mismatch")
	}
	return nil
}
