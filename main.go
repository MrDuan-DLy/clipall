package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

func main() {
	peers := flag.String("peers", "", "comma-separated peer addresses (host:port)")
	listen := flag.Int("listen", 9876, "port to listen on")
	configFile := flag.String("config", "", "path to config file (default: auto-detect)")
	flag.Parse()

	cfg := DefaultConfig()

	// Load config file if specified or if default exists.
	cfgPath := *configFile
	if cfgPath == "" {
		cfgPath = DefaultConfigPath()
	}
	if *configFile != "" {
		// Explicitly specified config must exist.
		var err error
		cfg, err = LoadConfig(cfgPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	} else if _, err := os.Stat(cfgPath); err == nil {
		// Default config path exists, load it.
		loaded, err := LoadConfig(cfgPath)
		if err != nil {
			log.Printf("[main] warning: ignoring config %s: %v", cfgPath, err)
		} else {
			cfg = loaded
		}
	}

	// CLI flags override config file.
	if *listen != 9876 {
		cfg.Listen.Port = *listen
	}

	// Build peer address list.
	var peerAddrs []string
	if *peers != "" {
		for _, p := range strings.Split(*peers, ",") {
			addr := strings.TrimSpace(p)
			if addr != "" {
				peerAddrs = append(peerAddrs, addr)
			}
		}
	} else {
		peerAddrs = cfg.PeerAddrs()
	}

	if len(peerAddrs) == 0 {
		fmt.Fprintln(os.Stderr, "error: no peers configured. Use --peers flag or config file.")
		fmt.Fprintf(os.Stderr, "  example: clipall --peers windows:9876\n")
		fmt.Fprintf(os.Stderr, "  config:  %s\n", DefaultConfigPath())
		os.Exit(1)
	}

	log.Printf("[main] clipall starting, peers: %v, listen: :%d", peerAddrs, cfg.Listen.Port)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	node := NewNode(cfg.Listen.Port, peerAddrs)
	if err := node.Run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	log.Println("[main] clipall stopped")
}
