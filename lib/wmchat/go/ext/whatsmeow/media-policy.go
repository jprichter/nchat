// Copyright (c) 2026 nchat contributors
// SPDX-License-Identifier: MPL-2.0

package whatsmeow

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"
)

var errUnsafeMediaURL = errors.New("media destination is not an approved HTTPS WhatsApp CDN")
var errMediaProxy = errors.New("media downloads through proxies are unsupported by the DNS safety policy; refusing a direct connection")

// The upstream media_conn response advertises mmg.whatsapp.net and regional
// media-*.cdn.whatsapp.net hosts. FetchStickerPack uses static.whatsapp.net.
// See doc/PRIVACY.md for evidence and the deliberate compatibility boundary.
func validateMediaURL(u *url.URL) error {
	if u == nil || u.Scheme != "https" || u.User != nil || u.Opaque != "" || u.Fragment != "" || strings.Contains(u.Host, ":") {
		return errUnsafeMediaURL
	}
	host := strings.ToLower(u.Hostname())
	if host == "mmg.whatsapp.net" || host == "static.whatsapp.net" {
		return nil
	}
	if strings.HasPrefix(host, "media-") && strings.HasSuffix(host, ".cdn.whatsapp.net") {
		label := strings.TrimSuffix(host, ".cdn.whatsapp.net")
		if len(label) <= 63 && len(label) > len("media-") && !strings.HasSuffix(label, "-") {
			for _, c := range label {
				if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '-' {
					return errUnsafeMediaURL
				}
			}
			return nil
		}
	}
	return errUnsafeMediaURL
}

func publicMediaIP(ip netip.Addr) bool {
	ip = ip.Unmap()
	return ip.IsGlobalUnicast() && !ip.IsPrivate() && !ip.IsLoopback() && !ip.IsLinkLocalUnicast() &&
		!netip.MustParsePrefix("100.64.0.0/10").Contains(ip) &&
		!netip.MustParsePrefix("192.0.0.0/24").Contains(ip) &&
		!netip.MustParsePrefix("192.0.2.0/24").Contains(ip) &&
		!netip.MustParsePrefix("198.18.0.0/15").Contains(ip) &&
		!netip.MustParsePrefix("198.51.100.0/24").Contains(ip) &&
		!netip.MustParsePrefix("203.0.113.0/24").Contains(ip) &&
		(!ip.Is6() || netip.MustParsePrefix("2000::/3").Contains(ip)) &&
		!netip.MustParsePrefix("2001::/23").Contains(ip) &&
		!netip.MustParsePrefix("2001:db8::/32").Contains(ip) &&
		!netip.MustParsePrefix("2002::/16").Contains(ip)
}

type mediaResolver interface {
	LookupNetIP(context.Context, string, string) ([]netip.Addr, error)
}

// Resolve once, reject mixed public/private answers, and dial the checked IP
// directly. TLS still authenticates the original hostname. Neither a second DNS
// lookup nor a proxy/custom TLS dialer can bypass this check.
func dialMedia(ctx context.Context, network, address string, resolver mediaResolver, dial func(context.Context, string, string) (net.Conn, error)) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil || port != "443" || validateMediaURL(&url.URL{Scheme: "https", Host: host}) != nil {
		return nil, errUnsafeMediaURL
	}
	ips, err := resolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return nil, errors.New("media DNS lookup failed")
	}
	if len(ips) == 0 {
		return nil, errors.New("media DNS returned no addresses")
	}
	for _, ip := range ips {
		if !publicMediaIP(ip) {
			return nil, errors.New("media DNS returned a non-public address")
		}
	}
	for _, ip := range ips {
		conn, dialErr := dial(ctx, network, net.JoinHostPort(ip.String(), port))
		if dialErr == nil {
			return conn, nil
		}
		err = dialErr
	}
	return nil, err
}

// Custom RoundTrippers are trusted application code (also useful for tests).
// URL validation still runs before invoking them, including on redirects.
func mediaDownloadClient(base *http.Client) *http.Client {
	client := *base
	transport := client.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	if original, ok := transport.(*http.Transport); ok {
		safe := original.Clone()
		if original.Proxy != nil {
			safe.Proxy = func(req *http.Request) (*url.URL, error) {
				proxyURL, err := original.Proxy(req)
				if err != nil {
					return nil, errMediaProxy
				}
				if proxyURL != nil {
					return nil, errMediaProxy
				}
				return nil, nil
			}
		}
		safe.DialTLS = nil
		safe.DialTLSContext = nil
		safe.DisableKeepAlives = true
		dialer := &net.Dialer{Timeout: 30 * time.Second}
		safe.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
			return dialMedia(ctx, network, address, net.DefaultResolver, dialer.DialContext)
		}
		transport = safe
	}
	client.Transport = mediaPolicyTransport{transport}
	previous := client.CheckRedirect
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if err := validateMediaURL(req.URL); err != nil {
			return err
		}
		if len(via) >= 10 {
			return errors.New("too many media redirects")
		}
		if previous != nil {
			return previous(req, via)
		}
		return nil
	}
	return &client
}

type mediaPolicyTransport struct{ http.RoundTripper }

func (t mediaPolicyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if err := validateMediaURL(req.URL); err != nil {
		return nil, err
	}
	return t.RoundTripper.RoundTrip(req)
}
