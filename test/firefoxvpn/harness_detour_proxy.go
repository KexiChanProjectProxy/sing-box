package firefoxvpn

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

func newObservableSOCKSProxy(t *testing.T, tag string) *observableProxy {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	proxy := &observableProxy{
		tag:         tag,
		listener:    listener,
		hostAliases: make(map[string]string),
	}
	proxy.wg.Add(1)
	go func() {
		defer proxy.wg.Done()
		for {
			conn, acceptErr := listener.Accept()
			if acceptErr != nil {
				if errors.Is(acceptErr, net.ErrClosed) {
					return
				}
				return
			}
			proxy.wg.Add(1)
			go func() {
				defer proxy.wg.Done()
				proxy.handleConnection(conn)
			}()
		}
	}()
	t.Cleanup(func() {
		_ = proxy.listener.Close()
		proxy.wg.Wait()
	})
	return proxy
}

func (p *observableProxy) Port() uint16 {
	return uint16(p.listener.Addr().(*net.TCPAddr).Port)
}

func (p *observableProxy) SetHostAlias(host string, replacement string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.hostAliases[host] = replacement
}

func (p *observableProxy) FailDialsWith(err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.forcedConnectError = err
}

func (p *observableProxy) Destinations() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string(nil), p.destinations...)
}

func (p *observableProxy) handleConnection(conn net.Conn) {
	defer conn.Close()
	reader := bufio.NewReader(conn)
	if err := p.negotiate(reader, conn); err != nil {
		return
	}
	destination, err := readSOCKSDestination(reader)
	if err != nil {
		return
	}
	p.recordDestination(destination)
	upstream, err := p.dialDestination(destination)
	if err != nil {
		_, _ = conn.Write([]byte{0x05, 0x01, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		return
	}
	defer upstream.Close()
	_, _ = conn.Write([]byte{0x05, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
	copyDone := make(chan struct{}, 1)
	go func() {
		_, _ = io.Copy(upstream, reader)
		if tcpConn, ok := upstream.(*net.TCPConn); ok {
			_ = tcpConn.CloseWrite()
		}
		copyDone <- struct{}{}
	}()
	_, _ = io.Copy(conn, upstream)
	<-copyDone
}

func (p *observableProxy) negotiate(reader *bufio.Reader, conn net.Conn) error {
	version, err := reader.ReadByte()
	if err != nil {
		return err
	}
	if version != 0x05 {
		return fmt.Errorf("unexpected SOCKS version: %d", version)
	}
	methodCount, err := reader.ReadByte()
	if err != nil {
		return err
	}
	if _, err = io.CopyN(io.Discard, reader, int64(methodCount)); err != nil {
		return err
	}
	_, err = conn.Write([]byte{0x05, 0x00})
	return err
}

func readSOCKSDestination(reader *bufio.Reader) (string, error) {
	requestHeader := make([]byte, 4)
	if _, err := io.ReadFull(reader, requestHeader); err != nil {
		return "", err
	}
	if requestHeader[0] != 0x05 {
		return "", fmt.Errorf("unexpected request version: %d", requestHeader[0])
	}
	if requestHeader[1] != 0x01 {
		return "", fmt.Errorf("unexpected command: %d", requestHeader[1])
	}
	var host string
	switch requestHeader[3] {
	case 0x01:
		addressBytes := make([]byte, net.IPv4len)
		if _, err := io.ReadFull(reader, addressBytes); err != nil {
			return "", err
		}
		host = net.IP(addressBytes).String()
	case 0x03:
		length, err := reader.ReadByte()
		if err != nil {
			return "", err
		}
		domain := make([]byte, length)
		if _, err = io.ReadFull(reader, domain); err != nil {
			return "", err
		}
		host = string(domain)
	case 0x04:
		addressBytes := make([]byte, net.IPv6len)
		if _, err := io.ReadFull(reader, addressBytes); err != nil {
			return "", err
		}
		host = net.IP(addressBytes).String()
	default:
		return "", fmt.Errorf("unexpected address type: %d", requestHeader[3])
	}
	portBytes := make([]byte, 2)
	if _, err := io.ReadFull(reader, portBytes); err != nil {
		return "", err
	}
	return net.JoinHostPort(host, strconv.Itoa(int(binary.BigEndian.Uint16(portBytes)))), nil
}

func (p *observableProxy) recordDestination(destination string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.destinations = append(p.destinations, destination)
}

func (p *observableProxy) dialDestination(destination string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(destination)
	if err != nil {
		return nil, err
	}
	p.mu.Lock()
	replacement := p.hostAliases[host]
	forcedErr := p.forcedConnectError
	p.mu.Unlock()
	if forcedErr != nil {
		return nil, forcedErr
	}
	if replacement != "" {
		host = replacement
	}
	var dialer net.Dialer
	return dialer.Dial("tcp", net.JoinHostPort(host, port))
}
