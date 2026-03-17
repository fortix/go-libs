# netx/dns

DNS resolver with caching, parallel queries, and domain-specific nameservers.

## Features

- Parallel DNS queries to multiple nameservers
- UDP with TCP fallback
- TTL-based caching with automatic cleanup
- Domain-specific nameservers
- System resolver fallback
- Injectable logger interface
- Mock-friendly for testing

## Installation

```bash
go get github.com/fortix/go-libs/netx/dns
```

## Usage

```go
import "github.com/fortix/go-libs/netx/dns"
```

### Basic Example

```go
// Create resolver with caching
resolver := dns.NewDNSResolver(dns.ResolverConfig{
    EnableCache:  true,
    MaxCacheTTL:  300,  // 5 minutes max
    QueryTimeout: 2 * time.Second,
})
defer resolver.Stop()

// Configure nameservers
resolver.UpdateNameservers([]string{
    "8.8.8.8",
    "1.1.1.1",
})

// Query A records
records, err := resolver.QueryUpstream("example.com", "A")
if err != nil {
    log.Fatal(err)
}
for _, r := range records {
    fmt.Printf("%s -> %s (TTL: %d)\n", r.Name, r.Target, r.TTL)
}
```

### Domain-Specific Nameservers

```go
resolver.UpdateNameservers([]string{
    "8.8.8.8",                        // Default nameserver
    "example.com/192.168.1.1",        // For *.example.com queries
    "internal.corp/10.0.0.1:5353",    // For *.internal.corp with custom port
})
```

### With Custom Logger

```go
type myLogger struct{}

func (l *myLogger) Trace(msg string, kv ...any) { /* ... */ }
func (l *myLogger) Debug(msg string, kv ...any) { /* ... */ }
func (l *myLogger) Info(msg string, kv ...any)  { /* ... */ }
func (l *myLogger) Warn(msg string, kv ...any)  { /* ... */ }
func (l *myLogger) Error(msg string, kv ...any) { /* ... */ }
func (l *myLogger) Fatal(msg string, kv ...any) { /* ... */ }

resolver := dns.NewDNSResolver(dns.ResolverConfig{
    Logger: &myLogger{},
})
```

### Supported Record Types

- `A` - IPv4 addresses
- `AAAA` - IPv6 addresses
- `CNAME` - Canonical names
- `MX` - Mail exchange
- `SRV` - Service records
- `TXT` - Text records

### SRV Record Lookup

```go
// Lookup SRV and resolve to TCP addresses
addrs, err := resolver.LookupSRV("_ldap._tcp.example.com")
if err != nil {
    log.Fatal(err)
}
for _, addr := range addrs {
    fmt.Printf("%s:%d\n", addr.IP, addr.Port)
}
```

### IP Lookup

```go
// Lookup IP addresses (tries A first, then AAAA)
ips, err := resolver.LookupIP("example.com")
if err != nil {
    log.Fatal(err)
}
fmt.Println(ips) // ["93.184.216.34"]
```

### SRV HTTP URLs

```go
// Resolve srv+ prefix to get port from SRV record
url := resolver.ResolveSRVHttp("srv+api.example.com/path")
// Returns: "https://api.example.com:8443/path" (port from SRV)
```

### Global Functions

```go
// Use the default resolver
dns.UpdateNameservers([]string{"8.8.8.8"})

records, _ := dns.LookupIP("example.com")
addrs, _ := dns.LookupSRV("_service._tcp.example.com")
url := dns.ResolveSRVHttp("srv+api.example.com")
```

## Configuration

| Option | Type | Description |
|--------|------|-------------|
| `QueryTimeout` | `time.Duration` | Timeout for DNS queries. Default: 2s |
| `EnableCache` | `bool` | Enable response caching. Default: false |
| `MaxCacheTTL` | `int` | Maximum cache TTL in seconds. 0 = unlimited |
| `Logger` | `logger.Logger` | Optional logger for debug output |

## DNSRecord Type

```go
type DNSRecord struct {
    Type     string // "A", "AAAA", "CNAME", "MX", "SRV", "TXT"
    Name     string // FQDN
    Target   string // IP, hostname, or value
    Port     int    // SRV only
    Priority int    // MX and SRV
    Weight   int    // SRV only
    TTL      int    // Seconds
}
```

## Testing with Mock

```go
import "github.com/miekg/dns"

// Create a mock client
type mockClient struct {
    records []dns.RR
}

func (m *mockClient) Exchange(ctx context.Context, msg *dns.Msg, ns string) (*dns.Msg, error) {
    return &dns.Msg{
        MsgHdr: dns.MsgHdr{Rcode: dns.RcodeSuccess},
        Answer: m.records,
    }, nil
}

// Use in tests
resolver := dns.NewDNSResolver(dns.ResolverConfig{})
resolver.SetClient(&mockClient{
    records: []dns.RR{
        &dns.A{
            Hdr: dns.RR_Header{Name: "example.com.", Rrtype: dns.TypeA, Ttl: 300},
            A:   net.ParseIP("93.184.216.34"),
        },
    },
})
resolver.UpdateNameservers([]string{"8.8.8.8"})

records, _ := resolver.QueryUpstream("example.com", "A")
```

## API

### `NewDNSResolver(config ResolverConfig) *DNSResolver`

Creates a new resolver with the given configuration.

### `UpdateNameservers(nameservers []string)`

Updates the nameserver configuration.

### `QueryUpstream(name, recordType string) ([]DNSRecord, error)`

Queries upstream DNS for records of the specified type.

### `LookupSRV(service string) ([]*net.TCPAddr, error)`

Looks up SRV records and resolves targets to TCP addresses.

### `LookupIP(host string) ([]string, error)`

Looks up IP addresses for a host (A then AAAA).

### `ResolveSRVHttp(uri string) string`

Resolves a URL with SRV lookup if prefixed with `srv+`.

### `ClearCache()`

Clears the DNS cache.

### `Stop()`

Stops the resolver and cleanup goroutines.

### `SetClient(client dnsClient)`

Sets a custom DNS client (for testing).
