package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/codingconcepts/env"
	"github.com/nuttmeister/go-shadowsocks2/core"
)

// Constants.
const (
	listen  = "127.0.0.1"             // Ip that v2ray will listen to.
	forward = "127.0.0.1"             // Ip that v2ray will relay to.
	v2ray   = "/usr/bin/v2ray-plugin" // The location of v2ray-plugin.

	// v2ray-plugin options.
	v2rayOpts = "server;fast-open;tls=true;cert=/ssl/cert.pem;key=/ssl/private.key;host=%s;path=/prxy/%d;loglevel=%s"
)

var cfg *config

// Config contains the programs config.
type config struct {
	// General
	Host    string `env:"HOST" required:"true"`
	Verbose bool   `env:"VERBOSE" default:"false"`

	// Shadowsocks config.
	Cipher   string `env:"SS_CIPHER" required:"true"`
	Password string `env:"SS_PASSWORD" required:"true"`
	Port     int    `env:"SS_PORT" required:"true"`

	// V2Ray config.
	V2Ray     bool `env:"V2RAY_ENABLED" default:"false"`
	V2RayPort int  `env:"V2RAY_PORT"`

	// Bloom config.
	BloomCapacity int     `env:"BLOOM_CAPACITY" default:"1000000"`
	BloomFPR      float64 `env:"BLOOM_FPR" default:"0.000001"`
	BloomSlot     int     `env:"BLOOM_SLOT" default:"10"`
}

func main() {
	// Configure the service.
	if err := configure(); err != nil {
		log.Fatal(err)
	}

	// Start v2ray if it's configured.
	if err := cfg.startV2Ray(); err != nil {
		log.Fatal(err)
	}

	// Create the Cipher.
	ciph, err := core.PickCipher(cfg.Cipher, cfg.Password)
	if err != nil {
		log.Fatal(err)
	}

	// Start the tcp listener.
	go tcpRemote(cfg.Port, ciph.StreamConn)

	// Listen for exit and error to quit.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	killPlugin()
}

// configure will configure the program from env vars.
// Returns *config and error.
func configure() error {
	cfg = &config{}
	if err := env.Set(&cfg); err != nil {
		return err
	}

	// Check if V2Ray was enabled but port not set.
	if cfg.V2Ray && cfg.V2RayPort == 0 {
		return fmt.Errorf("v2ray-plugin enabled but port not configured")
	}

	return nil
}

// plugin will start any configured plugins.
// Returns error.
func (cfg *config) startV2Ray() error {
	if !cfg.V2Ray {
		return nil
	}

	// Set loglevel to none or debug.
	loglevel := "none"
	if cfg.Verbose {
		loglevel = "debug"
	}

	// Start the plugin.
	opts := fmt.Sprintf(v2rayOpts, cfg.Host, cfg.V2RayPort, loglevel)
	return startPlugin(v2ray, opts, cfg.V2RayPort, cfg.Port)
}
