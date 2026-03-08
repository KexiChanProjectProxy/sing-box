//go:build libbox_legacy_ipc

package libbox

import (
	"bufio"
	"net"
	"time"

	"github.com/sagernet/sing-box/experimental/clashapi"
	"github.com/sagernet/sing-box/experimental/clashapi/trafficontrol"
	"github.com/sagernet/sing/common/binary"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	"github.com/sagernet/sing/common/varbin"

	"github.com/gofrs/uuid/v5"
)

func (c *CommandClient) handleConnectionsConn(conn net.Conn) {
	defer conn.Close()
	reader := bufio.NewReader(conn)
	var (
		rawConnections []Connection
		connections    Connections
	)
	for {
		rawConnections = nil
		err := varbin.Read(reader, binary.BigEndian, &rawConnections)
		if err != nil {
			c.handler.Disconnected(err.Error())
			return
		}
		connections.input = rawConnections
		c.handler.WriteConnections(&connections)
	}
}

func (s *CommandServer) handleConnectionsConn(conn net.Conn) error {
	var interval int64
	err := binary.Read(conn, binary.BigEndian, &interval)
	if err != nil {
		return E.Cause(err, "read interval")
	}
	ticker := time.NewTicker(time.Duration(interval))
	defer ticker.Stop()
	ctx := connKeepAlive(conn)
	var trafficManager *trafficontrol.Manager
	for {
		service := s.service
		if service != nil {
			trafficManager = service.clashServer.(*clashapi.Server).TrafficManager()
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
	var (
		connections    = make(map[uuid.UUID]*Connection)
		outConnections []Connection
	)
	writer := bufio.NewWriter(conn)
	for {
		outConnections = outConnections[:0]
		for _, connection := range trafficManager.Connections() {
			outConnections = append(outConnections, newConnection(connections, connection, false))
		}
		for _, connection := range trafficManager.ClosedConnections() {
			outConnections = append(outConnections, newConnection(connections, connection, true))
		}
		err = varbin.Write(writer, binary.BigEndian, outConnections)
		if err != nil {
			return err
		}
		err = writer.Flush()
		if err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func newConnection(connections map[uuid.UUID]*Connection, metadata trafficontrol.TrackerMetadata, isClosed bool) Connection {
	if oldConnection, loaded := connections[metadata.ID]; loaded {
		if isClosed {
			if oldConnection.ClosedAt == 0 {
				oldConnection.Uplink = 0
				oldConnection.Downlink = 0
				oldConnection.ClosedAt = metadata.ClosedAt.UnixMilli()
			}
			return *oldConnection
		}
		lastUplink := oldConnection.UplinkTotal
		lastDownlink := oldConnection.DownlinkTotal
		uplinkTotal := metadata.Upload.Load()
		downlinkTotal := metadata.Download.Load()
		oldConnection.Uplink = uplinkTotal - lastUplink
		oldConnection.Downlink = downlinkTotal - lastDownlink
		oldConnection.UplinkTotal = uplinkTotal
		oldConnection.DownlinkTotal = downlinkTotal
		return *oldConnection
	}
	var rule string
	if metadata.Rule != nil {
		rule = metadata.Rule.String()
	}
	uplinkTotal := metadata.Upload.Load()
	downlinkTotal := metadata.Download.Load()
	uplink := uplinkTotal
	downlink := downlinkTotal
	var closedAt int64
	if !metadata.ClosedAt.IsZero() {
		closedAt = metadata.ClosedAt.UnixMilli()
		uplink = 0
		downlink = 0
	}
	connection := Connection{
		ID:            metadata.ID.String(),
		Inbound:       metadata.Metadata.Inbound,
		InboundType:   metadata.Metadata.InboundType,
		IPVersion:     int32(metadata.Metadata.IPVersion),
		Network:       metadata.Metadata.Network,
		Source:        metadata.Metadata.Source.String(),
		Destination:   metadata.Metadata.Destination.String(),
		Domain:        metadata.Metadata.Domain,
		Protocol:      metadata.Metadata.Protocol,
		User:          metadata.Metadata.User,
		FromOutbound:  metadata.Metadata.Outbound,
		CreatedAt:     metadata.CreatedAt.UnixMilli(),
		ClosedAt:      closedAt,
		Uplink:        uplink,
		Downlink:      downlink,
		UplinkTotal:   uplinkTotal,
		DownlinkTotal: downlinkTotal,
		Rule:          rule,
		Outbound:      metadata.Outbound,
		OutboundType:  metadata.OutboundType,
		ChainList:     metadata.Chain,
	}
	connections[metadata.ID] = &connection
	return connection
}
