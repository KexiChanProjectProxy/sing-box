package noisyshuttle

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"io"

	E "github.com/sagernet/sing/common/exceptions"
)

var crlf = []byte{'\r', '\n'}

func PasswordHash(password string) [32]byte {
	hash := sha256.Sum256([]byte(password))
	return hash
}

func PasswordHashHex(password string) string {
	hash := sha256.Sum256([]byte(password))
	return hex.EncodeToString(hash[:])
}

func EncodePreface(writer io.Writer, password string, paddingLen int) error {
	if paddingLen < 0 || paddingLen > MaxPayloadLength {
		return E.New("invalid padding length: ", paddingLen)
	}
	passwordHash := PasswordHashHex(password)
	if _, err := writer.Write([]byte(passwordHash)); err != nil {
		return err
	}
	if _, err := writer.Write(crlf); err != nil {
		return err
	}
	if paddingLen > 0 {
		padding := make([]byte, paddingLen)
		if _, err := io.ReadFull(rand.Reader, padding); err != nil {
			return err
		}
		if _, err := writer.Write(padding); err != nil {
			return err
		}
	}
	_, err := writer.Write(crlf)
	return err
}

func DecodePreface(reader io.Reader, maxPadding int) ([64]byte, []byte, error) {
	var zero [64]byte
	if maxPadding < 0 {
		return zero, nil, E.New("invalid max padding: ", maxPadding)
	}
	limit := 64 + 2 + maxPadding + 2
	data := make([]byte, limit)
	readLen := 0
	for readLen < limit {
		n, err := reader.Read(data[readLen : readLen+1])
		if n > 0 {
			readLen += n
			if readLen >= 66 {
				padding := data[66:readLen]
				if index := bytes.Index(padding, crlf); index >= 0 {
					if data[64] != '\r' || data[65] != '\n' {
						return zero, nil, E.New("bad preface delimiter")
					}
					if index > maxPadding {
						return zero, nil, E.New("preface padding too large")
					}
					var passwordHash [64]byte
					copy(passwordHash[:], data[:64])
					paddingCopy := make([]byte, index)
					copy(paddingCopy, padding[:index])
					return passwordHash, paddingCopy, nil
				}
			}
		}
		if err != nil {
			return zero, nil, E.Cause(err, "read preface")
		}
	}
	return zero, nil, E.New("preface exceeds maximum length")
}

func VerifyPrefaceHash(received [64]byte, password string) bool {
	expected := PasswordHashHex(password)
	return bytes.Equal(received[:], []byte(expected))
}
