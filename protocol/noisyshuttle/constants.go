package noisyshuttle

const (
	ProtocolVersion = 0x01

	FrameTypeClientHello  = 0x01
	FrameTypeServerHello  = 0x02
	FrameTypeOpenRequest  = 0x03
	FrameTypeOpenResponse = 0x04
	FrameTypeData         = 0x05
	FrameTypeEndRequest   = 0x06
	FrameTypeEndResponse  = 0x07
	FrameTypeReset        = 0x08
	FrameTypePing         = 0x09
	FrameTypePong         = 0x0a
	FrameTypeClose        = 0x0b

	CapabilityReuse        = 0x0001
	CapabilityKeepalive    = 0x0002
	CapabilityUDPAssociate = 0x0004

	CommandConnect      = 0x01
	CommandUDPAssociate = 0x03

	AddressTypeIPv4   = 0x01
	AddressTypeDomain = 0x03
	AddressTypeIPv6   = 0x04

	ErrorOK                       = 0x00
	ErrorAuthFailed               = 0x01
	ErrorBadPreface               = 0x02
	ErrorVersionMismatch          = 0x03
	ErrorUnsupportedCommand       = 0x04
	ErrorInvalidAddress           = 0x05
	ErrorDialFailed               = 0x06
	ErrorNetworkUnreachable       = 0x07
	ErrorHostUnreachable          = 0x08
	ErrorConnectionRefused        = 0x09
	ErrorTTLExpired               = 0x0a
	ErrorProtocol                 = 0x0b
	ErrorUnknownFrame             = 0x0c
	ErrorPayloadTooLarge          = 0x0d
	ErrorStreamIDReused           = 0x0e
	ErrorStreamNotFound           = 0x0f
	ErrorKeepaliveTimeout         = 0x10
	ErrorMaxRequests              = 0x11
	ErrorIdleTimeout              = 0x12
	ErrorShutdownDrain            = 0x13
	ErrorUnsupportedFragmentation = 0x14
	ErrorInternal                 = 0x7f

	SessionStateConnecting = iota
	SessionStateHandshaking
	SessionStateIdle
	SessionStateOpening
	SessionStateActive
	SessionStateDraining
	SessionStateClosing
	SessionStateClosed
	SessionStatePoisoned

	HeaderSize              = 8
	MaxPayloadLength        = 65535
	DefaultClientMinPadding = 0
	DefaultClientMaxPadding = 24
	DefaultServerMaxPadding = 256
	KeepaliveMagic          = 0x6e
)

func ValidFrameType(frameType byte) bool {
	return frameType >= FrameTypeClientHello && frameType <= FrameTypeClose
}
