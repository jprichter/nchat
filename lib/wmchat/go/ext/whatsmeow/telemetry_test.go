package whatsmeow

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"net"
	"net/http"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/coder/websocket"
	"go.mau.fi/libsignal/ecc"
	"google.golang.org/protobuf/proto"

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/proto/waAdv"
	"go.mau.fi/whatsmeow/socket"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"go.mau.fi/whatsmeow/util/keys"
	waLog "go.mau.fi/whatsmeow/util/log"
)

// Capture protocol nodes at the same boundary used by production sends.
type telemetrySendLog struct {
	waLog.Logger
	nodes chan waBinary.Node
}

func (l telemetrySendLog) Debugf(format string, args ...any) {
	if format == "%s" && len(args) == 1 {
		if node, ok := args[0].(*waBinary.Node); ok {
			l.nodes <- *node
		}
	}
}

type pairingTestStore struct {
	store.DeviceContainer
	store.IdentityStore
	store.LIDStore
}

func (pairingTestStore) PutDevice(context.Context, *store.Device) error            { return nil }
func (pairingTestStore) PutIdentity(context.Context, string, [32]byte) error       { return nil }
func (pairingTestStore) PutLIDMapping(context.Context, types.JID, types.JID) error { return nil }

type telemetryListener struct {
	connections chan net.Conn
	closed      chan struct{}
	once        sync.Once
}

func (l *telemetryListener) Accept() (net.Conn, error) {
	select {
	case conn := <-l.connections:
		return conn, nil
	case <-l.closed:
		return nil, net.ErrClosed
	}
}
func (l *telemetryListener) Close() error { l.once.Do(func() { close(l.closed) }); return nil }
func (*telemetryListener) Addr() net.Addr { return &net.TCPAddr{} }

func telemetryTestClient(t *testing.T) (*Client, <-chan waBinary.Node) {
	t.Helper()
	// Carry real websocket/Noise frames over net.Pipe: no network or account.
	listener := &telemetryListener{connections: make(chan net.Conn), closed: make(chan struct{})}
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.CloseNow()
		for {
			if _, _, err = conn.Read(r.Context()); err != nil {
				return
			}
		}
	})}
	go server.Serve(listener)
	t.Cleanup(func() { _ = server.Close() })
	transport := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		client, peer := net.Pipe()
		select {
		case listener.connections <- peer:
			return client, nil
		case <-ctx.Done():
			client.Close()
			peer.Close()
			return nil, ctx.Err()
		}
	}}
	t.Cleanup(transport.CloseIdleConnections)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	fs := socket.NewFrameSocket(waLog.Noop, &http.Client{Transport: transport})
	fs.URL = "ws://offline.test/"
	fs.HTTPHeaders.Set("Origin", "http://offline.test")
	if err := fs.Connect(ctx); err != nil {
		t.Fatal(err)
	}
	handshake := socket.NewNoiseHandshake()
	handshake.Start("offline telemetry test", nil)
	sock, err := handshake.Finish(fs, func(context.Context, []byte) {}, func(context.Context, *socket.NoiseSocket, bool) {})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sock.Stop(true, false) })
	memory := pairingTestStore{}
	device := &store.Device{IdentityKey: keys.NewKeyPair(), AdvSecretKey: []byte("offline pairing secret"), PushName: "Offline", Container: memory, Identities: memory, LIDs: memory}
	cli := NewClient(device, nil)
	cli.socket = sock
	cli.BackgroundEventCtx = ctx
	nodes := make(chan waBinary.Node, 16)
	cli.sendLog = telemetrySendLog{Logger: waLog.Noop, nodes: nodes}
	return cli, nodes
}

func pairingSuccessNode(t *testing.T, cli *Client) *waBinary.Node {
	t.Helper()
	marshal := func(message proto.Message) []byte {
		data, err := proto.Marshal(message)
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	accountKey := keys.NewKeyPair()
	details := marshal(&waAdv.ADVDeviceIdentity{KeyIndex: proto.Uint32(1)})
	signature := ecc.CalculateSignature(ecc.NewDjbECPrivateKey(*accountKey.Priv), concatBytes(AdvAccountSignaturePrefix, details, cli.Store.IdentityKey.Pub[:]))
	identity := marshal(&waAdv.ADVSignedDeviceIdentity{Details: details, AccountSignatureKey: accountKey.Pub[:], AccountSignature: signature[:]})
	mac := hmac.New(sha256.New, cli.Store.AdvSecretKey)
	mac.Write(identity)
	container := marshal(&waAdv.ADVSignedDeviceIdentityHMAC{Details: identity, HMAC: mac.Sum(nil)})
	return &waBinary.Node{Tag: "iq", Attrs: waBinary.Attrs{"from": types.ServerJID, "id": "offline-pair", "t": time.Now().Unix()}, Content: []waBinary.Node{{
		Tag: "pair-success", Content: []waBinary.Node{
			{Tag: "device-identity", Content: container},
			{Tag: "device", Attrs: waBinary.Attrs{"jid": types.NewJID("123", types.DefaultUserServer), "lid": types.NewJID("456", types.HiddenUserServer)}},
		},
	}}}
}

func TestTelemetryCallerPaths(t *testing.T) {
	for _, caller := range []string{"pairing", "presence"} {
		for _, optIn := range []string{"", "0", "true", "1"} {
			t.Run(caller+"/opt-in="+optIn, func(t *testing.T) {
				t.Setenv("NCHAT_WHATSAPP_UNIFIED_SESSION", optIn)
				synctest.Test(t, func(t *testing.T) {
					cli, nodes := telemetryTestClient(t)
					ctx := cli.BackgroundEventCtx
					expectedTag := "presence"
					if caller == "pairing" {
						expectedTag = "iq"
						result := make(chan any, 1)
						cli.AddEventHandler(func(evt any) { result <- evt })
						cli.handleIQ(ctx, pairingSuccessNode(t, cli))
						select {
						case evt := <-result:
							if _, ok := evt.(*events.PairSuccess); !ok {
								t.Fatalf("pairing failed: %#v", evt)
							}
						case <-ctx.Done():
							t.Fatal("pairing did not finish")
						}
					} else if err := cli.SendPresence(ctx, types.PresenceAvailable); err != nil {
						t.Fatal(err)
					}
					normal, telemetry := 0, 0
					// Wait for the caller and any telemetry goroutine to become idle.
					// The opt-in case proves the observer sees telemetry when enabled.
					synctest.Wait()
					for len(nodes) > 0 {
						node := <-nodes
						if node.Tag == "ib" && node.GetChildByTag("unified_session").Tag != "" {
							telemetry++
						} else if node.Tag == expectedTag {
							normal++
						} else {
							t.Fatalf("unexpected node: %s", &node)
						}
					}
					wantTelemetry := 0
					if optIn == "1" {
						wantTelemetry = 1
					}
					if normal != 1 || telemetry != wantTelemetry {
						t.Fatalf("protocol nodes: normal=%d telemetry=%d opt-in=%q", normal, telemetry, optIn)
					}
				})
			})
		}
	}
}
