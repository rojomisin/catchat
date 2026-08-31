// Catchat is a small BitchX-inspired group-chat client and hub carried over
// Tailcat's encrypted, NAT-traversing userspace transport.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/rogermicone/catchat/internal/chat"
	"github.com/rogermicone/catchat/internal/transport"
	"github.com/rogermicone/catchat/internal/ui"
)

const defaultPort = 6667

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	var err error
	switch os.Args[1] {
	case "serve":
		err = serve(os.Args[2:])
	case "join":
		err = join(os.Args[2:])
	case "help", "-h", "--help":
		usage()
		return
	default:
		err = fmt.Errorf("unknown command %q", os.Args[1])
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "catchat:", err)
		os.Exit(1)
	}
}

func serve(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	port := fs.Int("port", defaultPort, "Tailcat port to serve")
	derpHost := fs.String("derp", "", "private DERP hostname (defaults to Tailcat's public DERP map)")
	derpPort := fs.Int("derp-port", 443, "private DERP HTTPS port")
	stunPort := fs.Int("stun-port", 3478, "private DERP STUN UDP port")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New("serve does not take positional arguments")
	}
	if err := validPort(*port, "port"); err != nil {
		return err
	}
	if *derpHost != "" {
		if err := validPort(*derpPort, "derp-port"); err != nil {
			return err
		}
		if err := validPort(*stunPort, "stun-port"); err != nil {
			return err
		}
	}

	hub := chat.NewHub()
	server, token, err := transport.Serve(uint16(*port), strings.TrimSpace(*derpHost), *derpPort, *stunPort, hub.ServeConn)
	if err != nil {
		return err
	}
	defer server.Close()

	fmt.Fprintln(os.Stderr, "Catchat #lobby is listening through Tailcat.")
	if *derpHost == "" {
		fmt.Fprintln(os.Stderr, "Using Tailcat's default DERP map. Use --derp for a private relay.")
	} else {
		fmt.Fprintf(os.Stderr, "Using private DERP %s. The token embeds this relay.\n", *derpHost)
	}
	fmt.Fprintln(os.Stderr, "Share this invitation token out of band:")
	fmt.Println(token)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()
	return nil
}

func join(args []string) error {
	fs := flag.NewFlagSet("join", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	nick := fs.String("nick", "", "nickname (1-24 IRC-safe characters)")
	port := fs.Int("port", defaultPort, "Tailcat port exposed by the hub")
	timeout := fs.Duration("timeout", 20*time.Second, "connection timeout")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("join requires exactly one invitation token")
	}
	if strings.TrimSpace(*nick) == "" {
		return errors.New("join requires --nick")
	}
	if err := validPort(*port, "port"); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	client, err := transport.Dial(ctx, strings.TrimSpace(fs.Arg(0)), uint16(*port))
	if err != nil {
		return err
	}
	defer client.Close()
	return ui.Run(client.Conn(), *nick)
}

func validPort(port int, name string) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("%s must be between 1 and 65535", name)
	}
	return nil
}

func usage() {
	fmt.Fprint(os.Stderr, `Catchat — small IRC-style chat over Tailcat

Usage:
  catchat serve [--port 6667] [--derp derp.example.com]
  catchat join --nick alice [--port 6667] <invitation-token>

The host prints a Tailcat invitation token. Share it out of band, then peers
join it from an interactive terminal. Tailcat encrypts transport traffic and
uses DERP only when a direct path cannot be made.

`)
}
