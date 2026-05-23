package noisyshuttle

import (
	"encoding/binary"
	"net/netip"

	E "github.com/sagernet/sing/common/exceptions"
)

type Address struct {
	Type byte
	Host string
}

type OpenRequest struct {
	Command byte
	Address Address
	Port    uint16
}

type OpenResponse struct {
	Status   byte
	Reserved byte
	Message  string
}

func EncodeAddress(address Address) ([]byte, error) {
	if address.Type == 0 {
		if addr, err := netip.ParseAddr(address.Host); err == nil {
			if addr.Is4() {
				address.Type = AddressTypeIPv4
			} else {
				address.Type = AddressTypeIPv6
			}
		} else {
			address.Type = AddressTypeDomain
		}
	}
	switch address.Type {
	case AddressTypeIPv4:
		addr, err := netip.ParseAddr(address.Host)
		if err != nil || !addr.Is4() {
			return nil, E.New("invalid ipv4 address: ", address.Host)
		}
		bytes := addr.As4()
		return append([]byte{AddressTypeIPv4}, bytes[:]...), nil
	case AddressTypeIPv6:
		addr, err := netip.ParseAddr(address.Host)
		if err != nil || !addr.Is6() || addr.Is4() {
			return nil, E.New("invalid ipv6 address: ", address.Host)
		}
		bytes := addr.As16()
		return append([]byte{AddressTypeIPv6}, bytes[:]...), nil
	case AddressTypeDomain:
		if len(address.Host) == 0 || len(address.Host) > 255 {
			return nil, E.New("invalid domain length: ", len(address.Host))
		}
		encoded := make([]byte, 2+len(address.Host))
		encoded[0] = AddressTypeDomain
		encoded[1] = byte(len(address.Host))
		copy(encoded[2:], address.Host)
		return encoded, nil
	default:
		return nil, E.New("invalid address type: ", address.Type)
	}
}

func DecodeAddress(payload []byte) (Address, int, error) {
	if len(payload) < 1 {
		return Address{}, 0, E.New("missing address type")
	}
	switch payload[0] {
	case AddressTypeIPv4:
		if len(payload) < 5 {
			return Address{}, 0, E.New("truncated ipv4 address")
		}
		addr := netip.AddrFrom4([4]byte(payload[1:5]))
		return Address{Type: AddressTypeIPv4, Host: addr.String()}, 5, nil
	case AddressTypeIPv6:
		if len(payload) < 17 {
			return Address{}, 0, E.New("truncated ipv6 address")
		}
		var bytes [16]byte
		copy(bytes[:], payload[1:17])
		addr := netip.AddrFrom16(bytes)
		return Address{Type: AddressTypeIPv6, Host: addr.String()}, 17, nil
	case AddressTypeDomain:
		if len(payload) < 2 {
			return Address{}, 0, E.New("truncated domain length")
		}
		length := int(payload[1])
		if length == 0 {
			return Address{}, 0, E.New("empty domain")
		}
		if len(payload) < 2+length {
			return Address{}, 0, E.New("truncated domain")
		}
		return Address{Type: AddressTypeDomain, Host: string(payload[2 : 2+length])}, 2 + length, nil
	default:
		return Address{}, 0, E.New("invalid address type: ", payload[0])
	}
}

func EncodeOpenRequest(request OpenRequest) ([]byte, error) {
	if request.Command != CommandConnect && request.Command != CommandUDPAssociate {
		return nil, E.New("unsupported command: ", request.Command)
	}
	if request.Command == CommandConnect && request.Port == 0 {
		return nil, E.New("invalid connect port")
	}
	address, err := EncodeAddress(request.Address)
	if err != nil {
		return nil, err
	}
	payload := make([]byte, 1+len(address)+2)
	payload[0] = request.Command
	copy(payload[1:], address)
	binary.BigEndian.PutUint16(payload[len(payload)-2:], request.Port)
	return payload, nil
}

func DecodeOpenRequest(payload []byte) (OpenRequest, error) {
	if len(payload) < 1+1+2 {
		return OpenRequest{}, E.New("truncated open request")
	}
	command := payload[0]
	if command != CommandConnect && command != CommandUDPAssociate {
		return OpenRequest{}, E.New("unsupported command: ", command)
	}
	address, offset, err := DecodeAddress(payload[1:])
	if err != nil {
		return OpenRequest{}, err
	}
	if len(payload) != 1+offset+2 {
		return OpenRequest{}, E.New("invalid open request length")
	}
	port := binary.BigEndian.Uint16(payload[len(payload)-2:])
	if command == CommandConnect && port == 0 {
		return OpenRequest{}, E.New("invalid connect port")
	}
	return OpenRequest{Command: command, Address: address, Port: port}, nil
}

func EncodeOpenResponse(response OpenResponse) ([]byte, error) {
	if len(response.Message) > 255 {
		return nil, E.New("open response message too long: ", len(response.Message))
	}
	payload := make([]byte, 3+len(response.Message))
	payload[0] = response.Status
	payload[1] = response.Reserved
	payload[2] = byte(len(response.Message))
	copy(payload[3:], response.Message)
	return payload, nil
}

func DecodeOpenResponse(payload []byte) (OpenResponse, error) {
	if len(payload) < 3 {
		return OpenResponse{}, E.New("truncated open response")
	}
	length := int(payload[2])
	if len(payload) != 3+length {
		return OpenResponse{}, E.New("invalid open response length")
	}
	return OpenResponse{Status: payload[0], Reserved: payload[1], Message: string(payload[3:])}, nil
}
