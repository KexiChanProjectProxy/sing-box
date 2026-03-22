package dialer

import (
	"crypto/rand"
	"encoding/binary"
	"hash/fnv"
	"io"
	"net/netip"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/option"
)

const ipv6SourceAddressModeHash5Tuple = option.IPv6SourceAddressMode("hash_5tuple")

type IPv6SourceAddressCandidate struct {
	Address  netip.Addr
	Prefix64 netip.Prefix
}

func SelectIPv6SourceAddressCandidate(prefix netip.Prefix, mode option.IPv6SourceAddressMode, metadata *adapter.InboundContext) (IPv6SourceAddressCandidate, bool) {
	return selectIPv6SourceAddressCandidateWithReader(prefix, mode, metadata, rand.Reader)
}

func selectIPv6SourceAddressCandidateWithReader(prefix netip.Prefix, mode option.IPv6SourceAddressMode, metadata *adapter.InboundContext, randomSource io.Reader) (IPv6SourceAddressCandidate, bool) {
	if !prefix.IsValid() || !prefix.Addr().Is6() || prefix.Bits() < 0 || prefix.Bits() > 64 || randomSource == nil {
		return IPv6SourceAddressCandidate{}, false
	}

	prefix = prefix.Masked()
	prefixBits := prefix.Bits()
	prefixBytes := prefix.Addr().As16()
	prefixUpper64 := binary.BigEndian.Uint64(prefixBytes[:8])
	upperMask := ipv6SourceAddressUpperMask(prefixBits)

	var selectedUpper64 uint64
	switch mode {
	case option.IPv6SourceAddressModeRandom:
		selectedUpper64 = prefixUpper64
		if prefixBits < 64 {
			randomUpper64, loaded := readRandomUint64(randomSource)
			if !loaded {
				return IPv6SourceAddressCandidate{}, false
			}
			selectedUpper64 = (prefixUpper64 & upperMask) | (randomUpper64 &^ upperMask)
		}
	case ipv6SourceAddressModeHash5Tuple:
		hashedUpper64, loaded := hashInboundContext5Tuple(metadata)
		if !loaded {
			return IPv6SourceAddressCandidate{}, false
		}
		selectedUpper64 = prefixUpper64
		if prefixBits < 64 {
			selectedUpper64 = (prefixUpper64 & upperMask) | (hashedUpper64 &^ upperMask)
		}
	default:
		return IPv6SourceAddressCandidate{}, false
	}

	randomLower64, loaded := readRandomUint64(randomSource)
	if !loaded {
		return IPv6SourceAddressCandidate{}, false
	}

	var selected [16]byte
	binary.BigEndian.PutUint64(selected[:8], selectedUpper64)
	binary.BigEndian.PutUint64(selected[8:], randomLower64)

	var prefix64 [16]byte
	binary.BigEndian.PutUint64(prefix64[:8], selectedUpper64)

	return IPv6SourceAddressCandidate{
		Address:  netip.AddrFrom16(selected),
		Prefix64: netip.PrefixFrom(netip.AddrFrom16(prefix64), 64).Masked(),
	}, true
}

func ipv6SourceAddressUpperMask(prefixBits int) uint64 {
	switch prefixBits {
	case 0:
		return 0
	case 64:
		return ^uint64(0)
	default:
		return ^uint64(0) << (64 - prefixBits)
	}
}

func readRandomUint64(randomSource io.Reader) (uint64, bool) {
	var bytes [8]byte
	_, err := io.ReadFull(randomSource, bytes[:])
	if err != nil {
		return 0, false
	}
	return binary.BigEndian.Uint64(bytes[:]), true
}

func hashInboundContext5Tuple(metadata *adapter.InboundContext) (uint64, bool) {
	if metadata == nil || metadata.Network == "" {
		return 0, false
	}
	if !metadata.Source.Addr.IsValid() || metadata.Source.Port == 0 {
		return 0, false
	}
	if !metadata.Destination.Addr.IsValid() || metadata.Destination.Port == 0 {
		return 0, false
	}

	hasher := fnv.New64a()
	writeTupleField(hasher, []byte(metadata.Network))
	writeTupleAddress(hasher, metadata.Source.Addr)
	writeTuplePort(hasher, metadata.Source.Port)
	writeTupleAddress(hasher, metadata.Destination.Addr)
	writeTuplePort(hasher, metadata.Destination.Port)
	return hasher.Sum64(), true
}

func writeTupleField(writer io.Writer, value []byte) {
	_, _ = writer.Write([]byte{byte(len(value))})
	_, _ = writer.Write(value)
}

func writeTupleAddress(writer io.Writer, addr netip.Addr) {
	addressBytes := addr.AsSlice()
	writeTupleField(writer, addressBytes)
}

func writeTuplePort(writer io.Writer, port uint16) {
	var portBytes [2]byte
	binary.BigEndian.PutUint16(portBytes[:], port)
	_, _ = writer.Write(portBytes[:])
}
