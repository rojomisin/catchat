// Package transport adapts Tailcat's encrypted userspace TCP transport for
// Catchat. The chat hub never needs an Internet-facing TCP listener.
package transport

import (
	"context"
	"fmt"
	"net"

	"github.com/tailscale/tailcat"
	"tailscale.com/tailcfg"
)

// Serve starts a Tailcat listener and dispatches only the requested port to
// handler. If derpHost is set, its details are embedded in the share token, so
// clients do not contact Tailcat's public DERP map.
func Serve(port uint16, derpHost string, derpPort, stunPort int, handler func(net.Conn)) (*tailcat.Server, string, error) {
	server := &tailcat.Server{
		OnTCP: func(requested uint16) func(net.Conn) {
			if requested != port {
				return nil
			}
			return handler
		},
	}
	if derpHost != "" {
		server.Region = customDERPRegion(derpHost, derpPort, stunPort)
	}
	if err := server.Start(); err != nil {
		return nil, "", fmt.Errorf("start Tailcat server: %w", err)
	}
	return server, string(server.ConnBlob()), nil
}

// Client owns both the Tailcat client and the open chat connection.
type Client struct {
	conn net.Conn
	peer *tailcat.Client
}

// Dial connects to a shared Catchat token.
func Dial(ctx context.Context, token string, port uint16) (*Client, error) {
	peer := tailcat.NewClient(tailcat.ConnBlob(token))
	conn, err := peer.DialTCPPort(ctx, port)
	if err != nil {
		_ = peer.Close()
		return nil, fmt.Errorf("connect through Tailcat: %w", err)
	}
	return &Client{conn: conn, peer: peer}, nil
}

func (c *Client) Conn() net.Conn { return c.conn }

func (c *Client) Close() error {
	if c.conn != nil {
		_ = c.conn.Close()
	}
	return c.peer.Close()
}

func customDERPRegion(host string, derpPort, stunPort int) *tailcfg.DERPRegion {
	return &tailcfg.DERPRegion{
		RegionID:   1,
		RegionCode: "catchat",
		RegionName: "Catchat private DERP",
		Nodes: []*tailcfg.DERPNode{{
			Name:     "catchat-derp-1",
			RegionID: 1,
			HostName: host,
			DERPPort: derpPort,
			STUNPort: stunPort,
		}},
	}
}
