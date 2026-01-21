package main

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/LubyRuffy/ProxyCraft/api"
	"github.com/LubyRuffy/ProxyCraft/certs"
	"github.com/LubyRuffy/ProxyCraft/cli"
	"github.com/LubyRuffy/ProxyCraft/harlogger" // Added for HAR logging
	"github.com/LubyRuffy/ProxyCraft/proxy"
	"github.com/LubyRuffy/ProxyCraft/proxy/handlers"
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

	if cfg.InstallCerts {
		if cfg.ForceReinstallCA {
			err = certManager.InstallCertsForce()
		} else {
			err = certManager.InstallCerts()
		}
		if err != nil {
			log.Fatalf("Error installing CA certificate: %v", err)
		}
		fmt.Println("CA certificate installed successfully. Exiting.")
		return
	}

	// Use custom CA certificate and key if provided
	if cfg.UseCACertPath != "" && cfg.UseCAKeyPath != "" {
		err = certManager.LoadCustomCA(cfg.UseCACertPath, cfg.UseCAKeyPath)
		if err != nil {
			log.Fatalf("Error loading custom CA certificate and key: %v", err)
		}
		log.Printf("Successfully loaded custom CA certificate and key")
	} else {
		if cfg.ForceReinstallCA {
			log.Printf("Force reinstalling CA certificate in system trust store...")
			err = certManager.InstallCertsForce()
			if err != nil {
				log.Printf("Warning: Failed to force reinstall CA certificate: %v", err)
				log.Printf("Please manually install the CA certificate using the -install-ca flag")
				log.Printf("Or export the certificate with -export-ca and install it manually")
			} else {
				log.Printf("CA certificate installed successfully")
			}
		} else {
			// Automatically check if CA certificate is installed and install if needed
			log.Printf("Checking if CA certificate is installed in system trust store...")
			if !certs.IsInstalled() {
				log.Printf("CA certificate not installed. Attempting to install...")
				err = certManager.InstallCerts()
				if err != nil {
					log.Printf("Warning: Failed to automatically install CA certificate: %v", err)
					log.Printf("Please manually install the CA certificate using the -install-ca flag")
					log.Printf("Or export the certificate with -export-ca and install it manually")
				} else {
					log.Printf("CA certificate installed successfully")
				}
			} else {
				log.Printf("CA certificate matches system trust store. Skipping installation.")
			}
		}
	}

	listenAddr := fmt.Sprintf("%s:%d", cfg.ListenHost, cfg.ListenPort)
	fmt.Printf("Proxy server attempting to listen on %s\n", listenAddr)
	if cfg.Verbose {
		fmt.Println("Verbose mode enabled.")
	}

	// Initialize HAR Logger
	harLogger := harlogger.NewLogger(cfg.HarOutputFile, appName, appVersion)
	if harLogger.IsEnabled() {
		log.Printf("HAR logging enabled, will save to: %s", cfg.HarOutputFile)

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

	// 根据模式选择事件处理器
	var eventHandler proxy.EventHandler

	// Web模式使用WebHandler
	if cfg.Mode == "web" {
		log.Printf("启动Web模式...")

		// 创建Web事件处理器
		webHandler := handlers.NewWebHandler(cfg.Verbose)

		// 创建API服务器，默认使用8081端口
		apiServer := api.NewServer(webHandler, 8081)

		// 启动API服务器
		go func() {
			log.Printf("启动API服务器在端口8081...")
			if err := apiServer.Start(); err != nil {
				log.Fatalf("启动API服务器失败: %v", err)
			}
		}()

		// 设置Web处理器为事件处理器
		eventHandler = webHandler

		log.Printf("Web模式已启用，界面地址: %s", apiServer.UIAddr)
		log.Printf("如果Web界面无法显示，请先运行: ./build_web.sh")
	} else {
		// CLI模式使用CLIHandler
		log.Printf("启动CLI模式...")

		cliHandler := handlers.NewCLIHandler(cfg.Verbose, cfg.DumpTraffic)
		statsReporter := handlers.NewStatsReporter(cliHandler, 10*time.Second)

		// 启动统计报告
		statsReporter.Start()

		// 设置CLI处理器为事件处理器
		eventHandler = cliHandler

		// 在函数返回时停止统计报告
		defer statsReporter.Stop()
	}

	// 创建服务器配置
	serverConfig := proxy.ServerConfig{
		Addr:          listenAddr,
		CertManager:   certManager,
		Verbose:       cfg.Verbose,
		HarLogger:     harLogger,
		UpstreamProxy: upstreamProxyURL,
		DumpTraffic:   cfg.DumpTraffic,
		EventHandler:  eventHandler,
	}

	// 初始化并启动代理服务器
	proxyServer := proxy.NewServerWithConfig(serverConfig)

	// 如果启用了流量输出
	if cfg.DumpTraffic {
		fmt.Println("Traffic dump enabled - HTTP request and response content will be displayed in console")
	}

	// Log MITM mode status
	log.Printf("MITM mode enabled - HTTPS traffic will be decrypted and inspected")
	log.Printf("Make sure to add the CA certificate to your browser/system trust store")
	log.Printf("You can export the CA certificate using the -export-ca flag")
	caCertPath := certs.MustGetCACertPath()
	log.Printf("CA certificate is located at: %s", caCertPath)
	log.Printf("For curl, you can use: curl --cacert %s --proxy http://%s https://example.com", caCertPath, listenAddr)

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
