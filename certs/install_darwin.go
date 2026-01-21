//go:build darwin

package certs

import (
    "bytes"
    "crypto/x509"
    "encoding/pem"
    "fmt"
    "log"
    "os"
    "os/exec"
    "strconv"
    "strings"
)

// runWithAdmin 使用 AppleScript 的 "do shell script ... with administrator privileges"
// 来弹出系统认证对话框并以 root 权限执行 shellCmd（例如 security ...）。
func runWithAdmin(args ...string) (string, error) {
	shellCmd := strings.Join(args, " ")
	// Quote the shell command so that special chars are safe inside the AppleScript string
	quotedCmd := strconv.Quote(shellCmd) // returns a double-quoted Go string literal, suitable for embedding

	// 构造 AppleScript：do shell script "the command" with administrator privileges
	// 注意：我们将 quotedCmd 的内容直接嵌入到 -e 参数中
	as := fmt.Sprintf("do shell script %s with administrator privileges", quotedCmd)

	// 调用 osascript
	cmd := exec.Command("osascript", "-e", as)

	stdout, err := cmd.CombinedOutput()

	if err != nil {
		// 如果用户取消认证，通常会在 stderr 或命令返回的错误中看到 "User canceled" 或类似信息
		// 返回更详细的错误信息给调用者
		return string(stdout), fmt.Errorf("osascript exit: %v; stderr: %s", err, string(stdout))
	}
	return string(stdout), nil
}

func runWithAdminShell(shellCmd string) (string, error) {
	quotedCmd := strconv.Quote(shellCmd)
	as := fmt.Sprintf("do shell script %s with administrator privileges", quotedCmd)
	cmd := exec.Command("osascript", "-e", as)
	stdout, err := cmd.CombinedOutput()
	if err != nil {
		return string(stdout), fmt.Errorf("osascript exit: %v; stderr: %s", err, string(stdout))
	}
	return string(stdout), nil
}

const systemKeychain = "/Library/Keychains/System.keychain"

// isInstalled checks if the exact CA certificate from ~/.proxycraft is already installed and trusted in the system trust store.
func isInstalled() (bool, error) {
	// Get the current CA certificate from ~/.proxycraft
	certPath := MustGetCACertPath()
	currentCertPEM, err := os.ReadFile(certPath)
	if err != nil {
		return false, fmt.Errorf("failed to read current CA certificate: %w", err)
	}
	currentBlock, _ := pem.Decode(currentCertPEM)
	if currentBlock == nil || currentBlock.Type != "CERTIFICATE" {
		return false, fmt.Errorf("failed to decode PEM block containing certificate from %s", certPath)
	}
	currentCert, err := x509.ParseCertificate(currentBlock.Bytes)
	if err != nil {
		return false, fmt.Errorf("failed to parse current CA certificate: %w", err)
	}

	// Get all certificates from the system keychain with our issuer name
	cmd := exec.Command("security", "find-certificate", "-c", IssuerName, "-p", systemKeychain)
	installedCertsPEM, err := cmd.CombinedOutput()
	if err != nil {
		return false, nil // No certificate found with that name
	}

	remaining := installedCertsPEM
	for len(remaining) > 0 {
		var block *pem.Block
		block, remaining = pem.Decode(remaining)
		if block == nil {
			break
		}
		if block.Type != "CERTIFICATE" {
			continue
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			continue
		}
		if bytes.Equal(cert.Raw, currentCert.Raw) {
			return true, nil
		}
	}

	// No matching certificate found or it's not trusted
	return false, nil
}

// InstallCerts installs the CA certificate to the system trust store on macOS.
// It requires sudo privileges. If a certificate with the same name but different content exists,
// it will be removed first to prevent conflicts.
func install() error {
	// Check if the exact certificate is already installed
	installed, err := isInstalled()
	if err != nil {
		return fmt.Errorf("failed to check if certificate is installed: %w", err)
	}

	if installed {
		fmt.Println("CA certificate is already installed in the system keychain. Skipping installation.")
		return nil
	}

	return installForce()
}

func installForce() error {
	certPath := MustGetCACertPath()

	// On macOS, we need to use the `security` command to manage certificates.
	// First, remove any existing certificate with the same name to prevent conflicts
	fmt.Println("Attempting to install CA certificate into system keychain...")
	fmt.Println("You might be prompted for your password.")

	// Remove any existing certificate with our issuer name
	// This is safe because we already checked that the exact certificate isn't installed
	deleteCmd := fmt.Sprintf("security delete-certificate -c %s %s >/dev/null 2>&1 || true", strconv.Quote(IssuerName), strconv.Quote(systemKeychain))
	installCmd := fmt.Sprintf("security add-trusted-cert -d -r trustRoot -k %s %s", strconv.Quote(systemKeychain), strconv.Quote(certPath))
	cmdOutput, err := runWithAdminShell(deleteCmd + "; " + installCmd)
	if err != nil {
		log.Printf("Automatic install via osascript failed: %v", err)
		sudoCmd := exec.Command("sudo", "/bin/sh", "-c", deleteCmd+"; "+installCmd)
		sudoOutput, sudoErr := sudoCmd.CombinedOutput()
		if sudoErr != nil {
			return fmt.Errorf("failed to install certificate via osascript and sudo: %w; output: %s", sudoErr, string(sudoOutput))
		}
		log.Println(string(sudoOutput))
		return nil
	}

	log.Println(cmdOutput)

	return nil
}

// uninstall uninstalls the CA certificate from the system trust store on macOS.
// It requires sudo privileges.
func uninstall() error {
	cmdOutput, err := runWithAdmin("security", "delete-certificate", "-c", IssuerName, systemKeychain)
	if err != nil {
		return fmt.Errorf("failed to uninstall certificate. command finished with error: %w", err)
	}

	log.Println(cmdOutput)

	return nil
}
