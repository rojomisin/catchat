# Catchat

A deliberately small, BitchX-inspired terminal chat room written in Go. It uses
[`github.com/tailscale/tailcat`](https://github.com/tailscale/tailcat) as its
transport: the host gives friends a Tailcat connection token, and every client
connects through a WireGuard-encrypted tunnel without a Tailscale account,
admin privileges, or an Internet-facing chat port.

This is an MVP: one ephemeral `#lobby`, nicknames, live messages, a user list,
and `/help` / `/quit`. There are no accounts, history, files, DMs, typing
indicators, or federation.

## Future direction

### Naming candidates

- **Catchat** — current working name; a nod to Tailcat and IRC-era chat.
- **Tailchat** — clearest description, but very close to the Tailcat transport
  dependency and Tailscale branding; use only after checking for naming and
  trademark conflicts.
- **trc** — compact terminal name, expandable as *Tail Relay Chat*; suitable
  for a binary/CLI even if the user-facing product keeps a fuller name.

### Possible next features

- Persistent client keys and Tailcat server allowlists.
- Multiple rooms and direct messages, with clear host-versus-peer encryption
  semantics.
- Optional local history and a signed, append-only event format.
- QR sharing for invitation tokens and identity keys.
- A separately designed, opt-in anti-spam or payment layer; it should not be a
  prerequisite for basic chat.

## Quick start

Requires Go 1.26.6+.

```sh
git clone <your-catchat-repository>
cd catchat
go run ./cmd/catchat serve
```

The host prints one invitation token. Send it to a friend through a channel you
already trust, then they can join:

```sh
go run ./cmd/catchat join --nick alice 'tc...'
```

Build a standalone binary with:

```sh
go build -o catchat ./cmd/catchat
```

## Bring your own DERP

With no extra flags, Tailcat uses its default DERP map. To ensure the token and
clients use only your relay, run a DERP server with TLS and STUN enabled, then:

```sh
catchat serve --derp derp.example.com
```

The invitation token embeds `derp.example.com`, so joining clients do not need
to fetch Tailcat's public DERP map. Use `--derp-port` and `--stun-port` if your
relay does not use ports 443 and 3478.

## Security model

Tailcat encrypts the network hop from each client to the hub with WireGuard and
can upgrade from DERP relay to direct UDP when NAT traversal succeeds. A DERP
relay cannot read that transport traffic. Owning the DERP server avoids using
Tailcat's public relay infrastructure, but it does **not** eliminate all
metadata: a relay can still observe connections and timing when it carries
traffic.

Catchat is **not Signal-like end-to-end messaging** in this MVP. The hub
decrypts each client-to-hub connection in order to fan messages out to the
room, so the host can read room content. Treat the invitation token as an
invite, share it out of band, and do not use the MVP for sensitive discussion.
It also has no persistent client identity or Tailcat allowlist yet.

## Why not add Lightning / WhatSat now?

Not for the first version. The useful shape to preserve from experiments such
as Lightning chat is a small, event-based message protocol and a transport that
can be swapped or extended. Payments, invoices, routing, persistence, and
anti-spam policy are separate product problems that would distract from proving
that the basic chat experience works. A later opt-in payment or relay-incentive
layer can sit beside this protocol without changing the terminal chat MVP.

## Development

```sh
go test ./...
go vet ./...
```

The test suite covers the bounded wire-frame parser and input validation.
