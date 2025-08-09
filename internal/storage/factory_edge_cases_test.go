package storage

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/JamesPrial/mcp-memory-core/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewBackend_EdgeCases_InvalidConfigs(t *testing.T) {
	t.Run("NilConfig", func(t *testing.T) {
		// Should panic when trying to access nil config
		defer func() {
			if r := recover(); r != nil {
				assert.Contains(t, fmt.Sprintf("%v", r), "nil pointer")
			}
		}()
		
		backend, err := NewBackend(nil)
		if err == nil && backend != nil {
			t.Error("Expected error or panic when passing nil config")
		}
	})

	t.Run("EmptyConfig", func(t *testing.T) {
		cfg := &config.Settings{}
		backend, err := NewBackend(cfg)
		
		// Should default to memory backend when StorageType is empty
		assert.NoError(t, err)
		assert.NotNil(t, backend)
		assert.IsType(t, &MemoryBackend{}, backend)
	})

	t.Run("InvalidStorageTypes", func(t *testing.T) {
		invalidTypes := []string{
			"invalid",
			"SQLITE",          // Wrong case
			"Memory",          // Wrong case
			"postgresql",      // Not supported
			"redis",           // Not supported
			"file",            // Not supported
			"mysql",           // Not supported
			"",                // Should default to memory
			"sqlite_invalid",  // Invalid variant
			"memory_invalid",  // Invalid variant
			"sqlite3",         // Close but wrong
			"sql",             // Partial
			"mem",             // Partial
			"\x00sqlite",      // With control chars
			"sqlite\x00",      // With control chars
			"sqlite\n",        // With newline
			"sqlite\t",        // With tab
			" sqlite ",        // With spaces
			"🦄invalid",       // With unicode
			"'; DROP TABLE backends; --", // SQL injection attempt
		}

		for _, storageType := range invalidTypes {
			t.Run(fmt.Sprintf("Type_%s", storageType), func(t *testing.T) {
				cfg := &config.Settings{
					StorageType: storageType,
					StoragePath: "/valid/path",
				}
				
				backend, err := NewBackend(cfg)
				
				if storageType == "" {
					// Empty string should default to memory
					assert.NoError(t, err)
					assert.IsType(t, &MemoryBackend{}, backend)
				} else {
					// All other invalid types should error
					assert.Error(t, err)
					assert.Nil(t, backend)
					assert.Contains(t, err.Error(), "unsupported storage type")
				}
			})
		}
	})
}

func TestNewBackend_EdgeCases_SQLiteConfig(t *testing.T) {
	t.Run("EmptyStoragePath", func(t *testing.T) {
		cfg := &config.Settings{
			StorageType: "sqlite",
			StoragePath: "",
		}
		
		backend, err := NewBackend(cfg)
		assert.Error(t, err)
		assert.Nil(t, backend)
		assert.Contains(t, err.Error(), "storage path is required for SQLite backend")
	})

	t.Run("OnlyWhitespaceStoragePath", func(t *testing.T) {
		whitespacePaths := []string{
			"   ",
			"\t",
			"\n",
			"\r",
			"\t\n\r   ",
		}

		for _, path := range whitespacePaths {
			t.Run(fmt.Sprintf("Path_%q", path), func(t *testing.T) {
				cfg := &config.Settings{
					StorageType: "sqlite",
					StoragePath: path,
				}
				
				// This should not error at config validation level,
				// but might fail when actually trying to create the database
				backend, err := NewBackend(cfg)
				// The behavior depends on SQLite implementation
				// It might succeed or fail, but should not panic
				if err != nil {
					assert.Nil(t, backend)
				}
			})
		}
	})

	t.Run("VeryLongStoragePath", func(t *testing.T) {
		// Create a path that's longer than most filesystem limits
		longPath := "/" + strings.Repeat("very_long_directory_name", 100) + "/database.db"
		
		cfg := &config.Settings{
			StorageType: "sqlite",
			StoragePath: longPath,
		}
		
		backend, err := NewBackend(cfg)
		// This might succeed or fail depending on the filesystem
		// But it shouldn't panic
		if err != nil {
			assert.Nil(t, backend)
		}
	})

	t.Run("SpecialCharacterPaths", func(t *testing.T) {
		specialPaths := []string{
			"/path/with spaces/db.sqlite",
			"/path/with-dashes/db.sqlite",
			"/path/with_underscores/db.sqlite",
			"/path/with.dots/db.sqlite",
			"/path/with(parentheses)/db.sqlite",
			"/path/with[brackets]/db.sqlite",
			"/path/with{braces}/db.sqlite",
			"/path/with'apostrophes'/db.sqlite",
			"/path/with\"quotes\"/db.sqlite",
			"/path/with;semicolons;/db.sqlite",
			"/path/with,commas,/db.sqlite",
			"/tmp/测试数据库.sqlite",      // Unicode filename
			"/tmp/🦄unicorn.sqlite",     // Emoji filename
		}

		for _, path := range specialPaths {
			t.Run(fmt.Sprintf("Path_%s", filepath.Base(path)), func(t *testing.T) {
				cfg := &config.Settings{
					StorageType: "sqlite",
					StoragePath: path,
				}
				
				backend, err := NewBackend(cfg)
				// These might succeed or fail depending on the filesystem and OS
				// But they shouldn't cause panics
				if err != nil {
					assert.Nil(t, backend)
				} else {
					assert.NotNil(t, backend)
					// Clean up if successful
					if sqliteBackend, ok := backend.(*SqliteBackend); ok {
						sqliteBackend.Close()
					}
				}
			})
		}
	})

	t.Run("InvalidPathCharacters", func(t *testing.T) {
		invalidPaths := []string{
			"\x00/null/byte/path.sqlite",
			"/path/with\x00null/byte.sqlite",
			"/path\x01\x02\x03control.sqlite",
			"relative/path.sqlite",           // Relative path
			"",                               // Empty (already tested but included for completeness)
			"../../../etc/passwd",            // Directory traversal attempt
			"/dev/null",                      // Special device file
			"/proc/self/mem",                 // Special system file
			"CON.sqlite",                     // Windows reserved name
			"PRN.sqlite",                     // Windows reserved name
			"AUX.sqlite",                     // Windows reserved name
			"NUL.sqlite",                     // Windows reserved name
		}

		for _, path := range invalidPaths {
			t.Run(fmt.Sprintf("InvalidPath_%d", len(path)), func(t *testing.T) {
				cfg := &config.Settings{
					StorageType: "sqlite",
					StoragePath: path,
				}
				
				backend, err := NewBackend(cfg)
				// Most of these should fail, but some might succeed depending on OS
				// The important thing is no panics
				if backend != nil && err == nil {
					// Clean up if it somehow succeeded
					if sqliteBackend, ok := backend.(*SqliteBackend); ok {
						sqliteBackend.Close()
					}
				}
			})
		}
	})

	t.Run("WALModeVariations", func(t *testing.T) {
		walModes := []bool{true, false}
		
		for _, walMode := range walModes {
			t.Run(fmt.Sprintf("WALMode_%t", walMode), func(t *testing.T) {
				// Use a temporary file path
				tempPath := filepath.Join(t.TempDir(), "test.db")
				
				cfg := &config.Settings{
					StorageType: "sqlite",
					StoragePath: tempPath,
					Sqlite: config.SqliteSettings{
						WALMode: walMode,
					},
				}
				
				backend, err := NewBackend(cfg)
				if err != nil {
					t.Logf("Failed to create backend with WAL mode %t: %v", walMode, err)
				} else {
					require.NotNil(t, backend)
					assert.IsType(t, &SqliteBackend{}, backend)
					
					// Clean up
					if sqliteBackend, ok := backend.(*SqliteBackend); ok {
						sqliteBackend.Close()
					}
				}
			})
		}
	})

	t.Run("PermissionDeniedPaths", func(t *testing.T) {
		restrictedPaths := []string{
			"/root/database.sqlite",          // Typically restricted
			"/etc/database.sqlite",           // System directory
			"/var/log/database.sqlite",       // Log directory
			"/usr/database.sqlite",           // System directory
			"/sys/database.sqlite",           // System filesystem
			"/proc/database.sqlite",          // Process filesystem
		}

		for _, path := range restrictedPaths {
			t.Run(fmt.Sprintf("RestrictedPath_%s", filepath.Base(path)), func(t *testing.T) {
				cfg := &config.Settings{
					StorageType: "sqlite",
					StoragePath: path,
				}
				
				backend, err := NewBackend(cfg)
				// These should likely fail due to permissions, but shouldn't panic
				if err != nil {
					assert.Nil(t, backend)
					// Don't assert on specific error message as it varies by OS
				} else if backend != nil {
					// If it somehow succeeded, clean up
					if sqliteBackend, ok := backend.(*SqliteBackend); ok {
						sqliteBackend.Close()
					}
				}
			})
		}
	})
}

func TestNewBackend_EdgeCases_MemoryConfig(t *testing.T) {
	t.Run("MemoryWithExtraSettings", func(t *testing.T) {
		cfg := &config.Settings{
			StorageType: "memory",
			StoragePath: "/ignored/path",    // Should be ignored for memory backend
			HTTPPort:    8080,               // Irrelevant to storage backend
			LogLevel:    "debug",            // Irrelevant to storage backend
			Sqlite: config.SqliteSettings{  // Should be ignored for memory backend
				WALMode: true,
			},
		}
		
		backend, err := NewBackend(cfg)
		assert.NoError(t, err)
		assert.NotNil(t, backend)
		assert.IsType(t, &MemoryBackend{}, backend)
	})

	t.Run("MemoryVariantCases", func(t *testing.T) {
		// Test different ways to specify memory backend
		memoryTypes := []string{
			"memory",
			"",  // Empty defaults to memory
		}

		for _, storageType := range memoryTypes {
			t.Run(fmt.Sprintf("Type_%s", storageType), func(t *testing.T) {
				cfg := &config.Settings{
					StorageType: storageType,
				}
				
				backend, err := NewBackend(cfg)
				assert.NoError(t, err)
				assert.NotNil(t, backend)
				assert.IsType(t, &MemoryBackend{}, backend)
			})
		}
	})
}

func TestNewBackend_EdgeCases_BoundaryConditions(t *testing.T) {
	t.Run("MaxPathLength", func(t *testing.T) {
		// Create a path at filesystem limits (typically 4096 on Linux)
		maxPath := "/" + strings.Repeat("a", 4090) + ".db" // 4095 total chars
		
		cfg := &config.Settings{
			StorageType: "sqlite",
			StoragePath: maxPath,
		}
		
		backend, err := NewBackend(cfg)
		// This will likely fail, but shouldn't panic
		if err != nil {
			assert.Nil(t, backend)
		} else if backend != nil {
			// Clean up if it somehow worked
			if sqliteBackend, ok := backend.(*SqliteBackend); ok {
				sqliteBackend.Close()
			}
		}
	})

	t.Run("ConcurrentFactoryCreation", func(t *testing.T) {
		// Test concurrent creation of backends
		const numGoroutines = 50
		results := make(chan struct {
			backend Backend
			err     error
		}, numGoroutines)
		
		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				// Create unique temp path for each goroutine
				tempPath := filepath.Join(t.TempDir(), fmt.Sprintf("concurrent_%d.db", id))
				cfg := &config.Settings{
					StorageType: "sqlite",
					StoragePath: tempPath,
				}
				
				backend, err := NewBackend(cfg)
				results <- struct {
					backend Backend
					err     error
				}{backend, err}
			}(i)
		}
		
		// Collect results
		successCount := 0
		for i := 0; i < numGoroutines; i++ {
			result := <-results
			if result.err == nil && result.backend != nil {
				successCount++
				// Clean up successful backends
				if sqliteBackend, ok := result.backend.(*SqliteBackend); ok {
					sqliteBackend.Close()
				}
			}
		}
		
		// Most should succeed (unless there are resource constraints)
		assert.Greater(t, successCount, 0, "At least some concurrent creations should succeed")
	})

	t.Run("ResourceExhaustion", func(t *testing.T) {
		// Try to create many backends without closing them
		// This tests resource limits and cleanup
		var backends []Backend
		defer func() {
			// Clean up all created backends
			for _, backend := range backends {
				if sqliteBackend, ok := backend.(*SqliteBackend); ok {
					sqliteBackend.Close()
				}
			}
		}()

		maxBackends := 100 // Reasonable limit for testing
		for i := 0; i < maxBackends; i++ {
			tempPath := filepath.Join(t.TempDir(), fmt.Sprintf("resource_test_%d.db", i))
			cfg := &config.Settings{
				StorageType: "sqlite",
				StoragePath: tempPath,
			}
			
			backend, err := NewBackend(cfg)
			if err != nil {
				// Resource exhaustion or other error - this is acceptable
				break
			}
			
			backends = append(backends, backend)
		}
		
		// Should have created at least some backends
		assert.Greater(t, len(backends), 0, "Should create at least some backends before resource exhaustion")
	})
}

func TestNewBackend_EdgeCases_ErrorRecovery(t *testing.T) {
	t.Run("PartialFailureRecovery", func(t *testing.T) {
		// Try to create backend with invalid type, then valid one
		invalidCfg := &config.Settings{
			StorageType: "invalid_type",
			StoragePath: "/tmp/test.db",
		}
		
		backend, err := NewBackend(invalidCfg)
		assert.Error(t, err)
		assert.Nil(t, backend)
		
		// Now try with valid config
		validCfg := &config.Settings{
			StorageType: "memory",
		}
		
		backend, err = NewBackend(validCfg)
		assert.NoError(t, err)
		assert.NotNil(t, backend)
		assert.IsType(t, &MemoryBackend{}, backend)
	})

	t.Run("ErrorMessageContent", func(t *testing.T) {
		// Test that error messages contain useful information
		testCases := []struct {
			name        string
			cfg         *config.Settings
			expectedErr string
		}{
			{
				name: "UnsupportedType",
				cfg: &config.Settings{
					StorageType: "unsupported_type",
				},
				expectedErr: "unsupported storage type: unsupported_type",
			},
			{
				name: "EmptyPathForSQLite",
				cfg: &config.Settings{
					StorageType: "sqlite",
					StoragePath: "",
				},
				expectedErr: "storage path is required for SQLite backend",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				backend, err := NewBackend(tc.cfg)
				assert.Error(t, err)
				assert.Nil(t, backend)
				assert.Equal(t, tc.expectedErr, err.Error())
			})
		}
	})
}

func TestNewBackend_EdgeCases_ConfigStructureValidation(t *testing.T) {
	t.Run("ConfigWithUnexpectedFields", func(t *testing.T) {
		// The config should handle extra fields gracefully
		// Since Go structs ignore unknown fields during unmarshaling
		cfg := &config.Settings{
			StorageType: "memory",
			// These would come from a config file with extra fields
		}
		
		backend, err := NewBackend(cfg)
		assert.NoError(t, err)
		assert.NotNil(t, backend)
		assert.IsType(t, &MemoryBackend{}, backend)
	})

	t.Run("DefaultValues", func(t *testing.T) {
		// Test behavior with zero-value config struct
		cfg := &config.Settings{
			// All fields are zero values
			StorageType: "",     // Should default to memory
			StoragePath: "",     // Should not matter for memory
			HTTPPort:    0,      // Should not matter for storage
			LogLevel:    "",     // Should not matter for storage
			Sqlite:      config.SqliteSettings{WALMode: false}, // Should not matter for memory
		}
		
		backend, err := NewBackend(cfg)
		assert.NoError(t, err)
		assert.NotNil(t, backend)
		assert.IsType(t, &MemoryBackend{}, backend)
	})
}