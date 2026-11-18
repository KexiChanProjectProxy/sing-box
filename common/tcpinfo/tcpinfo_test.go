package tcpinfo

import (
	"io"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestReadFromLoopbackConnection(t *testing.T) {
	t.Parallel()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()
	go func() {
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			return
		}
		defer conn.Close()
		_, _ = io.Copy(io.Discard, conn)
	}()
	conn, err := net.Dial("tcp", listener.Addr().String())
	require.NoError(t, err)
	defer conn.Close()
	payload := make([]byte, 256*1024)
	_, err = conn.Write(payload)
	require.NoError(t, err)
	time.Sleep(50 * time.Millisecond)

	info, ok := Read(conn)
	if !Supported {
		require.False(t, ok)
		return
	}
	require.True(t, ok)
	require.NotZero(t, info.DataSegmentsOut)
	require.GreaterOrEqual(t, info.BytesSent, uint64(len(payload)))
	require.Positive(t, info.RTT)
}

func TestReadRejectsNonTCP(t *testing.T) {
	t.Parallel()
	left, right := net.Pipe()
	defer left.Close()
	defer right.Close()
	_, ok := Read(left)
	require.False(t, ok)
}

// The kernel reports the chronograph and byte counters on any kernel this is
// tested on.
func TestReadReportsSenderDetail(t *testing.T) {
	t.Parallel()
	if !Supported {
		t.Skip("TCP_INFO is Linux only")
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()
	go func() {
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			return
		}
		defer conn.Close()
		_, _ = io.Copy(io.Discard, conn)
	}()
	conn, err := net.Dial("tcp", listener.Addr().String())
	require.NoError(t, err)
	defer conn.Close()
	payload := make([]byte, 4<<20)
	_, err = conn.Write(payload)
	require.NoError(t, err)
	time.Sleep(50 * time.Millisecond)

	info, ok := Read(conn)
	require.True(t, ok)
	// The busy time is counted in ticks, which a loopback transfer can fit
	// inside, so only its presence is checked.
	require.True(t, info.HasChrono)
	require.LessOrEqual(t, info.NetworkLimited(), info.BusyTime)
	require.GreaterOrEqual(t, info.BytesAcked, uint64(len(payload)))
	require.Positive(t, info.MinRTT)
	require.NotEmpty(t, info.CongestionControl)
	if info.CongestionControl == "bbr" {
		require.NotNil(t, info.BBR)
		require.Positive(t, info.BBR.Bandwidth)
	}
}
