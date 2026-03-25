package main

import (
	"bufio"
	"fmt"
	"os"
	"runtime"
	"strings"
)

func loadEnv() {
	f, err := os.Open(".env")
	if err != nil {
		return
	}
	defer f.Close()

	// Check file permissions on Unix systems
	if runtime.GOOS != "windows" {
		info, err := f.Stat()
		if err == nil {
			mode := info.Mode().Perm()
			if mode&0o077 != 0 {
				fmt.Fprintf(os.Stderr, "Warning: .env file has permissions %o — should be 600 (owner-only). Fix with: chmod 600 .env\n", mode)
			}
		}
	}

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		if key != "" && val != "" {
			if os.Getenv(key) == "" {
				os.Setenv(key, val)
			}
		}
	}
}
