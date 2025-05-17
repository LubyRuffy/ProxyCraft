package main

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/LubyRuffy/ProxyCraft/certs"
	"github.com/LubyRuffy/ProxyCraft/cli"
	"github.com/LubyRuffy/ProxyCraft/harlogger" // Added for HAR logging
	"github.com/LubyRuffy/ProxyCraft/proxy"
)

const appName = "ProxyCraft CLI"
const appVersion = "0.1.0" // TODO: This should ideally come from a build flag or version file

func main() {
	cfg := cli.ParseFlags()

	if cfg.ShowHelp {
		cli.PrintHelp()
		return
	}

	fmt.Println("ProxyCraft CLI starting...")

	certManager, err := certs.NewManager()
	if err != nil {
		log.Fatalf("Error initializing certificate manager: %v", err)
	}

	if cfg.ExportCAPath != "" {
		err = certManager.ExportCACert(cfg.ExportCAPath)
		if err != nil {
			log.Fatalf("Error exporting CA certificate: %v", err)
		}
		fmt.Printf("CA certificate exported to %s. Exiting.\n", cfg.ExportCAPath)
		return
	}

	// Use custom CA certificate and key if provided
	if cfg.UseCACertPath != "" && cfg.UseCAKeyPath != "" {
		err = certManager.LoadCustomCA(cfg.UseCACertPath, cfg.UseCAKeyPath)
		if err != nil {
			log.Fatalf("Error loading custom CA certificate and key: %v", err)
		}
		log.Printf("Successfully loaded custom CA certificate and key")
	}

	listenAddr := fmt.Sprintf("%s:%d", cfg.ListenHost, cfg.ListenPort)
	fmt.Printf("Proxy server attempting to listen on %s\n", listenAddr)
	if cfg.Verbose {
		fmt.Println("Verbose mode enabled.")
	}

	// Initialize HAR Logger
	harLogger := harlogger.NewLogger(cfg.OutputFile, appName, appVersion)
	if harLogger.IsEnabled() {
		log.Printf("HAR logging enabled, will save to: %s", cfg.OutputFile)

		// Enable auto-save if interval > 0
		if cfg.AutoSaveInterval > 0 {
			log.Printf("Auto-save enabled, HAR log will be saved every %d seconds", cfg.AutoSaveInterval)
			harLogger.EnableAutoSave(time.Duration(cfg.AutoSaveInterval) * time.Second)
		} else {
			log.Printf("Auto-save disabled, HAR log will only be saved on exit")
		}

		// Also save on exit
		defer func() {
			if cfg.AutoSaveInterval > 0 {
				harLogger.DisableAutoSave() // Stop auto-save before final save
			}
			if err := harLogger.Save(); err != nil {
				log.Printf("Error saving HAR log on exit: %v", err)
			}
		}()
	}

	// 解析上层代理URL
	var upstreamProxyURL *url.URL
	if cfg.UpstreamProxy != "" {
		var err error
		upstreamProxyURL, err = url.Parse(cfg.UpstreamProxy)
		if err != nil {
			log.Fatalf("Error parsing upstream proxy URL: %v", err)
		}
		log.Printf("Using upstream proxy: %s", upstreamProxyURL.String())
	}

	// Initialize and start the proxy server
	proxyServer := proxy.NewServer(listenAddr, certManager, cfg.Verbose, harLogger, cfg.EnableMITM, upstreamProxyURL, cfg.DumpTraffic)

	// 如果启用了流量输出
	if cfg.DumpTraffic {
		fmt.Println("Traffic dump enabled - HTTP request and response content will be displayed in console")
	}

	// Log MITM mode status
	if cfg.EnableMITM {
		log.Printf("MITM mode enabled - HTTPS traffic will be decrypted and inspected")
		log.Printf("Make sure to add the CA certificate to your browser/system trust store")
		log.Printf("You can export the CA certificate using the -export-ca flag")
		log.Printf("CA certificate is located at: %s", certs.GetCACertPath())
		log.Printf("For curl, you can use: curl --cacert %s --proxy http://%s https://example.com", certs.GetCACertPath(), listenAddr)
	} else {
		log.Printf("MITM mode disabled - HTTPS traffic will be tunneled directly (no inspection)")
		log.Printf("To enable MITM mode, use the -mitm flag")
	}

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start the proxy server in a goroutine
	go func() {
		log.Printf("Starting proxy server on %s", listenAddr)
		if err := proxyServer.Start(); err != nil {
			log.Fatalf("Failed to start proxy server: %v", err)
		}
	}()

	// Wait for termination signal
	sig := <-sigChan
	log.Printf("Received signal %v, shutting down...", sig)
	// The deferred harLogger.Save() will be called when main() exits
}
