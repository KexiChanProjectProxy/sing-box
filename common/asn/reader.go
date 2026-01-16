package asn

import (
	"net/netip"

	E "github.com/sagernet/sing/common/exceptions"

	"github.com/oschwald/maxminddb-golang"
)

// ASNRecord represents the structure returned by MaxMind GeoLite2-ASN database
type ASNRecord struct {
	AutonomousSystemNumber       uint   `maxminddb:"autonomous_system_number"`
	AutonomousSystemOrganization string `maxminddb:"autonomous_system_organization"`
}

// Reader provides ASN lookup functionality using MaxMind MMDB format
type Reader struct {
	reader *maxminddb.Reader
}

// Open opens an ASN database file and returns a Reader.
// The database must be in MaxMind MMDB format (GeoLite2-ASN or compatible).
func Open(path string) (*Reader, error) {
	database, err := maxminddb.Open(path)
	if err != nil {
		return nil, err
	}
	// Accept both official MaxMind and custom formats
	dbType := database.Metadata.DatabaseType
	if dbType != "GeoLite2-ASN" && dbType != "sing-asn" {
		database.Close()
		return nil, E.New("incorrect database type, expected GeoLite2-ASN or sing-asn, got ", dbType)
	}
	return &Reader{database}, nil
}

// Lookup returns the Autonomous System Number for the given IP address.
// Returns 0 if the IP is not found in the database or if lookup fails.
func (r *Reader) Lookup(addr netip.Addr) uint {
	var record ASNRecord
	err := r.reader.Lookup(addr.AsSlice(), &record)
	if err != nil {
		return 0
	}
	return record.AutonomousSystemNumber
}

// LookupWithOrg returns both the ASN and organization name for the given IP address.
// Returns (0, "") if the IP is not found in the database or if lookup fails.
func (r *Reader) LookupWithOrg(addr netip.Addr) (uint, string) {
	var record ASNRecord
	err := r.reader.Lookup(addr.AsSlice(), &record)
	if err != nil {
		return 0, ""
	}
	return record.AutonomousSystemNumber, record.AutonomousSystemOrganization
}

// Close closes the ASN database reader and releases resources.
func (r *Reader) Close() error {
	return r.reader.Close()
}
