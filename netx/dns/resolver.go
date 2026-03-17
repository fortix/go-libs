// Package dns provides a DNS resolver with caching, parallel queries,
// and support for domain-specific nameservers.
package dns

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fortix/go-libs/cache"
	"github.com/fortix/go-libs/logger"
	"github.com/miekg/dns"
)

// dnsClient is an interface for DNS client operations, allowing mocking in tests.
type dnsClient interface {
	Exchange(ctx context.Context, msg *dns.Msg, nameserver string) (*dns.Msg, error)
}

// miekgClient wraps the miekg/dns Client to implement dnsClient.
type miekgClient struct {
	timeout time.Duration
}

func (c *miekgClient) Exchange(ctx context.Context, msg *dns.Msg, nameserver string) (*dns.Msg, error) {
	// Try UDP first
	client := &dns.Client{
		Net:     "udp",
		Timeout: c.timeout,
	}

	response, _, err := client.ExchangeContext(ctx, msg, nameserver)
	if err == nil && response != nil {
		// Check if truncated - if so, retry with TCP
		if response.Truncated {
			client.Net = "tcp"
			response, _, err = client.ExchangeContext(ctx, msg, nameserver)
		}
		if err == nil && response != nil {
			return response, nil
		}
	}

	// If UDP failed or context cancelled, try TCP as fallback
	if ctx.Err() == nil {
		client.Net = "tcp"
		response, _, err = client.ExchangeContext(ctx, msg, nameserver)
		if err == nil && response != nil {
			return response, nil
		}
	}

	return nil, errors.New("dns query failed")
}

// ResolverConfig configures the DNS resolver behavior.
type ResolverConfig struct {
	// QueryTimeout is the timeout for upstream DNS queries.
	// 0 uses the default of 2 seconds.
	QueryTimeout time.Duration

	// EnableCache enables caching of DNS responses.
	EnableCache bool

	// MaxCacheTTL is the maximum TTL for cache entries in seconds.
	// 0 means unlimited.
	MaxCacheTTL int

	// Logger is an optional logger for debug output.
	// If nil, a no-op logger is used.
	Logger logger.Logger
}

// DNSRecord represents a parsed DNS record.
type DNSRecord struct {
	Type     string // "A", "AAAA", "CNAME", "MX", "SRV", "TXT"
	Name     string // fully qualified domain name (normalized)
	Target   string // IP for A/AAAA, FQDN for CNAME/MX/SRV, value for TXT
	Port     int    // only for SRV records
	Priority int    // for SRV and MX records
	Weight   int    // only for SRV records
	TTL      int    // time to live in seconds
}

// DNSResolver is a DNS resolver with caching and parallel query support.
type DNSResolver struct {
	config        ResolverConfig
	nameservers   []string            // General nameservers
	domainServers map[string][]string // Domain-specific nameservers
	cache         *cache.Cache[[]DNSRecord]
	client        dnsClient // DNS client for queries
	mu            sync.RWMutex
	log           logger.Logger
}

// NewDNSResolver creates a new DNS resolver with the given configuration.
func NewDNSResolver(config ResolverConfig) *DNSResolver {
	if config.QueryTimeout == 0 {
		config.QueryTimeout = 2 * time.Second
	}

	log := config.Logger
	if log == nil {
		log = logger.Noop()
	}

	resolver := &DNSResolver{
		config:        config,
		nameservers:   make([]string, 0),
		domainServers: make(map[string][]string),
		client:        &miekgClient{timeout: config.QueryTimeout},
		log:           log,
	}

	// Initialize cache if enabled
	if config.EnableCache {
		cleanupInterval := 5 * time.Minute
		maxTTL := time.Duration(config.MaxCacheTTL) * time.Second
		if maxTTL > 0 && maxTTL < cleanupInterval {
			cleanupInterval = maxTTL
		}
		if cleanupInterval < 30*time.Second {
			cleanupInterval = 30 * time.Second
		}

		resolver.cache = cache.New[[]DNSRecord](cache.Options{
			MaxTTL:         maxTTL,
			DefaultTTL:     60 * time.Second,
			CleanupInterval: cleanupInterval,
		})
	}

	return resolver
}

// UpdateNameservers updates the resolver's nameservers.
// Format:
//
//	nameserver         -> default nameserver, port 53
//	nameserver:port    -> default nameserver with custom port
//	domain/nameserver  -> domain-specific nameserver, port 53
//	domain/nameserver:port -> domain-specific nameserver with custom port
func (r *DNSResolver) UpdateNameservers(nameservers []string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Clear existing configuration
	r.nameservers = make([]string, 0)
	r.domainServers = make(map[string][]string)

	// Process nameservers
	for _, ns := range nameservers {
		ns = strings.TrimSpace(ns)
		if ns == "" || strings.HasPrefix(ns, "#") {
			continue // Skip empty lines and comments
		}

		if strings.Contains(ns, "/") {
			// Domain-specific nameserver
			parts := strings.SplitN(ns, "/", 2)
			if len(parts) != 2 {
				continue // Skip invalid entries
			}

			domain := parts[0]
			nameserver := parts[1]

			// Ensure domain ends with dot
			if !strings.HasSuffix(domain, ".") {
				domain = domain + "."
			}

			// Add port if not specified
			if _, _, err := net.SplitHostPort(nameserver); err != nil {
				nameserver = net.JoinHostPort(nameserver, "53")
			}

			// Add to domain servers
			if _, exists := r.domainServers[domain]; !exists {
				r.domainServers[domain] = make([]string, 0)
			}
			r.domainServers[domain] = append(r.domainServers[domain], nameserver)
		} else {
			// Default nameserver
			nameserver := ns

			// Add port if not specified
			if _, _, err := net.SplitHostPort(nameserver); err != nil {
				nameserver = net.JoinHostPort(nameserver, "53")
			}

			r.nameservers = append(r.nameservers, nameserver)
		}
	}

	r.ClearCache()

	// Log the resulting configuration for diagnostics
	if len(r.domainServers) > 0 || len(r.nameservers) > 0 {
		var general []string
		general = append(general, r.nameservers...)
		for domain, nss := range r.domainServers {
			r.log.Debug("configured domain-specific nameservers", "domain", domain, "nameservers", nss)
		}
		if len(general) > 0 {
			r.log.Debug("configured default nameservers", "nameservers", general)
		}
	}
}

// SetConfig updates the resolver configuration.
// Note: Changing EnableCache requires calling this before UpdateNameservers.
func (r *DNSResolver) SetConfig(newConfig ResolverConfig) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Handle cache changes
	if r.config.EnableCache && !newConfig.EnableCache {
		// Cache was enabled, now disabled
		if r.cache != nil {
			r.cache.Stop()
			r.cache = nil
		}
	} else if !r.config.EnableCache && newConfig.EnableCache {
		// Cache was disabled, now enabled
		cleanupInterval := 5 * time.Minute
		maxTTL := time.Duration(newConfig.MaxCacheTTL) * time.Second
		if maxTTL > 0 && maxTTL < cleanupInterval {
			cleanupInterval = maxTTL
		}
		if cleanupInterval < 30*time.Second {
			cleanupInterval = 30 * time.Second
		}

		r.cache = cache.New[[]DNSRecord](cache.Options{
			MaxTTL:         maxTTL,
			DefaultTTL:     60 * time.Second,
			CleanupInterval: cleanupInterval,
		})
	}

	if newConfig.Logger != nil {
		r.log = newConfig.Logger
	}
	if newConfig.QueryTimeout > 0 {
		r.config.QueryTimeout = newConfig.QueryTimeout
	}
	r.config.EnableCache = newConfig.EnableCache
	r.config.MaxCacheTTL = newConfig.MaxCacheTTL
}

// SetClient sets a custom DNS client. This is primarily intended for testing
// with mock implementations.
func (r *DNSResolver) SetClient(client dnsClient) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.client = client
}

func (r *DNSResolver) getResolvers(record string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if !strings.HasSuffix(record, ".") {
		record = record + "."
	}

	// Look through the domains map to see if we have specific servers for this domain
	for domain, ns := range r.domainServers {
		if strings.HasSuffix(record, domain) {
			return ns
		}
	}

	// If no specific servers are found, use the default servers
	if len(r.nameservers) == 0 {
		return nil
	}
	return r.nameservers
}

// QueryUpstream queries the upstream resolver for records using parallel DNS forwarding.
func (r *DNSResolver) QueryUpstream(name string, recordType string) ([]DNSRecord, error) {
	cacheKey := fmt.Sprintf("%s:%s", name, recordType)

	// Check cache first
	if r.config.EnableCache && r.cache != nil {
		if records, found := r.cache.Get(cacheKey); found {
			return records, nil
		}
	}

	// Get nameservers for this query
	nameservers := r.getResolvers(name)
	r.log.Debug("upstream query", "query", name, "type", recordType, "nameservers", nameservers)
	if len(nameservers) == 0 {
		// Use system resolver
		return r.querySystemResolver(name, recordType)
	}

	// Create DNS query message
	msg := new(dns.Msg)
	msg.SetQuestion(dns.Fqdn(name), r.stringToType(recordType))
	msg.RecursionDesired = true

	// Use context for cancellation
	ctx, cancel := context.WithTimeout(context.Background(), r.config.QueryTimeout)
	defer cancel()

	// Response channel - buffered to prevent goroutine leaks
	respChan := make(chan *dns.Msg, 1) // Only need 1 success
	errChan := make(chan error, len(nameservers))

	var wg sync.WaitGroup

	// Query all nameservers in parallel
	for _, nameserver := range nameservers {
		wg.Add(1)

		go func(ns string) {
			defer wg.Done()

			response := r.queryNameserver(ctx, msg, ns)
			if response != nil && response.Rcode == dns.RcodeSuccess && len(response.Answer) > 0 {
				select {
				case respChan <- response:
					cancel() // Cancel other queries on first success
				case <-ctx.Done():
					// Context already cancelled
				}
			} else {
				select {
				case errChan <- fmt.Errorf("nameserver %s returned no valid response", ns):
				case <-ctx.Done():
					// Context already cancelled
				}
			}
		}(nameserver)
	}

	// Close channels when all goroutines complete
	go func() {
		wg.Wait()
		close(respChan)
		close(errChan)
	}()

	// Wait for first success or all failures
	select {
	case response, ok := <-respChan:
		if !ok {
			// Channel closed, no successful response
			break
		}
		// Success - convert DNS RRs to our internal format
		var results []DNSRecord
		for _, rr := range response.Answer {
			if record := r.rrToRecord(rr); record != nil {
				results = append(results, *record)
			}
		}

		// Cache the results
		if r.config.EnableCache && r.cache != nil {
			// Determine TTL for caching
			minTTL := 0
			for _, rec := range results {
				if rec.TTL > 0 && (minTTL == 0 || rec.TTL < minTTL) {
					minTTL = rec.TTL
				}
			}
			r.cache.Set(cacheKey, results, time.Duration(minTTL)*time.Second)
		}
		return results, nil

	case <-ctx.Done():
		// Context timeout/cancellation
	}

	// Collect all errors
	var errs []error
	for err := range errChan {
		errs = append(errs, err)
	}

	if len(errs) == 0 {
		errs = append(errs, fmt.Errorf("query timeout after %v", r.config.QueryTimeout))
	}

	return nil, fmt.Errorf("all nameservers failed: %w", errors.Join(errs...))
}

// queryNameserver queries a single nameserver using the dnsClient interface.
func (r *DNSResolver) queryNameserver(ctx context.Context, msg *dns.Msg, nameserver string) *dns.Msg {
	response, err := r.client.Exchange(ctx, msg, nameserver)
	if err != nil || response == nil {
		return nil
	}
	return response
}

// rrToRecord converts a DNS RR to our internal DNSRecord format.
func (r *DNSResolver) rrToRecord(rr dns.RR) *DNSRecord {
	header := rr.Header()
	record := &DNSRecord{
		Name: header.Name,
		TTL:  int(header.Ttl),
	}

	switch v := rr.(type) {
	case *dns.A:
		record.Type = "A"
		record.Target = v.A.String()
		return record

	case *dns.AAAA:
		record.Type = "AAAA"
		record.Target = v.AAAA.String()
		return record

	case *dns.CNAME:
		record.Type = "CNAME"
		record.Target = strings.TrimSuffix(v.Target, ".")
		return record

	case *dns.MX:
		record.Type = "MX"
		record.Target = strings.TrimSuffix(v.Mx, ".")
		record.Priority = int(v.Preference)
		return record

	case *dns.SRV:
		record.Type = "SRV"
		record.Target = strings.TrimSuffix(v.Target, ".")
		record.Port = int(v.Port)
		record.Priority = int(v.Priority)
		record.Weight = int(v.Weight)
		return record

	case *dns.TXT:
		record.Type = "TXT"
		if len(v.Txt) > 0 {
			record.Target = strings.Join(v.Txt, " ")
		}
		return record

	default:
		// Unsupported record type
		return nil
	}
}

// stringToType converts string type to DNS type constant.
func (r *DNSResolver) stringToType(recordType string) uint16 {
	switch recordType {
	case "A":
		return dns.TypeA
	case "AAAA":
		return dns.TypeAAAA
	case "CNAME":
		return dns.TypeCNAME
	case "MX":
		return dns.TypeMX
	case "SRV":
		return dns.TypeSRV
	case "TXT":
		return dns.TypeTXT
	default:
		return dns.TypeA
	}
}

// querySystemResolver uses the Go net.Resolver as a fallback.
func (r *DNSResolver) querySystemResolver(name string, recordType string) ([]DNSRecord, error) {
	ctx, cancel := context.WithTimeout(context.Background(), r.config.QueryTimeout)
	defer cancel()
	var records []DNSRecord
	var err error
	resolver := net.Resolver{}

	switch recordType {
	case "A":
		var addrs []string
		addrs, err = resolver.LookupHost(ctx, name)
		for _, addr := range addrs {
			if net.ParseIP(addr) != nil && !strings.Contains(addr, ":") {
				records = append(records, DNSRecord{Type: "A", Name: name, Target: addr, TTL: 300})
			}
		}
	case "AAAA":
		var addrs []string
		addrs, err = resolver.LookupHost(ctx, name)
		for _, addr := range addrs {
			if net.ParseIP(addr) != nil && strings.Contains(addr, ":") {
				records = append(records, DNSRecord{Type: "AAAA", Name: name, Target: addr, TTL: 300})
			}
		}
	case "CNAME":
		var cname string
		cname, err = resolver.LookupCNAME(ctx, name)
		if err == nil {
			records = append(records, DNSRecord{Type: "CNAME", Name: name, Target: cname, TTL: 300})
		}
	case "TXT":
		var txts []string
		txts, err = resolver.LookupTXT(ctx, name)
		for _, txt := range txts {
			records = append(records, DNSRecord{Type: "TXT", Name: name, Target: txt, TTL: 300})
		}
	case "MX":
		var mxs []*net.MX
		mxs, err = resolver.LookupMX(ctx, name)
		for _, mx := range mxs {
			records = append(records, DNSRecord{Type: "MX", Name: name, Target: mx.Host, Priority: int(mx.Pref), TTL: 300})
		}
	case "SRV":
		var srvs []*net.SRV
		_, srvs, err = resolver.LookupSRV(ctx, "", "", name)
		for _, srv := range srvs {
			records = append(records, DNSRecord{
				Type:     "SRV",
				Name:     name,
				Target:   srv.Target,
				Port:     int(srv.Port),
				Priority: int(srv.Priority),
				Weight:   int(srv.Weight),
				TTL:      300,
			})
		}
	default:
		return nil, fmt.Errorf("system resolver fallback does not support type %s", recordType)
	}

	if err != nil {
		return nil, err
	}
	return records, nil
}

// ClearCache clears the DNS cache.
func (r *DNSResolver) ClearCache() {
	if r.cache != nil {
		r.cache.Clear()
	}
}

// Stop stops the resolver and cleans up resources.
func (r *DNSResolver) Stop() {
	if r.cache != nil {
		r.cache.Stop()
	}
}

// LookupSRV looks up SRV records and returns TCP addresses.
func (r *DNSResolver) LookupSRV(service string) ([]*net.TCPAddr, error) {
	records, err := r.QueryUpstream(service, "SRV")
	if err != nil {
		return nil, err
	}

	// Order SRV records by RFC 2782: lowest priority first, then weighted selection
	ordered := r.orderSRVRecords(records)

	var tcpAddrs []*net.TCPAddr
	for _, rec := range ordered {
		// Look up IPs for the target of this SRV record
		ips, err := r.LookupIP(rec.Target)
		if err != nil {
			continue
		}
		for _, ip := range ips {
			if parsedIP := net.ParseIP(ip); parsedIP != nil {
				tcpAddrs = append(tcpAddrs, &net.TCPAddr{IP: parsedIP, Port: rec.Port})
			}
		}
	}

	if len(tcpAddrs) == 0 {
		return nil, errors.New("no such host")
	}

	return tcpAddrs, nil
}

// LookupIP looks up IP addresses for a host.
func (r *DNSResolver) LookupIP(host string) ([]string, error) {
	var lastErr error

	// Try A records first
	records, err := r.QueryUpstream(host, "A")
	if err == nil && len(records) > 0 {
		var ips []string
		for _, record := range records {
			if record.Type == "A" {
				ips = append(ips, record.Target)
			}
		}
		if len(ips) > 0 {
			return ips, nil
		}
	} else {
		lastErr = err
	}

	// Try AAAA records
	records, err = r.QueryUpstream(host, "AAAA")
	if err == nil && len(records) > 0 {
		var ips []string
		for _, record := range records {
			if record.Type == "AAAA" {
				ips = append(ips, record.Target)
			}
		}
		if len(ips) > 0 {
			return ips, nil
		}
	} else if lastErr == nil {
		lastErr = err
	}

	if lastErr != nil {
		return nil, fmt.Errorf("failed to resolve %s: %w", host, lastErr)
	}
	return nil, fmt.Errorf("no IP addresses found for %s", host)
}

// ResolveSRVHttp resolves a URL with SRV record lookup.
// If the URL starts with "srv+" or "SRV+", it performs SRV lookup
// to get the port while preserving the hostname for SNI.
func (r *DNSResolver) ResolveSRVHttp(uri string) string {
	// If url starts with srv+ then remove it and resolve the actual url
	if strings.HasPrefix(uri, "srv+") || strings.HasPrefix(uri, "SRV+") {

		// Parse the url excluding the srv+ prefix
		u, err := url.Parse(uri[4:])
		if err != nil {
			return uri[4:]
		}

		// Query SRV directly to obtain the service-selected port while preserving hostname for SNI
		srvRecords, err := r.QueryUpstream(u.Host, "SRV")
		if err != nil || len(srvRecords) == 0 {
			return uri[4:]
		}
		ordered := r.orderSRVRecords(srvRecords)
		if len(ordered) == 0 {
			return uri[4:]
		}
		port := ordered[0].Port
		if port <= 0 {
			return uri[4:]
		}

		// Keep original hostname, only set the port
		host := net.JoinHostPort(u.Hostname(), strconv.Itoa(port))
		u.Host = host
		return u.String()
	}

	if !strings.HasPrefix(uri, "http://") && !strings.HasPrefix(uri, "https://") {
		return "https://" + uri
	}

	return uri
}

// orderSRVRecords filters invalid SRV records and returns them ordered by RFC 2782:
// ascending priority, and within the same priority group, a weighted-shuffle by Weight.
func (r *DNSResolver) orderSRVRecords(records []DNSRecord) []DNSRecord {
	// Filter valid SRV records
	var srvs []DNSRecord
	for _, rec := range records {
		if rec.Type != "SRV" {
			continue
		}
		if rec.Port <= 0 {
			continue
		}
		if rec.Target == "" || rec.Target == "." {
			continue
		}
		srvs = append(srvs, rec)
	}
	if len(srvs) == 0 {
		return nil
	}

	// Collect unique priorities
	prioSet := map[int]struct{}{}
	for _, s := range srvs {
		prioSet[s.Priority] = struct{}{}
	}
	// Extract and sort priorities ascending
	priorities := make([]int, 0, len(prioSet))
	for p := range prioSet {
		priorities = append(priorities, p)
	}
	// Simple insertion sort to avoid importing sort
	for i := 1; i < len(priorities); i++ {
		key := priorities[i]
		j := i - 1
		for j >= 0 && priorities[j] > key {
			priorities[j+1] = priorities[j]
			j--
		}
		priorities[j+1] = key
	}

	// Weighted shuffle within each priority
	rnd := rand.New(rand.NewSource(time.Now().UnixNano()))
	var ordered []DNSRecord
	for _, p := range priorities {
		// Gather candidates of this priority
		var candidates []DNSRecord
		for _, s := range srvs {
			if s.Priority == p {
				candidates = append(candidates, s)
			}
		}
		// Repeatedly pick one using weight
		for len(candidates) > 0 {
			total := 0
			for _, c := range candidates {
				if c.Weight > 0 {
					total += c.Weight
				}
			}
			var idx int
			if total == 0 {
				// All zero weights: pick uniformly
				idx = rnd.Intn(len(candidates))
			} else {
				pick := rnd.Intn(total)
				sum := 0
				for i, c := range candidates {
					w := c.Weight
					if w < 0 {
						w = 0
					}
					sum += w
					if pick < sum {
						idx = i
						break
					}
				}
			}
			ordered = append(ordered, candidates[idx])
			// Remove chosen idx
			candidates = append(candidates[:idx], candidates[idx+1:]...)
		}
	}

	return ordered
}

// Default global resolver instance
var defaultResolver = NewDNSResolver(ResolverConfig{})

// Global convenience functions that use the default resolver

// UpdateNameservers updates the default resolver's nameservers.
func UpdateNameservers(nameservers []string) {
	defaultResolver.UpdateNameservers(nameservers)
}

// SetConfig updates the default resolver's configuration.
func SetConfig(newConfig ResolverConfig) {
	defaultResolver.SetConfig(newConfig)
}

// LookupSRV looks up SRV records using the default resolver.
func LookupSRV(service string) ([]*net.TCPAddr, error) {
	return defaultResolver.LookupSRV(service)
}

// LookupIP looks up IP addresses using the default resolver.
func LookupIP(host string) ([]string, error) {
	return defaultResolver.LookupIP(host)
}

// ResolveSRVHttp resolves a URL with SRV lookup using the default resolver.
func ResolveSRVHttp(uri string) string {
	return defaultResolver.ResolveSRVHttp(uri)
}

// GetDefaultResolver returns the default resolver instance.
func GetDefaultResolver() *DNSResolver {
	return defaultResolver
}
