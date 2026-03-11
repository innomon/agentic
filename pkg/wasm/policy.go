package wasm

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/tetratelabs/wazero"
)

type SecurityPolicy struct {
	AllowedPaths   []string `yaml:"allowed_paths"`
	AllowedDomains []string `yaml:"allowed_domains"`
	MemoryMaxPages uint32   `yaml:"memory_max_pages"`
}

const defaultMemoryMaxPages = 256 // 16MB

func (p *SecurityPolicy) Validate() error {
	for _, path := range p.AllowedPaths {
		if !filepath.IsAbs(path) {
			return fmt.Errorf("allowed_paths must be absolute: %q", path)
		}
	}
	return nil
}

func (p *SecurityPolicy) FSConfig() wazero.FSConfig {
	fs := wazero.NewFSConfig()
	for _, path := range p.AllowedPaths {
		cleaned := filepath.Clean(path)
		fs = fs.WithReadOnlyDirMount(cleaned, cleaned)
	}
	return fs
}

func (p *SecurityPolicy) EffectiveMemoryMaxPages() uint32 {
	if p.MemoryMaxPages > 0 {
		return p.MemoryMaxPages
	}
	return defaultMemoryMaxPages
}

func (p *SecurityPolicy) CheckDomain(rawURL string) error {
	if len(p.AllowedDomains) == 0 {
		return fmt.Errorf("network access denied: no domains allowed")
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return fmt.Errorf("unsupported scheme %q: only http/https allowed", parsed.Scheme)
	}

	host := parsed.Hostname()
	if host == "" {
		return fmt.Errorf("empty host in URL")
	}

	// Block loopback/private addresses
	if host == "localhost" || host == "127.0.0.1" || host == "::1" ||
		strings.HasPrefix(host, "10.") || strings.HasPrefix(host, "192.168.") {
		return fmt.Errorf("access to private/loopback addresses denied: %s", host)
	}

	for _, allowed := range p.AllowedDomains {
		if host == allowed {
			return nil
		}
		// Support wildcard subdomain matching: *.example.com
		if strings.HasPrefix(allowed, "*.") {
			suffix := allowed[1:] // ".example.com"
			if strings.HasSuffix(host, suffix) || host == allowed[2:] {
				return nil
			}
		}
	}

	return fmt.Errorf("domain %q not in allowed list", host)
}
