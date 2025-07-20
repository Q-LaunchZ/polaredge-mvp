package traefik

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const fallbackVersion = "v3.4.2"
const binPath = "./bin/traefik"

func IsInstalled() bool {
	_, err := os.Stat(binPath)
	return err == nil
}

func Install() error {
	fmt.Printf("🧠 Detected OS: %s, ARCH: %s\n", runtime.GOOS, runtime.GOARCH)

	url, version, err := getLatestTraefikURL()
	if err != nil {
		return fmt.Errorf("❌ Failed to fetch Traefik release: %w", err)
	}

	fmt.Printf("⬇️  Downloading Traefik %s from %s...\n", version, url)

	if err := downloadAndExtract(url, binPath); err != nil {
		return fmt.Errorf("install failed: %w", err)
	}

	fmt.Println("✅ Traefik installed to", binPath)
	return nil
}

func getLatestTraefikURL() (string, string, error) {
	resp, err := http.Get("https://api.github.com/repos/traefik/traefik/releases")
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	var releases []struct {
		TagName    string `json:"tag_name"`
		Prerelease bool   `json:"prerelease"`
		Draft      bool   `json:"draft"`
		Assets     []struct {
			BrowserDownloadURL string `json:"browser_download_url"`
			Name               string `json:"name"`
		}
	}

	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return "", "", err
	}

	osName := runtime.GOOS
	arch := runtime.GOARCH
	var suffix string

	switch osName {
	case "darwin":
		if arch == "arm64" {
			suffix = "darwin_arm64.tar.gz"
		} else {
			suffix = "darwin_amd64.tar.gz"
		}
	case "linux":
		if arch == "arm64" {
			suffix = "linux_arm64.tar.gz"
		} else {
			suffix = "linux_amd64.tar.gz"
		}
	default:
		return "", "", fmt.Errorf("unsupported platform: %s/%s", osName, arch)
	}

	for _, release := range releases {
		if release.Draft || release.Prerelease {
			continue
		}
		for _, asset := range release.Assets {
			if strings.HasSuffix(asset.Name, suffix) {
				fmt.Printf("🔍 Selected version: %s (%s)\n", release.TagName, asset.Name)
				return asset.BrowserDownloadURL, release.TagName, nil
			}
		}
	}

	return "", "", errors.New("no matching asset found for your platform")
}

func downloadAndExtract(url, destPath string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("bad HTTP response: %d %s — %s", resp.StatusCode, resp.Status, string(body))
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType != "application/octet-stream" && !strings.Contains(contentType, "gzip") {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("unexpected content-type: %s\nBody: %s", contentType, string(body))
	}

	gzr, err := gzip.NewReader(resp.Body)
	if err != nil {
		return fmt.Errorf("gzip error: %w", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return err
	}

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if filepath.Base(hdr.Name) == "traefik" {
			out, err := os.Create(destPath)
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			out.Close()
			return os.Chmod(destPath, 0755)
		}
	}

	return errors.New("traefik binary not found in archive")
}

func GetBinaryPath() string {
	return binPath
}

func RunWithConfig(configPath string) error {
	cmd := exec.Command(GetBinaryPath(), "--configFile", configPath)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe error: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("stderr pipe error: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("traefik failed to start: %w", err)
	}

	go io.Copy(os.Stdout, stdout)
	go io.Copy(os.Stderr, stderr)

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("traefik exited with error: %w", err)
	}

	fmt.Println("🚀 Traefik exited cleanly.")
	return nil
}

func Verify() error {
	if _, err := os.Stat(binPath); err != nil {
		return fmt.Errorf("traefik binary missing: %w", err)
	}

	cmd := exec.Command(GetBinaryPath(), "version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("traefik version failed: %w", err)
	}

	fmt.Printf("🔎 Traefik OK: %s\n", strings.TrimSpace(string(output)))
	return nil
}
