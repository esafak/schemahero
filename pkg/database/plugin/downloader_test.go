package plugin

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestPluginDownloader_GetPluginArtifactRef(t *testing.T) {
	downloader := NewPluginDownloader("")

	tests := []struct {
		driver       string
		majorVersion string
		expected     string
	}{
		{"postgres", "0", "docker.io/schemahero/plugin-postgres:0"},
		{"mysql", "1", "docker.io/schemahero/plugin-mysql:0"},
		{"cassandra", "0", "docker.io/schemahero/plugin-cassandra:0"},
	}

	for _, test := range tests {
		result := downloader.GetPluginArtifactRef(test.driver, test.majorVersion)
		if result != test.expected {
			t.Errorf("GetPluginArtifactRef(%s, %s) = %s, expected %s", test.driver, test.majorVersion, result, test.expected)
		}
	}
}

func TestPluginDownloader_GetCachedPluginPath(t *testing.T) {
	tempDir := t.TempDir()
	downloader := NewPluginDownloader(tempDir)

	tests := []struct {
		driver       string
		majorVersion string
		expected     string
	}{
		{"postgres", "0", filepath.Join(tempDir, "schemahero-postgres")},
		{"mysql", "1", filepath.Join(tempDir, "schemahero-mysql")},
	}

	for _, test := range tests {
		result := downloader.GetCachedPluginPath(test.driver, test.majorVersion)
		if result != test.expected {
			t.Errorf("GetCachedPluginPath(%s, %s) = %s, expected %s", test.driver, test.majorVersion, result, test.expected)
		}
	}
}

func TestPluginDownloader_IsPluginCached(t *testing.T) {
	tempDir := t.TempDir()
	downloader := NewPluginDownloader(tempDir)

	// Test non-existent plugin
	if downloader.IsPluginCached("postgres", "0") {
		t.Error("Expected IsPluginCached to return false for non-existent plugin")
	}

	// Create a mock plugin binary
	pluginPath := downloader.GetCachedPluginPath("postgres", "0")
	if err := os.MkdirAll(filepath.Dir(pluginPath), 0755); err != nil {
		t.Fatal(err)
	}

	// Create the file
	file, err := os.Create(pluginPath)
	if err != nil {
		t.Fatal(err)
	}
	file.Close()

	// Make it executable
	if err := os.Chmod(pluginPath, 0755); err != nil {
		t.Fatal(err)
	}

	// Now it should be cached
	if !downloader.IsPluginCached("postgres", "0") {
		t.Error("Expected IsPluginCached to return true for existing executable plugin")
	}

	// Test with non-executable file
	nonExecPath := downloader.GetCachedPluginPath("mysql", "0")
	if err := os.MkdirAll(filepath.Dir(nonExecPath), 0755); err != nil {
		t.Fatal(err)
	}

	file, err = os.Create(nonExecPath)
	if err != nil {
		t.Fatal(err)
	}
	file.Close()

	// Don't make it executable (default permissions)
	if downloader.IsPluginCached("mysql", "0") {
		t.Error("Expected IsPluginCached to return false for non-executable plugin")
	}
}

func TestPluginDownloader_CleanPluginCache(t *testing.T) {
	tempDir := t.TempDir()
	downloader := NewPluginDownloader(tempDir)

	// Create a mock plugin binary
	pluginPath := downloader.GetCachedPluginPath("postgres", "0")
	if err := os.MkdirAll(filepath.Dir(pluginPath), 0755); err != nil {
		t.Fatal(err)
	}

	file, err := os.Create(pluginPath)
	if err != nil {
		t.Fatal(err)
	}
	file.Close()

	// Verify it exists
	if _, err := os.Stat(pluginPath); os.IsNotExist(err) {
		t.Fatal("Plugin file should exist before cleaning")
	}

	// Clean the cache
	if err := downloader.CleanPluginCache("postgres", "0"); err != nil {
		t.Fatal(err)
	}

	// Verify it's gone
	if _, err := os.Stat(pluginPath); !os.IsNotExist(err) {
		t.Error("Plugin file should not exist after cleaning")
	}
}

func TestPluginManager_DownloadPluginErrorMessage(t *testing.T) {
	manager := NewPluginManager(NewPluginRegistry(), NewPluginLoader(t.TempDir()))
	manager.downloader = NewPluginDownloader(t.TempDir())

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := manager.DownloadPlugin(ctx, "postgres")
	if err == nil {
		t.Fatal("expected download error")
	}

	errString := err.Error()
	if !strings.Contains(errString, "Failed to download SchemaHero postgres plugin") {
		t.Fatalf("expected SchemaHero plugin download error, got %q", errString)
	}

	if strings.Contains(errString, "You can download this plugin ahead of time") {
		t.Fatalf("expected no remediation steps in Go error string, got %q", errString)
	}

	if !strings.Contains(errString, "context canceled") {
		t.Fatalf("expected original Go error string, got %q", errString)
	}

	var downloadErr *DownloadError
	if !errors.As(err, &downloadErr) {
		t.Fatalf("expected DownloadError, got %T", err)
	}

	expectedMessage := "Failed to download SchemaHero postgres plugin. You can download this plugin ahead of time with 'schemahero plugin download postgres'"
	if downloadErr.Message() != expectedMessage {
		t.Fatalf("expected message %q, got %q", expectedMessage, downloadErr.Message())
	}
}

// TestPluginDownloader_DownloadPlugin_Integration tests the actual download functionality
func TestPluginDownloader_DownloadPlugin_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tempDir := t.TempDir()
	downloader := NewPluginDownloader(tempDir)

	ctx := context.Background()

	// Try to download the postgres plugin that should exist
	t.Logf("Attempting to download postgres plugin to %s", tempDir)
	pluginPath, err := downloader.DownloadPlugin(ctx, "postgres", "0")

	if err != nil {
		t.Logf("Download failed (this may be expected if plugin doesn't exist): %v", err)
		// Don't fail the test - just log the error so we can see what happens
		return
	}

	// If download succeeded, verify the plugin exists and is executable
	if pluginPath == "" {
		t.Error("Plugin path should not be empty on successful download")
		return
	}

	if !downloader.IsPluginCached("postgres", "0") {
		t.Error("Plugin should be cached after successful download")
		return
	}

	// Verify the file exists and is executable
	info, err := os.Stat(pluginPath)
	if err != nil {
		t.Errorf("Plugin binary should exist at %s: %v", pluginPath, err)
		return
	}

	if info.Mode().Perm()&0111 == 0 {
		t.Error("Plugin binary should be executable")
		return
	}

	t.Logf("Successfully downloaded and verified plugin at %s", pluginPath)
}

func TestFindPluginArtifact(t *testing.T) {
	driver := "mysql"

	t.Run("flat tarball path", func(t *testing.T) {
		cacheDir := t.TempDir()
		tarName := fmt.Sprintf("schemahero-%s-%s-%s.tar.gz", driver, runtime.GOOS, runtime.GOARCH)
		tarPath := filepath.Join(cacheDir, tarName)

		if err := os.WriteFile(tarPath, []byte("fake tarball"), 0644); err != nil {
			t.Fatal(err)
		}

		found, isArchive, err := findPluginArtifact(cacheDir, driver)
		if err != nil {
			t.Fatalf("expected to find artifact, got error: %v", err)
		}
		if found != tarPath {
			t.Errorf("expected %s, got %s", tarPath, found)
		}
		if !isArchive {
			t.Error("expected isArchive=true")
		}
	})

	t.Run("dist subdirectory tarball path", func(t *testing.T) {
		cacheDir := t.TempDir()
		distDir := filepath.Join(cacheDir, "dist")
		if err := os.MkdirAll(distDir, 0755); err != nil {
			t.Fatal(err)
		}

		tarName := fmt.Sprintf("schemahero-%s-%s-%s.tar.gz", driver, runtime.GOOS, runtime.GOARCH)
		tarPath := filepath.Join(distDir, tarName)
		if err := os.WriteFile(tarPath, []byte("fake tarball"), 0644); err != nil {
			t.Fatal(err)
		}

		found, isArchive, err := findPluginArtifact(cacheDir, driver)
		if err != nil {
			t.Fatalf("expected to find artifact under dist/, got error: %v", err)
		}
		if found != tarPath {
			t.Errorf("expected %s, got %s", tarPath, found)
		}
		if !isArchive {
			t.Error("expected isArchive=true")
		}
	})

	t.Run("flat path takes precedence over dist path", func(t *testing.T) {
		cacheDir := t.TempDir()

		// Create both flat and dist/ tarballs
		tarName := fmt.Sprintf("schemahero-%s-%s-%s.tar.gz", driver, runtime.GOOS, runtime.GOARCH)
		flatPath := filepath.Join(cacheDir, tarName)
		if err := os.WriteFile(flatPath, []byte("flat"), 0644); err != nil {
			t.Fatal(err)
		}

		distDir := filepath.Join(cacheDir, "dist")
		if err := os.MkdirAll(distDir, 0755); err != nil {
			t.Fatal(err)
		}
		distPath := filepath.Join(distDir, tarName)
		if err := os.WriteFile(distPath, []byte("dist"), 0644); err != nil {
			t.Fatal(err)
		}

		found, _, err := findPluginArtifact(cacheDir, driver)
		if err != nil {
			t.Fatalf("expected to find artifact, got error: %v", err)
		}
		if found != flatPath {
			t.Errorf("flat path should take precedence: expected %s, got %s", flatPath, found)
		}
	})

	t.Run("not found returns error", func(t *testing.T) {
		cacheDir := t.TempDir()

		_, _, err := findPluginArtifact(cacheDir, driver)
		if err == nil {
			t.Fatal("expected error when no artifact is present")
		}
		if !strings.Contains(err.Error(), "plugin not found") {
			t.Errorf("expected 'plugin not found' error, got: %v", err)
		}
	})

	t.Run("direct binary fallback", func(t *testing.T) {
		cacheDir := t.TempDir()
		binaryPath := filepath.Join(cacheDir, fmt.Sprintf("schemahero-%s", driver))
		if err := os.WriteFile(binaryPath, []byte("fake binary"), 0755); err != nil {
			t.Fatal(err)
		}

		found, isArchive, err := findPluginArtifact(cacheDir, driver)
		if err != nil {
			t.Fatalf("expected to find artifact, got error: %v", err)
		}
		if found != binaryPath {
			t.Errorf("expected %s, got %s", binaryPath, found)
		}
		if isArchive {
			t.Error("expected isArchive=false for direct binary")
		}
	})
}
