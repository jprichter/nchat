package whatsmeow

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"testing"

	"go.mau.fi/whatsmeow/store"
	waLog "go.mau.fi/whatsmeow/util/log"
)

type testMediaTransport func(*http.Request) (*http.Response, error)

func (f testMediaTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestMediaURLPolicy(t *testing.T) {
	for _, raw := range []string{"https://mmg.whatsapp.net/v/t62.7118-24/file?token=x", "https://media-fra3-2.cdn.whatsapp.net/mms/image/file", "https://static.whatsapp.net/sticker?id=1"} {
		u, _ := url.Parse(raw)
		if err := validateMediaURL(u); err != nil {
			t.Errorf("valid URL rejected: %s", raw)
		}
	}
	bad := []string{
		"http://mmg.whatsapp.net/file", "https://mmg.whatsapp.net:443/file", "https://mmg.whatsapp.net:8443/file",
		"https://user@mmg.whatsapp.net/file", "https://mmg.whatsapp.net.evil.test/file", "https://evilwhatsapp.net/file",
		"https://evil.cdn.whatsapp.net/file", "https://media-a.evil.cdn.whatsapp.net/file", "https://mmg.whatsapp.net./file",
		"https://127.0.0.1/file", "https://[::1]/file", "https://169.254.169.254/file", "file:///etc/passwd",
		"https://mmg.whatsapp.net/file#fragment", "//mmg.whatsapp.net/file", "https://mmg.whatsapp.net:/file",
	}
	for _, raw := range bad {
		t.Run(raw, func(t *testing.T) {
			cli := &Client{Log: waLog.Noop, mediaHTTP: &http.Client{Transport: testMediaTransport(func(r *http.Request) (*http.Response, error) {
				t.Fatal("disallowed URL reached transport")
				return nil, nil
			})}}
			if _, err := cli.doMediaDownloadRequest(context.Background(), raw); err == nil {
				t.Fatal("unsafe URL accepted")
			}
		})
	}
}

func TestMediaRedirectPolicy(t *testing.T) {
	for _, location := range []string{"http://127.0.0.1/secret", "https://evil.test/secret", "https://mmg.whatsapp.net:443/file", "https://mmg.whatsapp.net:/file", "https://user@mmg.whatsapp.net/file"} {
		calls := 0
		cli := &Client{mediaHTTP: &http.Client{Transport: testMediaTransport(func(r *http.Request) (*http.Response, error) {
			calls++
			if calls > 1 {
				t.Fatal("disallowed redirect reached transport")
			}
			return &http.Response{StatusCode: 302, Header: http.Header{"Location": {location}}, Body: io.NopCloser(strings.NewReader("")), Request: r}, nil
		})}}
		if _, err := cli.doMediaDownloadRequest(context.Background(), "https://mmg.whatsapp.net/file"); err == nil {
			t.Fatal("redirect accepted")
		}
		if calls != 1 {
			t.Fatal("unexpected request count")
		}
	}
}

func TestAllowedMediaRedirect(t *testing.T) {
	calls := 0
	cli := &Client{mediaHTTP: &http.Client{Transport: testMediaTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			return &http.Response{StatusCode: 302, Header: http.Header{"Location": {"https://media-fra3-2.cdn.whatsapp.net/file"}}, Body: io.NopCloser(strings.NewReader("")), Request: r}, nil
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("media")), Request: r}, nil
	})}}
	data, err := cli.downloadMedia(context.Background(), "https://mmg.whatsapp.net/file")
	if err != nil || string(data) != "media" || calls != 2 {
		t.Fatalf("valid media failed: %s %v", data, err)
	}
}

type testMediaResolver []netip.Addr

func (r testMediaResolver) LookupNetIP(context.Context, string, string) ([]netip.Addr, error) {
	return r, nil
}

func TestMediaDNSPolicy(t *testing.T) {
	for _, raw := range []string{"127.0.0.1", "10.0.0.1", "172.16.0.1", "192.168.1.1", "169.254.169.254", "100.64.0.1", "0.0.0.0", "224.0.0.1", "::1", "fe80::1", "fc00::1", "::ffff:127.0.0.1", "64:ff9b::7f00:1", "2002:7f00:1::", "2001:db8::1"} {
		for _, ips := range []testMediaResolver{{netip.MustParseAddr(raw)}, {netip.MustParseAddr("157.240.0.60"), netip.MustParseAddr(raw)}} {
			_, err := dialMedia(context.Background(), "tcp", "mmg.whatsapp.net:443", ips, func(context.Context, string, string) (net.Conn, error) {
				t.Fatal("private/mixed DNS answer reached dialer")
				return nil, nil
			})
			if err == nil {
				t.Fatalf("unsafe DNS answer accepted: %s", raw)
			}
		}
	}
	var addresses []string
	_, _ = dialMedia(context.Background(), "tcp", "mmg.whatsapp.net:443", testMediaResolver{netip.MustParseAddr("157.240.0.60"), netip.MustParseAddr("2a03:2880:f277:1cd:face:b00c:0:167")}, func(ctx context.Context, network, address string) (net.Conn, error) {
		addresses = append(addresses, address)
		return nil, errors.New("offline")
	})
	if len(addresses) != 2 || addresses[0] != "157.240.0.60:443" || addresses[1] != "[2a03:2880:f277:1cd:face:b00c:0:167]:443" {
		t.Fatalf("dial did not use pinned IPs: %v", addresses)
	}
}

func TestMediaTransportRemovesAlternateDialers(t *testing.T) {
	base := &http.Client{Transport: &http.Transport{DialTLSContext: func(context.Context, string, string) (net.Conn, error) {
		t.Fatal("custom TLS dialer called")
		return nil, nil
	}}}
	client := mediaDownloadClient(base)
	transport := client.Transport.(mediaPolicyTransport).RoundTripper.(*http.Transport)
	if transport.DialTLSContext != nil || transport.DialContext == nil {
		t.Fatal("unsafe alternate route remains")
	}
	if base.Transport.(*http.Transport).DialTLSContext == nil {
		t.Fatal("modified shared HTTP client")
	}
}

func TestMediaHTTPProxyFailsClosed(t *testing.T) {
	for _, scheme := range []string{"http", "https"} {
		t.Run(scheme, func(t *testing.T) {
			cli := NewClient(&store.Device{}, nil)
			proxyURL, _ := url.Parse(scheme + "://127.0.0.1:1")
			cli.SetProxy(http.ProxyURL(proxyURL))
			original := cli.mediaHTTP.Transport.(*http.Transport)
			// No DNS lookup, direct dial or proxy dial may occur.
			original.DialContext = func(context.Context, string, string) (net.Conn, error) {
				t.Fatal("proxy rejection reached dialer")
				return nil, nil
			}
			_, err := cli.doMediaDownloadRequest(context.Background(), "https://mmg.whatsapp.net/file")
			if !errors.Is(err, errMediaProxy) {
				t.Fatalf("expected clear proxy error, got %v", err)
			}
			if original.Proxy == nil {
				t.Fatal("modified shared proxy configuration")
			}
		})
	}
}

type testSOCKSProxy struct{ t *testing.T }

func (p testSOCKSProxy) Dial(network, address string) (net.Conn, error) {
	p.t.Fatal("SOCKS proxy dialed")
	return nil, nil
}

func (p testSOCKSProxy) DialContext(context.Context, string, string) (net.Conn, error) {
	p.t.Fatal("SOCKS proxy dialed")
	return nil, nil
}

func TestMediaSOCKSProxyFailsClosed(t *testing.T) {
	cli := NewClient(&store.Device{}, nil)
	cli.SetSOCKSProxy(testSOCKSProxy{t})
	_, err := cli.doMediaDownloadRequest(context.Background(), "https://mmg.whatsapp.net/file")
	if !errors.Is(err, errMediaProxy) {
		t.Fatalf("expected clear proxy error, got %v", err)
	}
	cli.SetProxy(nil)
	if cli.mediaSOCKSProxy {
		t.Fatal("removing proxy did not restore direct media policy")
	}
	cli.SetSOCKSProxy(testSOCKSProxy{t}, SetProxyOptions{NoMedia: true})
	if cli.mediaSOCKSProxy {
		t.Fatal("websocket-only proxy blocked direct media")
	}
}

func TestUnifiedSessionDefaultOff(t *testing.T) {
	for _, value := range []string{"", "0", "true"} {
		t.Setenv("NCHAT_WHATSAPP_UNIFIED_SESSION", value)
		// A zero client has no socket or logger: trying to send would panic.
		(&Client{}).sendUnifiedSession()
	}
}

func TestMediaErrorsRedactURLTokens(t *testing.T) {
	cli := &Client{mediaHTTP: &http.Client{Transport: testMediaTransport(func(r *http.Request) (*http.Response, error) {
		return nil, errors.New("offline")
	})}}
	_, err := cli.doMediaDownloadRequest(context.Background(), "https://mmg.whatsapp.net/file?token=SECRET")
	if err == nil || strings.Contains(err.Error(), "SECRET") {
		t.Fatalf("unsafe request diagnostic: %v", err)
	}
}
