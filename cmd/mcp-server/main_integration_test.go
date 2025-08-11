package main

import (
	"context"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/JamesPrial/mcp-memory-core/internal/knowledge"
	"github.com/JamesPrial/mcp-memory-core/internal/storage"
	"github.com/JamesPrial/mcp-memory-core/internal/transport"
	"github.com/JamesPrial/mcp-memory-core/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMainWithValidConfig tests the main function with valid configuration
func TestMainWithValidConfig(t *testing.T) {
	// Skip if this is running as part of main execution
	if os.Getenv("TEST_MAIN") == "" {
		t.Skip("Skipping main test - set TEST_MAIN=1 to run")
	}

	// Create temp directory and config file
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "test-config.yaml")
	
	configContent := `storageType: "memory"
httpPort: 8080
logLevel: "info"`
	
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	// Save original args and flags
	originalArgs := os.Args
	originalCommandLine := flag.CommandLine
	
	// Reset flag.CommandLine for this test
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

	defer func() {
		os.Args = originalArgs
		flag.CommandLine = originalCommandLine
	}()

	// Set up args to simulate command line
	os.Args = []string{"test-server", "-config", configPath}
	
	// We can't directly test main() since it runs forever, but we can test
	// the initialization part by calling the same logic
	t.Run("config loading and server creation", func(t *testing.T) {
		// This tests the same logic as main() but without the infinite Run() loop
		var configPathFlag string
		flag.StringVar(&configPathFlag, "config", "config.yaml", "Path to configuration file")
		flag.Parse()

		// Should parse the config path correctly
		assert.Equal(t, configPath, configPathFlag)
	})
}

// TestMainFunctionParts tests individual parts of the main function logic
func TestMainFunctionParts(t *testing.T) {
	t.Run("invalid config path", func(t *testing.T) {
		// Test with non-existent config file - this would cause main() to exit
		tempDir := t.TempDir()
		nonExistentPath := filepath.Join(tempDir, "nonexistent.yaml")

		// We can't directly test main() failure, but we can test the config loading
		// that main() uses
		_, err := loadConfigSafely(nonExistentPath)
		assert.Error(t, err, "Should fail with non-existent config")
	})

	t.Run("valid memory config", func(t *testing.T) {
		tempDir := t.TempDir()
		configPath := filepath.Join(tempDir, "memory-config.yaml")
		
		configContent := `storageType: "memory"
httpPort: 8080
logLevel: "info"`
		
		err := os.WriteFile(configPath, []byte(configContent), 0644)
		require.NoError(t, err)

		cfg, err := loadConfigSafely(configPath)
		require.NoError(t, err)
		assert.Equal(t, "memory", cfg.StorageType)
	})

	t.Run("valid sqlite config", func(t *testing.T) {
		tempDir := t.TempDir()
		dbPath := filepath.Join(tempDir, "test.db")
		configPath := filepath.Join(tempDir, "sqlite-config.yaml")
		
		configContent := `storageType: "sqlite"
storagePath: "` + dbPath + `"
httpPort: 8080
logLevel: "info"
sqlite:
  walMode: true`
		
		err := os.WriteFile(configPath, []byte(configContent), 0644)
		require.NoError(t, err)

		cfg, err := loadConfigSafely(configPath)
		require.NoError(t, err)
		assert.Equal(t, "sqlite", cfg.StorageType)
		assert.Equal(t, dbPath, cfg.StoragePath)
	})
}

// TestServerInitialization tests the server initialization process
func TestServerInitialization(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	
	configContent := `storageType: "memory"
httpPort: 8080
logLevel: "info"`
	
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	// Test the server initialization that happens in main()
	cfg, err := loadConfigSafely(configPath)
	require.NoError(t, err)

	backend, err := createBackendSafely(cfg)
	require.NoError(t, err)
	require.NotNil(t, backend)
	defer backend.Close()

	manager, server := createServerComponents(backend)
	require.NotNil(t, manager)
	require.NotNil(t, server)

	// Test that server can handle a basic request
	ctx := context.Background()
	req := &transport.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/list",
	}
	resp := server.HandleRequest(ctx, req)
	assert.NotNil(t, resp.Result)
	assert.Nil(t, resp.Error)
}

// TestServerWithTransport tests the server with transport layer
func TestServerWithTransport(t *testing.T) {
	server := createTestServer(t)

	// Create stdio transport
	trans := transport.NewStdioTransport()
	require.NotNil(t, trans)
	assert.Equal(t, "stdio", trans.Name())

	// Test that server can handle requests through transport
	ctx := context.Background()
	
	// Test request handling
	req := &transport.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/list",
	}

	resp := server.HandleRequest(ctx, req)
	assert.NotNil(t, resp)
	assert.Nil(t, resp.Error)
	assert.NotNil(t, resp.Result)

	// Test stopping transport
	err := trans.Stop(ctx)
	assert.NoError(t, err)
}

// Helper functions to safely test parts of main() logic without causing exits

func loadConfigSafely(configPath string) (*config.Settings, error) {
	// This duplicates the config loading logic from main() but returns errors
	// instead of calling log.Fatalf
	return config.Load(configPath)
}

func createBackendSafely(cfg *config.Settings) (storage.Backend, error) {
	// This duplicates the backend creation logic from main()
	return storage.NewBackend(cfg)
}

func createServerComponents(backend storage.Backend) (*knowledge.Manager, *Server) {
	// This duplicates the server component creation from main()
	manager := knowledge.NewManager(backend)
	server := NewServer(manager)
	return manager, server
}