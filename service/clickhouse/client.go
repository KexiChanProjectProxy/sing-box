package clickhouse

import (
	"context"
	"net"
	"regexp"
	"strings"
	"time"

	"github.com/sagernet/sing-box/log"
	E "github.com/sagernet/sing/common/exceptions"
)

const (
	defaultMaxWait    = time.Second
	defaultMaxEntries = 100
	defaultQueueSize  = 4096
	defaultNativePort = "9000"
	pushTimeout       = 10 * time.Second
)

const insertColumns = "node, id, start, end, duration_ms, action, network, protocol, user, source_ip, source_port, source_mac, destination_domain, destination_ip, destination_port, inbound, inbound_type, outbound, outbound_type, chain, rule, upload, download, close, process"

var identRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

type preparedBatch interface {
	Append(v ...any) error
	Send() error
	Close() error
}

type batchConn interface {
	PrepareBatch(ctx context.Context, query string) (preparedBatch, error)
	Close() error
}

func parseServer(server string) (string, error) {
	server = strings.TrimSpace(server)
	if server == "" {
		return "", E.New("missing server")
	}
	if strings.Contains(server, "://") {
		return "", E.New("server must be host:port")
	}
	if _, _, err := net.SplitHostPort(server); err != nil {
		if strings.Contains(server, ":") {
			return "", E.Cause(err, "parse server")
		}
		return net.JoinHostPort(server, defaultNativePort), nil
	}
	return server, nil
}

func quoteIdent(name string) (string, error) {
	if !identRe.MatchString(name) {
		return "", E.New("invalid identifier: ", name)
	}
	return "`" + name + "`", nil
}

func tableRef(database string, table string) (string, error) {
	quotedTable, err := quoteIdent(table)
	if err != nil {
		return "", E.Cause(err, "table")
	}
	if database == "" {
		return quotedTable, nil
	}
	quotedDatabase, err := quoteIdent(database)
	if err != nil {
		return "", E.Cause(err, "database")
	}
	return quotedDatabase + "." + quotedTable, nil
}

func buildInsertQuery(database string, table string) (string, error) {
	ref, err := tableRef(database, table)
	if err != nil {
		return "", err
	}
	return "INSERT INTO " + ref + " (" + insertColumns + ")", nil
}

func (s *Service) loopPush() {
	ticker := time.NewTicker(s.maxWait)
	defer ticker.Stop()
	batch := make([]sessionEvent, 0, s.maxEntries)
	flush := func() {
		if len(batch) == 0 {
			return
		}
		err := s.push(batch)
		if err != nil {
			s.logger.WarnEvent("clickhouse.push_failed", "failed to insert session logs",
				log.Err(err),
				log.Int("entries", len(batch)),
			)
		}
		batch = batch[:0]
	}
	for {
		select {
		case <-s.ctx.Done():
			for {
				select {
				case event := <-s.queue:
					batch = append(batch, event)
					if len(batch) >= s.maxEntries {
						flush()
					}
				default:
					flush()
					return
				}
			}
		case event := <-s.queue:
			batch = append(batch, event)
			if len(batch) >= s.maxEntries {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}

func (s *Service) push(events []sessionEvent) error {
	if len(events) == 0 {
		return nil
	}
	if s.conn == nil {
		return E.New("clickhouse not connected")
	}
	ctx, cancel := context.WithTimeout(context.Background(), pushTimeout)
	defer cancel()
	batch, err := s.conn.PrepareBatch(ctx, s.insertQuery)
	if err != nil {
		return err
	}
	defer batch.Close()
	for _, event := range events {
		err = batch.Append(event.appendArgs()...)
		if err != nil {
			return err
		}
	}
	return batch.Send()
}

func (s *Service) enqueue(event sessionEvent) {
	select {
	case <-s.ctx.Done():
		return
	default:
	}
	select {
	case s.queue <- event:
	default:
		dropped := s.dropped.Add(1)
		if dropped == 1 || dropped%1000 == 0 {
			s.logger.WarnEvent("clickhouse.dropped", "session log queue full", log.Uint64("dropped", dropped))
		}
	}
}
