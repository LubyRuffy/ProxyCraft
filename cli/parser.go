package cli

import (
	"flag"
	"fmt"
	"os"
)

// Config holds all configurable options for ProxyCraft.
// These will be populated from command-line arguments.
type Config struct {
	ListenHost       string
	ListenPort       int
	Verbose          bool
	OutputFile       string
	AutoSaveInterval int
	Filter           string
	ExportCAPath     string
	UseCACertPath    string
	UseCAKeyPath     string
	ShowHelp         bool
	EnableMITM       bool // Enable MITM mode for HTTPS traffic inspection
}

// ParseFlags parses the command-line arguments and returns a Config struct.
func ParseFlags() *Config {
	cfg := &Config{}

	flag.StringVar(&cfg.ListenHost, "l", "127.0.0.1", "IP address to listen on")
	flag.StringVar(&cfg.ListenHost, "listen-host", "127.0.0.1", "IP address to listen on")
	flag.IntVar(&cfg.ListenPort, "p", 8080, "Port to listen on")
	flag.IntVar(&cfg.ListenPort, "listen-port", 8080, "Port to listen on")
	flag.BoolVar(&cfg.Verbose, "v", false, "Enable verbose output")
	flag.BoolVar(&cfg.Verbose, "verbose", false, "Enable verbose output")
	flag.StringVar(&cfg.OutputFile, "o", "", "Save traffic to FILE (HAR format recommended)")
	flag.StringVar(&cfg.OutputFile, "output-file", "", "Save traffic to FILE (HAR format recommended)")
	flag.IntVar(&cfg.AutoSaveInterval, "auto-save", 10, "Auto-save HAR file every N seconds (0 to disable)")
	flag.StringVar(&cfg.Filter, "filter", "", "Filter displayed traffic (e.g., \"host=example.com\")")
	flag.StringVar(&cfg.ExportCAPath, "export-ca", "", "Export the root CA certificate to FILEPATH and exit")
	flag.StringVar(&cfg.UseCACertPath, "use-ca", "", "Use custom root CA certificate from CERT_PATH")
	flag.StringVar(&cfg.UseCAKeyPath, "use-key", "", "Use custom root CA private key from KEY_PATH")
	flag.BoolVar(&cfg.EnableMITM, "mitm", false, "Enable MITM mode for HTTPS traffic inspection")

	// Custom help flag
	flag.BoolVar(&cfg.ShowHelp, "h", false, "Show this help message and exit")
	flag.BoolVar(&cfg.ShowHelp, "help", false, "Show this help message and exit")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "ProxyCraft CLI - A command-line HTTPS/HTTP2/SSE proxy tool.\n")
		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	return cfg
}

// PrintHelp prints the help message.
func PrintHelp() {
	flag.Usage()
}
