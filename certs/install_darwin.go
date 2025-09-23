//go:build darwin

package certs

import (
	"fmt"
	"log"
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

// isInstalled checks if the CA certificate is already installed in the system trust store.
func isInstalled() (bool, error) {
	// Use security command to find the certificate in the system keychain
	// We search by the certificate's common name. Reading from system keychain doesn't require sudo.
	cmdOutput, err := runWithAdmin("security", "find-certificate", "-c", fmt.Sprintf(`"%s"`, IssuerName), "/Library/Keychains/System.keychain")
	if err != nil {
		return false, err
	}
	// log.Println(cmd.Args)

	if strings.Contains(cmdOutput, "The specified item could not be found in the keychain.") {
		return false, nil
	}

	if strings.Contains(cmdOutput, "version:") && strings.Contains(cmdOutput, "keychain:") {
		return true, nil
	}

	return true, nil
}

// InstallCerts installs the CA certificate to the system trust store on macOS.
// It requires sudo privileges. If the certificate is already installed, it will skip the installation.
func install() error {
	// Check if the certificate is already installed
	installed, err := isInstalled()
	if err != nil {
		return fmt.Errorf("failed to check if certificate is installed: %w", err)
	}

	if installed {
		fmt.Println("CA certificate is already installed in the system keychain. Skipping installation.")
		return nil
	}

	certPath := MustGetCACertPath()

	// On macOS, we need to use the `security` command to install the cert.
	// This requires sudo.
	fmt.Println("Attempting to install CA certificate into system keychain...")
	fmt.Println("You might be prompted for your password.")

	cmdOutput, err := runWithAdmin("security", "add-trusted-cert", "-d", "-r", "trustRoot", "-k", "/Library/Keychains/System.keychain", certPath)
	if err != nil {
		return fmt.Errorf("failed to install certificate. command finished with error: %w. Make sure you have sudo privileges", err)
	}

	log.Println(cmdOutput)

	return nil
}

// uninstall uninstalls the CA certificate from the system trust store on macOS.
// It requires sudo privileges.
func uninstall() error {
	cmdOutput, err := runWithAdmin("security", "delete-certificate", "-c", IssuerName, "/Library/Keychains/System.keychain")
	if err != nil {
		return fmt.Errorf("failed to uninstall certificate. command finished with error: %w", err)
	}

	log.Println(cmdOutput)

	return nil
}
