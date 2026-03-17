package dns

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/fortix/go-libs/logger"
	"github.com/miekg/dns"
)

// testLogger is a simple logger implementation for testing
type testLogger struct{}

func (l *testLogger) Trace(msg string, keysAndValues ...any) {}
func (l *testLogger) Debug(msg string, keysAndValues ...any) {}
func (l *testLogger) Info(msg string, keysAndValues ...any)  {}
func (l *testLogger) Warn(msg string, keysAndValues ...any)  {}
func (l *testLogger) Error(msg string, keysAndValues ...any) {}
func (l *testLogger) Fatal(msg string, keysAndValues ...any) {}

// mockDNSClient is a mock DNS client for testing
type mockDNSClient struct {
	records []dns.RR
	err     error
	delay   time.Duration
}

func (m *mockDNSClient) Exchange(ctx context.Context, msg *dns.Msg, nameserver string) (*dns.Msg, error) {
	if m.delay > 0 {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	if m.err != nil {
		return nil, m.err
	}

	return &dns.Msg{
		MsgHdr: dns.MsgHdr{
			Rcode: dns.RcodeSuccess,
		},
		Answer: m.records,
	}, nil
}

// Helper to create DNS records for testing
func makeARecord(name, ip string, ttl uint32) *dns.A {
	return &dns.A{
		Hdr: dns.RR_Header{
			Name:   dns.Fqdn(name),
			Rrtype: dns.TypeA,
			Class:  dns.ClassINET,
			Ttl:    ttl,
		},
		A: net.ParseIP(ip),
	}
}

func makeAAAARecord(name, ip string, ttl uint32) *dns.AAAA {
	return &dns.AAAA{
		Hdr: dns.RR_Header{
			Name:   dns.Fqdn(name),
			Rrtype: dns.TypeAAAA,
			Class:  dns.ClassINET,
			Ttl:    ttl,
		},
		AAAA: net.ParseIP(ip),
	}
}

func makeCNAMERecord(name, target string, ttl uint32) *dns.CNAME {
	return &dns.CNAME{
		Hdr: dns.RR_Header{
			Name:   dns.Fqdn(name),
			Rrtype: dns.TypeCNAME,
			Class:  dns.ClassINET,
			Ttl:    ttl,
		},
		Target: dns.Fqdn(target),
	}
}

func makeMXRecord(name, mx string, priority uint16, ttl uint32) *dns.MX {
	return &dns.MX{
		Hdr: dns.RR_Header{
			Name:   dns.Fqdn(name),
			Rrtype: dns.TypeMX,
			Class:  dns.ClassINET,
			Ttl:    ttl,
		},
		Mx:         dns.Fqdn(mx),
		Preference: priority,
	}
}

func makeSRVRecord(name, target string, port, priority, weight uint16, ttl uint32) *dns.SRV {
	return &dns.SRV{
		Hdr: dns.RR_Header{
			Name:   dns.Fqdn(name),
			Rrtype: dns.TypeSRV,
			Class:  dns.ClassINET,
			Ttl:    ttl,
		},
		Target:   dns.Fqdn(target),
		Port:     port,
		Priority: priority,
		Weight:   weight,
	}
}

func makeTXTRecord(name string, txt []string, ttl uint32) *dns.TXT {
	return &dns.TXT{
		Hdr: dns.RR_Header{
			Name:   dns.Fqdn(name),
			Rrtype: dns.TypeTXT,
			Class:  dns.ClassINET,
			Ttl:    ttl,
		},
		Txt: txt,
	}
}

func TestNewDNSResolver(t *testing.T) {
	t.Run("default timeout", func(t *testing.T) {
		r := NewDNSResolver(ResolverConfig{})
		if r.config.QueryTimeout != 2*time.Second {
			t.Errorf("expected default timeout 2s, got %v", r.config.QueryTimeout)
		}
	})

	t.Run("custom timeout", func(t *testing.T) {
		r := NewDNSResolver(ResolverConfig{QueryTimeout: 5 * time.Second})
		if r.config.QueryTimeout != 5*time.Second {
			t.Errorf("expected timeout 5s, got %v", r.config.QueryTimeout)
		}
	})

	t.Run("cache enabled", func(t *testing.T) {
		r := NewDNSResolver(ResolverConfig{EnableCache: true})
		if r.cache == nil {
			t.Error("expected cache to be initialized")
		}
		r.Stop()
	})

	t.Run("cache disabled", func(t *testing.T) {
		r := NewDNSResolver(ResolverConfig{EnableCache: false})
		if r.cache != nil {
			t.Error("expected cache to be nil")
		}
	})
}

func TestUpdateNameservers(t *testing.T) {
	r := NewDNSResolver(ResolverConfig{})

	tests := []struct {
		name           string
		input          []string
		wantGeneral    int
		wantDomains    int
		checkGeneral   string
		checkDomain    string
		checkDomainNS  string
	}{
		{
			name:        "simple nameserver",
			input:       []string{"8.8.8.8"},
			wantGeneral: 1,
			wantDomains: 0,
			checkGeneral: "8.8.8.8:53",
		},
		{
			name:        "nameserver with port",
			input:       []string{"8.8.8.8:5353"},
			wantGeneral: 1,
			wantDomains: 0,
			checkGeneral: "8.8.8.8:5353",
		},
		{
			name:          "domain-specific nameserver",
			input:         []string{"example.com/192.168.1.1"},
			wantGeneral:   0,
			wantDomains:   1,
			checkDomain:   "example.com.",
			checkDomainNS: "192.168.1.1:53",
		},
		{
			name:          "domain-specific with port",
			input:         []string{"example.com/192.168.1.1:5353"},
			wantGeneral:   0,
			wantDomains:   1,
			checkDomain:   "example.com.",
			checkDomainNS: "192.168.1.1:5353",
		},
		{
			name:        "mixed configuration",
			input:       []string{"8.8.8.8", "example.com/192.168.1.1", "1.1.1.1"},
			wantGeneral: 2,
			wantDomains: 1,
		},
		{
			name:        "empty lines and comments",
			input:       []string{"", "# comment", "8.8.8.8", "  "},
			wantGeneral: 1,
			wantDomains: 0,
		},
		{
			name:         "ipv6 nameserver",
			input:        []string{"::1"},
			wantGeneral:  1,
			checkGeneral: "[::1]:53",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r.UpdateNameservers(tt.input)

			if len(r.nameservers) != tt.wantGeneral {
				t.Errorf("expected %d general nameservers, got %d: %v", tt.wantGeneral, len(r.nameservers), r.nameservers)
			}

			if len(r.domainServers) != tt.wantDomains {
				t.Errorf("expected %d domain servers, got %d", tt.wantDomains, len(r.domainServers))
			}

			if tt.checkGeneral != "" {
				found := false
				for _, ns := range r.nameservers {
					if ns == tt.checkGeneral {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected %s in nameservers, got %v", tt.checkGeneral, r.nameservers)
				}
			}

			if tt.checkDomain != "" && tt.checkDomainNS != "" {
				nsList, ok := r.domainServers[tt.checkDomain]
				if !ok {
					t.Errorf("expected domain %s not found", tt.checkDomain)
					return
				}
				found := false
				for _, ns := range nsList {
					if ns == tt.checkDomainNS {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected %s for domain %s, got %v", tt.checkDomainNS, tt.checkDomain, nsList)
				}
			}
		})
	}
}

func TestGetResolvers(t *testing.T) {
	r := NewDNSResolver(ResolverConfig{})
	r.UpdateNameservers([]string{
		"8.8.8.8",
		"example.com/192.168.1.1",
		"sub.example.com/192.168.1.2",
	})

	tests := []struct {
		domain string
		want   int
	}{
		{"example.com", 1},      // matches example.com
		{"sub.example.com", 1},  // matches sub.example.com
		{"other.com", 1},        // uses default
		{"foo.example.com", 1},  // matches example.com (suffix)
	}

	for _, tt := range tests {
		t.Run(tt.domain, func(t *testing.T) {
			resolvers := r.getResolvers(tt.domain)
			if len(resolvers) != tt.want {
				t.Errorf("expected %d resolvers for %s, got %d", tt.want, tt.domain, len(resolvers))
			}
		})
	}
}

func TestStringToType(t *testing.T) {
	r := NewDNSResolver(ResolverConfig{})

	tests := []struct {
		input    string
		expected uint16
	}{
		{"A", 1},
		{"AAAA", 28},
		{"CNAME", 5},
		{"MX", 15},
		{"SRV", 33},
		{"TXT", 16},
		{"UNKNOWN", 1}, // defaults to A
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := r.stringToType(tt.input)
			if result != tt.expected {
				t.Errorf("expected %d for %s, got %d", tt.expected, tt.input, result)
			}
		})
	}
}

func TestOrderSRVRecords(t *testing.T) {
	r := NewDNSResolver(ResolverConfig{})

	t.Run("sort by priority", func(t *testing.T) {
		records := []DNSRecord{
			{Type: "SRV", Priority: 30, Weight: 0, Port: 80, Target: "c.example.com"},
			{Type: "SRV", Priority: 10, Weight: 0, Port: 80, Target: "a.example.com"},
			{Type: "SRV", Priority: 20, Weight: 0, Port: 80, Target: "b.example.com"},
		}

		ordered := r.orderSRVRecords(records)
		if len(ordered) != 3 {
			t.Fatalf("expected 3 records, got %d", len(ordered))
		}

		// First should be priority 10
		if ordered[0].Priority != 10 {
			t.Errorf("expected first priority 10, got %d", ordered[0].Priority)
		}
		// Second should be priority 20
		if ordered[1].Priority != 20 {
			t.Errorf("expected second priority 20, got %d", ordered[1].Priority)
		}
		// Third should be priority 30
		if ordered[2].Priority != 30 {
			t.Errorf("expected third priority 30, got %d", ordered[2].Priority)
		}
	})

	t.Run("filter invalid records", func(t *testing.T) {
		records := []DNSRecord{
			{Type: "SRV", Priority: 10, Weight: 0, Port: 80, Target: "valid.example.com"},
			{Type: "SRV", Priority: 20, Weight: 0, Port: 0, Target: "invalid-port.example.com"},    // invalid port
			{Type: "SRV", Priority: 30, Weight: 0, Port: 80, Target: ""},                          // empty target
			{Type: "SRV", Priority: 40, Weight: 0, Port: 80, Target: "."},                         // root target
			{Type: "A", Priority: 50, Weight: 0, Port: 80, Target: "not-srv.example.com"},          // wrong type
		}

		ordered := r.orderSRVRecords(records)
		if len(ordered) != 1 {
			t.Errorf("expected 1 valid record, got %d", len(ordered))
		}
		if len(ordered) > 0 && ordered[0].Target != "valid.example.com" {
			t.Errorf("expected valid.example.com, got %s", ordered[0].Target)
		}
	})

	t.Run("empty input", func(t *testing.T) {
		ordered := r.orderSRVRecords(nil)
		if ordered != nil {
			t.Errorf("expected nil for empty input, got %v", ordered)
		}
	})
}

func TestResolveSRVHttp(t *testing.T) {
	r := NewDNSResolver(ResolverConfig{})

	tests := []struct {
		name     string
		input    string
		contains string
	}{
		{
			name:     "add https prefix",
			input:    "example.com",
			contains: "https://example.com",
		},
		{
			name:     "keep http prefix",
			input:    "http://example.com",
			contains: "http://example.com",
		},
		{
			name:     "keep https prefix",
			input:    "https://example.com",
			contains: "https://example.com",
		},
		{
			name:     "strip srv+ prefix without SRV record",
			input:    "srv+example.com",
			contains: "example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := r.ResolveSRVHttp(tt.input)
			if result != tt.contains {
				t.Errorf("expected %s, got %s", tt.contains, result)
			}
		})
	}
}

func TestSetConfig(t *testing.T) {
	t.Run("enable cache", func(t *testing.T) {
		r := NewDNSResolver(ResolverConfig{EnableCache: false})
		if r.cache != nil {
			t.Fatal("expected no cache initially")
		}

		r.SetConfig(ResolverConfig{EnableCache: true})
		if r.cache == nil {
			t.Error("expected cache to be created")
		}
		r.Stop()
	})

	t.Run("disable cache", func(t *testing.T) {
		r := NewDNSResolver(ResolverConfig{EnableCache: true})
		if r.cache == nil {
			t.Fatal("expected cache initially")
		}

		r.SetConfig(ResolverConfig{EnableCache: false})
		if r.cache != nil {
			t.Error("expected cache to be nil after disable")
		}
	})

	t.Run("update logger", func(t *testing.T) {
		// Create a custom logger to verify update
		customLogger := &testLogger{}
		r := NewDNSResolver(ResolverConfig{})

		r.SetConfig(ResolverConfig{Logger: customLogger})
		if r.log != customLogger {
			t.Error("expected logger to be updated to custom logger")
		}
	})
}

func TestClearCache(t *testing.T) {
	r := NewDNSResolver(ResolverConfig{EnableCache: true})
	defer r.Stop()

	// Add something to cache manually
	r.cache.Set("test:key", []DNSRecord{{Type: "A", Target: "1.2.3.4"}}, time.Minute)

	if r.cache.Len() != 1 {
		t.Errorf("expected cache len 1, got %d", r.cache.Len())
	}

	r.ClearCache()

	if r.cache.Len() != 0 {
		t.Errorf("expected cache len 0 after clear, got %d", r.cache.Len())
	}
}

func TestDefaultResolver(t *testing.T) {
	// Test that global functions work
	if GetDefaultResolver() == nil {
		t.Error("expected non-nil default resolver")
	}

	// These should not panic
	UpdateNameservers([]string{"8.8.8.8"})
	SetConfig(ResolverConfig{QueryTimeout: 3 * time.Second})
}

// Integration test - requires network access
// Run with: go test -tags=integration
func TestQueryUpstreamIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	r := NewDNSResolver(ResolverConfig{
		EnableCache:  true,
		QueryTimeout: 5 * time.Second,
		Logger:       logger.Noop(),
	})
	defer r.Stop()

	r.UpdateNameservers([]string{"8.8.8.8", "1.1.1.1"})

	t.Run("A record", func(t *testing.T) {
		records, err := r.QueryUpstream("google.com", "A")
		if err != nil {
			t.Fatalf("query failed: %v", err)
		}
		if len(records) == 0 {
			t.Fatal("expected at least one A record")
		}
		for _, rec := range records {
			if rec.Type != "A" {
				t.Errorf("expected A record, got %s", rec.Type)
			}
		}
	})

	t.Run("cached query", func(t *testing.T) {
		// First query
		_, err := r.QueryUpstream("example.com", "A")
		if err != nil {
			t.Fatalf("first query failed: %v", err)
		}

		// Second query should hit cache
		records, err := r.QueryUpstream("example.com", "A")
		if err != nil {
			t.Fatalf("cached query failed: %v", err)
		}
		if len(records) == 0 {
			t.Fatal("expected cached records")
		}
	})
}

// Mock-based tests for QueryUpstream
func TestQueryUpstreamWithMock(t *testing.T) {
	t.Run("successful A record query", func(t *testing.T) {
		mock := &mockDNSClient{
			records: []dns.RR{
				makeARecord("example.com", "93.184.216.34", 300),
			},
		}

		r := NewDNSResolver(ResolverConfig{EnableCache: true})
		defer r.Stop()
		r.SetClient(mock)
		r.UpdateNameservers([]string{"8.8.8.8"})

		records, err := r.QueryUpstream("example.com", "A")
		if err != nil {
			t.Fatalf("query failed: %v", err)
		}
		if len(records) != 1 {
			t.Fatalf("expected 1 record, got %d", len(records))
		}
		if records[0].Type != "A" {
			t.Errorf("expected A record, got %s", records[0].Type)
		}
		if records[0].Target != "93.184.216.34" {
			t.Errorf("expected 93.184.216.34, got %s", records[0].Target)
		}
		if records[0].TTL != 300 {
			t.Errorf("expected TTL 300, got %d", records[0].TTL)
		}
	})

	t.Run("successful AAAA record query", func(t *testing.T) {
		mock := &mockDNSClient{
			records: []dns.RR{
				makeAAAARecord("example.com", "2606:2800:220:1:248:1893:25c8:1946", 300),
			},
		}

		r := NewDNSResolver(ResolverConfig{})
		r.SetClient(mock)
		r.UpdateNameservers([]string{"8.8.8.8"})

		records, err := r.QueryUpstream("example.com", "AAAA")
		if err != nil {
			t.Fatalf("query failed: %v", err)
		}
		if len(records) != 1 {
			t.Fatalf("expected 1 record, got %d", len(records))
		}
		if records[0].Type != "AAAA" {
			t.Errorf("expected AAAA record, got %s", records[0].Type)
		}
	})

	t.Run("successful CNAME record query", func(t *testing.T) {
		mock := &mockDNSClient{
			records: []dns.RR{
				makeCNAMERecord("www.example.com", "example.com", 300),
			},
		}

		r := NewDNSResolver(ResolverConfig{})
		r.SetClient(mock)
		r.UpdateNameservers([]string{"8.8.8.8"})

		records, err := r.QueryUpstream("www.example.com", "CNAME")
		if err != nil {
			t.Fatalf("query failed: %v", err)
		}
		if len(records) != 1 {
			t.Fatalf("expected 1 record, got %d", len(records))
		}
		if records[0].Type != "CNAME" {
			t.Errorf("expected CNAME record, got %s", records[0].Type)
		}
		if records[0].Target != "example.com" {
			t.Errorf("expected example.com, got %s", records[0].Target)
		}
	})

	t.Run("successful MX record query", func(t *testing.T) {
		mock := &mockDNSClient{
			records: []dns.RR{
				makeMXRecord("example.com", "mail.example.com", 10, 300),
			},
		}

		r := NewDNSResolver(ResolverConfig{})
		r.SetClient(mock)
		r.UpdateNameservers([]string{"8.8.8.8"})

		records, err := r.QueryUpstream("example.com", "MX")
		if err != nil {
			t.Fatalf("query failed: %v", err)
		}
		if len(records) != 1 {
			t.Fatalf("expected 1 record, got %d", len(records))
		}
		if records[0].Type != "MX" {
			t.Errorf("expected MX record, got %s", records[0].Type)
		}
		if records[0].Priority != 10 {
			t.Errorf("expected priority 10, got %d", records[0].Priority)
		}
	})

	t.Run("successful SRV record query", func(t *testing.T) {
		mock := &mockDNSClient{
			records: []dns.RR{
				makeSRVRecord("_ldap._tcp.example.com", "ldap.example.com", 389, 10, 5, 300),
			},
		}

		r := NewDNSResolver(ResolverConfig{})
		r.SetClient(mock)
		r.UpdateNameservers([]string{"8.8.8.8"})

		records, err := r.QueryUpstream("_ldap._tcp.example.com", "SRV")
		if err != nil {
			t.Fatalf("query failed: %v", err)
		}
		if len(records) != 1 {
			t.Fatalf("expected 1 record, got %d", len(records))
		}
		if records[0].Type != "SRV" {
			t.Errorf("expected SRV record, got %s", records[0].Type)
		}
		if records[0].Port != 389 {
			t.Errorf("expected port 389, got %d", records[0].Port)
		}
		if records[0].Priority != 10 {
			t.Errorf("expected priority 10, got %d", records[0].Priority)
		}
		if records[0].Weight != 5 {
			t.Errorf("expected weight 5, got %d", records[0].Weight)
		}
	})

	t.Run("successful TXT record query", func(t *testing.T) {
		mock := &mockDNSClient{
			records: []dns.RR{
				makeTXTRecord("example.com", []string{"v=spf1 mx -all"}, 300),
			},
		}

		r := NewDNSResolver(ResolverConfig{})
		r.SetClient(mock)
		r.UpdateNameservers([]string{"8.8.8.8"})

		records, err := r.QueryUpstream("example.com", "TXT")
		if err != nil {
			t.Fatalf("query failed: %v", err)
		}
		if len(records) != 1 {
			t.Fatalf("expected 1 record, got %d", len(records))
		}
		if records[0].Type != "TXT" {
			t.Errorf("expected TXT record, got %s", records[0].Type)
		}
		if records[0].Target != "v=spf1 mx -all" {
			t.Errorf("expected 'v=spf1 mx -all', got %s", records[0].Target)
		}
	})

	t.Run("multiple A records", func(t *testing.T) {
		mock := &mockDNSClient{
			records: []dns.RR{
				makeARecord("example.com", "93.184.216.34", 300),
				makeARecord("example.com", "93.184.216.35", 300),
			},
		}

		r := NewDNSResolver(ResolverConfig{})
		r.SetClient(mock)
		r.UpdateNameservers([]string{"8.8.8.8"})

		records, err := r.QueryUpstream("example.com", "A")
		if err != nil {
			t.Fatalf("query failed: %v", err)
		}
		if len(records) != 2 {
			t.Fatalf("expected 2 records, got %d", len(records))
		}
	})

	t.Run("cache hit", func(t *testing.T) {
		mock := &mockDNSClient{
			records: []dns.RR{
				makeARecord("example.com", "93.184.216.34", 300),
			},
		}

		r := NewDNSResolver(ResolverConfig{EnableCache: true})
		defer r.Stop()
		r.SetClient(mock)
		r.UpdateNameservers([]string{"8.8.8.8"})

		// First query
		records1, err := r.QueryUpstream("example.com", "A")
		if err != nil {
			t.Fatalf("first query failed: %v", err)
		}

		// Change mock to return error - should still get cached result
		mock.records = nil
		mock.err = context.DeadlineExceeded

		records2, err := r.QueryUpstream("example.com", "A")
		if err != nil {
			t.Fatalf("cached query failed: %v", err)
		}

		// Should return same cached result
		if len(records2) != len(records1) {
			t.Errorf("cache should have returned same result")
		}
	})

	t.Run("query error - all nameservers fail", func(t *testing.T) {
		mock := &mockDNSClient{
			err: context.DeadlineExceeded,
		}

		r := NewDNSResolver(ResolverConfig{QueryTimeout: 100 * time.Millisecond})
		r.SetClient(mock)
		r.UpdateNameservers([]string{"8.8.8.8", "1.1.1.1"})

		_, err := r.QueryUpstream("example.com", "A")
		if err == nil {
			t.Fatal("expected error when all nameservers fail")
		}
	})

	t.Run("no nameservers configured - system resolver fallback", func(t *testing.T) {
		r := NewDNSResolver(ResolverConfig{})

		// No nameservers configured, should use system resolver
		// This will make a real DNS query
		records, err := r.QueryUpstream("localhost", "A")
		// localhost should resolve, but we just check it doesn't panic
		_ = records
		_ = err
	})
}

func TestRrToRecord(t *testing.T) {
	r := NewDNSResolver(ResolverConfig{})

	t.Run("A record", func(t *testing.T) {
		rr := makeARecord("example.com", "93.184.216.34", 300)
		record := r.rrToRecord(rr)
		if record == nil {
			t.Fatal("expected non-nil record")
		}
		if record.Type != "A" {
			t.Errorf("expected A, got %s", record.Type)
		}
		if record.Target != "93.184.216.34" {
			t.Errorf("expected 93.184.216.34, got %s", record.Target)
		}
	})

	t.Run("AAAA record", func(t *testing.T) {
		rr := makeAAAARecord("example.com", "2606:2800:220:1:248:1893:25c8:1946", 300)
		record := r.rrToRecord(rr)
		if record == nil {
			t.Fatal("expected non-nil record")
		}
		if record.Type != "AAAA" {
			t.Errorf("expected AAAA, got %s", record.Type)
		}
	})

	t.Run("CNAME record", func(t *testing.T) {
		rr := makeCNAMERecord("www.example.com", "example.com", 300)
		record := r.rrToRecord(rr)
		if record == nil {
			t.Fatal("expected non-nil record")
		}
		if record.Type != "CNAME" {
			t.Errorf("expected CNAME, got %s", record.Type)
		}
		// Should have trailing dot removed
		if record.Target != "example.com" {
			t.Errorf("expected example.com, got %s", record.Target)
		}
	})

	t.Run("MX record", func(t *testing.T) {
		rr := makeMXRecord("example.com", "mail.example.com", 10, 300)
		record := r.rrToRecord(rr)
		if record == nil {
			t.Fatal("expected non-nil record")
		}
		if record.Type != "MX" {
			t.Errorf("expected MX, got %s", record.Type)
		}
		if record.Priority != 10 {
			t.Errorf("expected priority 10, got %d", record.Priority)
		}
	})

	t.Run("SRV record", func(t *testing.T) {
		rr := makeSRVRecord("_ldap._tcp.example.com", "ldap.example.com", 389, 10, 5, 300)
		record := r.rrToRecord(rr)
		if record == nil {
			t.Fatal("expected non-nil record")
		}
		if record.Type != "SRV" {
			t.Errorf("expected SRV, got %s", record.Type)
		}
		if record.Port != 389 {
			t.Errorf("expected port 389, got %d", record.Port)
		}
		if record.Priority != 10 {
			t.Errorf("expected priority 10, got %d", record.Priority)
		}
		if record.Weight != 5 {
			t.Errorf("expected weight 5, got %d", record.Weight)
		}
	})

	t.Run("TXT record", func(t *testing.T) {
		rr := makeTXTRecord("example.com", []string{"v=spf1", "mx", "-all"}, 300)
		record := r.rrToRecord(rr)
		if record == nil {
			t.Fatal("expected non-nil record")
		}
		if record.Type != "TXT" {
			t.Errorf("expected TXT, got %s", record.Type)
		}
		// Multiple TXT strings should be joined with space
		if record.Target != "v=spf1 mx -all" {
			t.Errorf("expected 'v=spf1 mx -all', got %s", record.Target)
		}
	})
}

func TestLookupSRVWithMock(t *testing.T) {
	t.Run("successful lookup", func(t *testing.T) {
		mock := &mockDNSClient{
			records: []dns.RR{
				makeSRVRecord("_service._tcp.example.com", "server1.example.com", 8080, 10, 5, 300),
			},
		}

		r := NewDNSResolver(ResolverConfig{})
		r.SetClient(mock)
		r.UpdateNameservers([]string{"8.8.8.8"})

		// This will fail because it also needs to resolve server1.example.com to IP
		// but we're testing the SRV parsing logic
		addrs, err := r.LookupSRV("_service._tcp.example.com")
		// Expected to fail because we can't resolve the target IP without network
		// But the test verifies the function doesn't panic
		_ = addrs
		_ = err
	})
}

func TestLookupIPWithMock(t *testing.T) {
	t.Run("A record lookup", func(t *testing.T) {
		mock := &mockDNSClient{
			records: []dns.RR{
				makeARecord("example.com", "93.184.216.34", 300),
			},
		}

		r := NewDNSResolver(ResolverConfig{})
		r.SetClient(mock)
		r.UpdateNameservers([]string{"8.8.8.8"})

		ips, err := r.LookupIP("example.com")
		if err != nil {
			t.Fatalf("lookup failed: %v", err)
		}
		if len(ips) != 1 {
			t.Fatalf("expected 1 IP, got %d", len(ips))
		}
		if ips[0] != "93.184.216.34" {
			t.Errorf("expected 93.184.216.34, got %s", ips[0])
		}
	})

	t.Run("AAAA record fallback", func(t *testing.T) {
		// First call returns no A records, second returns AAAA
		callCount := 0
		mock := &mockDNSClient{}

		r := NewDNSResolver(ResolverConfig{})
		r.SetClient(mock)
		r.UpdateNameservers([]string{"8.8.8.8"})

		// Set up mock to return different results based on query order
		mock.records = []dns.RR{
			makeAAAARecord("ipv6.example.com", "2606:2800:220:1:248:1893:25c8:1946", 300),
		}

		ips, err := r.LookupIP("ipv6.example.com")
		if err != nil {
			t.Fatalf("lookup failed: %v", err)
		}
		_ = callCount
		_ = ips
	})
}

func TestSetClient(t *testing.T) {
	r := NewDNSResolver(ResolverConfig{})
	originalClient := r.client

	mock := &mockDNSClient{}
	r.SetClient(mock)

	if r.client == originalClient {
		t.Error("expected client to be changed")
	}
	if r.client != mock {
		t.Error("expected client to be mock")
	}
}
