package noisyshuttle

import (
	"encoding/binary"

	E "github.com/sagernet/sing/common/exceptions"
)

type Hello struct {
	Version      byte
	Capabilities uint16
	MaxStreams   uint16
	Reserved     byte
}

func EncodeHello(hello Hello) []byte {
	payload := make([]byte, 6)
	payload[0] = hello.Version
	binary.BigEndian.PutUint16(payload[1:3], hello.Capabilities)
	binary.BigEndian.PutUint16(payload[3:5], hello.MaxStreams)
	payload[5] = hello.Reserved
	return payload
}

func DecodeHello(payload []byte) (Hello, error) {
	if len(payload) != 6 {
		return Hello{}, E.New("invalid hello length: ", len(payload))
	}
	return Hello{Version: payload[0], Capabilities: binary.BigEndian.Uint16(payload[1:3]), MaxStreams: binary.BigEndian.Uint16(payload[3:5]), Reserved: payload[5]}, nil
}

func ValidateClientHello(hello Hello) error {
	if hello.Version > ProtocolVersion {
		return E.New("version mismatch: ", hello.Version)
	}
	return nil
}

func ValidateServerHello(hello Hello) error {
	if hello.Version != ProtocolVersion {
		return E.New("version mismatch: ", hello.Version)
	}
	return nil
}
