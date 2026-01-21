package cli

import (
	"flag"
	"fmt"
	"os"
)

// Config holds all configurable options for ProxyCraft.
// These will be populated from command-line arguments.
type Config struct {
	ListenHost       string // Proxy server host
	ListenPort       int    // Proxy server port
	WebPort          int    // Web UI port
	Verbose          bool   // More verbose
	HarOutputFile    string // Save traffic to FILE (HAR format recommended)
	AutoSaveInterval int    // Auto-save HAR file every N seconds (0 to disable)
	Filter           string // Filter displayed traffic (e.g., "host=example.com")
	ExportCAPath     string // Export the root CA certificate to FILEPATH and exit
	UseCACertPath    string // Use custom root CA certificate from CERT_PATH
	UseCAKeyPath     string // Use custom root CA private key from KEY_PATH
	InstallCerts     bool   // Install CA certificate to system trust store
	ForceReinstallCA bool   // Force reinstall CA certificate to system trust store
	ShowHelp         bool   // Show this help message and exit
	UpstreamProxy    string // Upstream proxy URL (e.g., "http://proxy.example.com:8080")
	DumpTraffic      bool   // Enable dumping traffic content to console
	Mode             string // 运行模式: "" (CLI模式) 或 "web" (Web界面模式)
}

// ParseFlags parses the command-line arguments and returns a Config struct.
func ParseFlags() *Config {
	cfg := &Config{}

	flag.StringVar(&cfg.ListenHost, "l", "127.0.0.1", "IP address to listen on")
	flag.StringVar(&cfg.ListenHost, "listen-host", "127.0.0.1", "IP address to listen on")
	flag.IntVar(&cfg.ListenPort, "p", 38080, "Port to listen on")
	flag.IntVar(&cfg.ListenPort, "listen-port", 38080, "Port to listen on")
	flag.BoolVar(&cfg.Verbose, "v", false, "Enable verbose output")
	flag.BoolVar(&cfg.Verbose, "verbose", false, "Enable verbose output")
	flag.StringVar(&cfg.HarOutputFile, "o", "", "Save traffic to FILE (HAR format recommended)")
	flag.StringVar(&cfg.HarOutputFile, "output-file", "", "Save traffic to FILE (HAR format recommended)")
	flag.IntVar(&cfg.AutoSaveInterval, "auto-save", 10, "Auto-save HAR file every N seconds (0 to disable)")
	flag.StringVar(&cfg.Filter, "filter", "", "Filter displayed traffic (e.g., \"host=example.com\")")
	flag.StringVar(&cfg.ExportCAPath, "export-ca", "", "Export the root CA certificate to FILEPATH and exit")
	flag.StringVar(&cfg.UseCACertPath, "use-ca", "", "Use custom root CA certificate from CERT_PATH")
	flag.StringVar(&cfg.UseCAKeyPath, "use-key", "", "Use custom root CA private key from KEY_PATH")
	flag.BoolVar(&cfg.InstallCerts, "install-ca", false, "Install the CA certificate to system trust store and exit")
	flag.BoolVar(&cfg.ForceReinstallCA, "force-reinstall-ca", false, "Force reinstall the CA certificate to system trust store")
	flag.StringVar(&cfg.UpstreamProxy, "upstream-proxy", "", "Upstream proxy URL (e.g., \"http://proxy.example.com:8080\")")
	flag.BoolVar(&cfg.DumpTraffic, "dump", false, "Dump traffic content to console with headers (binary content will not be displayed)")
	flag.StringVar(&cfg.Mode, "mode", "", "Running mode: empty for CLI mode, 'web' for Web UI mode")

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
