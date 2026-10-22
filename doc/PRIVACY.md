# Privacy and attachment security in this fork

WhatsApp attachment cache names hash the message ID and append a separately
sanitized, length-limited filename. Images, videos (including video notes),
audio, documents and stickers use the same naming helper. Serialized download
metadata is untrusted, including old cache entries: only canonical immediate
children of the current profile's `tmp` directory are accepted. Absolute paths
are accepted only inside that directory, to retain valid existing profiles.
Directory descriptors, no-follow opens and exclusive atomic publication prevent
symlink traversal and overwriting a destination created during download.
Existing regular files remain usable; directories, symlinks and multiply linked
files fail. A malicious process running as your user can still change files
after validation or before a viewer opens them; this is not isolation from that
user. A local reproduction demonstrated a cache escape before this fix. Whether
WhatsApp's servers accept such message IDs is unverified; no remote exploit is
claimed.

Media downloads require HTTPS without userinfo, explicit ports or fragments.
The allowlist is `mmg.whatsapp.net`, `static.whatsapp.net`, and single-label
`media-*.cdn.whatsapp.net` regional hosts, with label boundary checks. The
vendored `download.go` builds direct-path requests using `media_conn` hostnames
and uses `static.whatsapp.net/sticker` for sticker packs. The upstream
[media connection example](https://github.com/tulir/whatsmeow/issues/481)
shows the fallback `mmg.whatsapp.net` and regional CDN names. This is a
deliberately narrow set, rather than permitting every Meta-owned hostname;
new CDN naming schemes may need a reviewed allowlist update.

The same request boundary validates message URLs, direct-path downloads and
redirects before transport. For the standard HTTP transport, DNS answers must
all be public addresses; loopback, private, link-local, shared, documentation
and transition ranges are rejected. Connections dial the checked IP directly
while TLS verifies the original hostname, preventing a second DNS lookup from
rebinding the destination. Attachment downloads deliberately ignore the
configured `proxy_` SOCKS5 or HTTP proxy, environment proxies and custom dialers,
because a proxy could resolve the hostname again. With `proxy_` set, downloads
therefore connect directly to WhatsApp's media servers and do not use your proxy;
that exposes your IP address to them. Websocket and upload traffic still use the
configured proxy.
A library caller injecting its own RoundTripper supplies trusted application
code responsible for equivalent connection checks; URL restrictions still apply.
No live account was used for the offline regression tests, so worldwide CDN
compatibility is not claimed.

This fork suppresses WhatsApp `unified_session` telemetry by default at the
shared send function, covering pairing and presence. If protocol compatibility
requires it, explicitly launch with `NCHAT_WHATSAPP_UNIFIED_SESSION=1` to opt in.
Other values do not enable it. Authentication and other protocol messages are
unchanged. Suppressing this message does not remove WhatsApp's inherent service
metadata, such as account identity, connection addresses, timing and routing.
No separate developer analytics were identified in the reviewed paths.

AI auto-compose remains disabled (`auto_compose_enabled=0`). While disabled,
nchat does not invoke the compose helper or automatically upload history for
AI replies. When enabled and explicitly invoked with the auto-compose key
(default Alt-I), it passes up to `auto_compose_history_count` messages (default
25) around the selected/latest message, sender display names and your own name
to the configured command via a local plaintext temporary history file. The
bundled helper sends those names and text in the request payload. OpenAI is the
default provider; Gemini, Ollama and custom compatible endpoints are supported.
See [Auto-Compose](AUTOCOMPOSE.md) for configuration and disclosure details.

Local profiles, messages, downloaded attachments and WhatsApp `session.db` are
not encrypted at rest by nchat. The default `umask 077` remains in place, and new
media files are created with mode 0600. Filesystem permissions do not protect
against another process running as your user or an administrator. Use encrypted
storage and encrypted backups, restrict backup access, and treat `session.db`
as an account credential. Stop nchat before taking a consistent profile backup;
avoid sharing profiles, history exports or diagnostic logs. Media keys,
serialized attachment metadata and token-bearing media URLs are no longer
printed by the attachment diagnostics changed here; other trace output may
still include private chat data.

The convenience installer executes a script fetched from GitHub and downloads
release archives and `sha256sums.txt` from the same release origin. Checksums
detect corruption and mismatch against that manifest; they do not establish an
independent signature or protect against compromise of the release origin.
There is no independent signing-key or source/binary-equivalence guarantee in
this workflow. Inspect a downloaded script before running it, choose the
intended repository (`NCHAT_REPO`, default `d99kris/nchat`) and pin a release with
`NCHAT_VERSION` where appropriate. Building reviewed source is a separate trust
choice, not proof that a downloaded release binary matches it.
