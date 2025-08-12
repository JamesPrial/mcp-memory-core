package logging

import (
	"testing"
	"time"
)

func TestLogLevel_String(t *testing.T) {
	tests := []struct {
		level    LogLevel
		expected string
	}{
		{LogLevelDebug, "debug"},
		{LogLevelInfo, "info"},
		{LogLevelWarn, "warn"},
		{LogLevelError, "error"},
		{LogLevel("unknown"), "unknown"},
	}

	for _, tt := range tests {
		t.Run(string(tt.level), func(t *testing.T) {
			if string(tt.level) != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, string(tt.level))
			}
		})
	}
}

func TestLogFormat_String(t *testing.T) {
	tests := []struct {
		format   LogFormat
		expected string
	}{
		{LogFormatJSON, "json"},
		{LogFormatText, "text"},
		{LogFormat("unknown"), "unknown"},
	}

	for _, tt := range tests {
		t.Run(string(tt.format), func(t *testing.T) {
			if string(tt.format) != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, string(tt.format))
			}
		})
	}
}

func TestLogOutput_String(t *testing.T) {
	tests := []struct {
		output   LogOutput
		expected string
	}{
		{LogOutputStdout, "stdout"},
		{LogOutputStderr, "stderr"},
		{LogOutputFile, "file"},
		{LogOutput("unknown"), "unknown"},
	}

	for _, tt := range tests {
		t.Run(string(tt.output), func(t *testing.T) {
			if string(tt.output) != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, string(tt.output))
			}
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	// Test default values
	if config.Level != LogLevelInfo {
		t.Errorf("Expected default level to be info, got %s", config.Level)
	}

	if config.Format != LogFormatJSON {
		t.Errorf("Expected default format to be json, got %s", config.Format)
	}

	if config.Output != LogOutputStdout {
		t.Errorf("Expected default output to be stdout, got %s", config.Output)
	}

	if !config.EnableRequestID {
		t.Error("Expected EnableRequestID to be true by default")
	}

	if config.EnableTracing {
		t.Error("Expected EnableTracing to be false by default")
	}

	if config.AsyncLogging {
		t.Error("Expected AsyncLogging to be false by default")
	}

	if config.BufferSize != 1024 {
		t.Errorf("Expected default buffer size to be 1024, got %d", config.BufferSize)
	}

	// Test sampling defaults
	if config.Sampling.Enabled {
		t.Error("Expected sampling to be disabled by default")
	}

	if config.Sampling.Rate != 1.0 {
		t.Errorf("Expected default sampling rate to be 1.0, got %f", config.Sampling.Rate)
	}

	if !config.Sampling.AlwaysErrors {
		t.Error("Expected sampling to always log errors by default")
	}

	// Test masking defaults
	if !config.Masking.Enabled {
		t.Error("Expected masking to be enabled by default")
	}

	if !config.Masking.MaskEmails {
		t.Error("Expected email masking to be enabled by default")
	}

	// Test metrics defaults
	metricsConfig := DefaultMetricsConfig()
	if !metricsConfig.Enabled {
		t.Error("Expected metrics to be enabled by default")
	}

	if metricsConfig.ListenAddr != ":9090" {
		t.Errorf("Expected default metrics listen addr to be :9090, got %s", metricsConfig.ListenAddr)
	}
}

func TestDevelopmentConfig(t *testing.T) {
	config := DevelopmentConfig()

	// Test development-specific settings
	if config.Level != LogLevelDebug {
		t.Errorf("Expected development level to be debug, got %s", config.Level)
	}

	if config.Format != LogFormatText {
		t.Errorf("Expected development format to be text, got %s", config.Format)
	}

	if !config.EnableStackTrace {
		t.Error("Expected stack traces to be enabled in development")
	}

	if !config.EnableCaller {
		t.Error("Expected caller info to be enabled in development")
	}

	if !config.PrettyPrint {
		t.Error("Expected pretty printing to be enabled in development")
	}

	if config.AsyncLogging {
		t.Error("Expected async logging to be disabled in development")
	}

	if config.Masking.Enabled {
		t.Error("Expected masking to be disabled in development")
	}

	if config.Sampling.Enabled {
		t.Error("Expected sampling to be disabled in development")
	}
}

func TestProductionConfig(t *testing.T) {
	config := ProductionConfig()

	// Test production-specific settings
	if config.Level != LogLevelInfo {
		t.Errorf("Expected production level to be info, got %s", config.Level)
	}

	if config.Format != LogFormatJSON {
		t.Errorf("Expected production format to be json, got %s", config.Format)
	}

	if !config.AsyncLogging {
		t.Error("Expected async logging to be enabled in production")
	}

	if config.BufferSize != 4096 {
		t.Errorf("Expected production buffer size to be 4096, got %d", config.BufferSize)
	}

	if !config.Sampling.Enabled {
		t.Error("Expected sampling to be enabled in production")
	}

	if config.Sampling.Rate != 0.1 {
		t.Errorf("Expected production sampling rate to be 0.1, got %f", config.Sampling.Rate)
	}

	if !config.EnableAudit {
		t.Error("Expected audit logging to be enabled in production")
	}

	if config.EnableStackTrace {
		t.Error("Expected stack traces to be disabled in production")
	}

	if config.PrettyPrint {
		t.Error("Expected pretty printing to be disabled in production")
	}

	if !config.Masking.Enabled {
		t.Error("Expected masking to be enabled in production")
	}
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name      string
		config    *Config
		expectErr bool
		errMsg    string
	}{
		{
			name:      "valid default config",
			config:    DefaultConfig(),
			expectErr: false,
		},
		{
			name: "invalid log level",
			config: &Config{
				Level:  LogLevel("invalid"),
				Format: LogFormatJSON,
				Output: LogOutputStdout,
			},
			expectErr: true,
			errMsg:    "invalid log level",
		},
		{
			name: "invalid log format",
			config: &Config{
				Level:  LogLevelInfo,
				Format: LogFormat("invalid"),
				Output: LogOutputStdout,
			},
			expectErr: true,
			errMsg:    "invalid log format",
		},
		{
			name: "invalid log output",
			config: &Config{
				Level:  LogLevelInfo,
				Format: LogFormatJSON,
				Output: LogOutput("invalid"),
			},
			expectErr: true,
			errMsg:    "invalid log output",
		},
		{
			name: "file output without file path",
			config: &Config{
				Level:  LogLevelInfo,
				Format: LogFormatJSON,
				Output: LogOutputFile,
			},
			expectErr: true,
			errMsg:    "filePath required",
		},
		{
			name: "file output with valid file path",
			config: &Config{
				Level:    LogLevelInfo,
				Format:   LogFormatJSON,
				Output:   LogOutputFile,
				FilePath: "/tmp/test.log",
			},
			expectErr: false,
		},
		{
			name: "invalid component level",
			config: &Config{
				Level:  LogLevelInfo,
				Format: LogFormatJSON,
				Output: LogOutputStdout,
				ComponentLevels: map[string]LogLevel{
					"test": LogLevel("invalid"),
				},
			},
			expectErr: true,
			errMsg:    "invalid log level for component",
		},
		{
			name: "invalid sampling rate - too low",
			config: &Config{
				Level:  LogLevelInfo,
				Format: LogFormatJSON,
				Output: LogOutputStdout,
				Sampling: SamplingConfig{
					Enabled: true,
					Rate:    -0.1,
				},
			},
			expectErr: true,
			errMsg:    "sampling rate must be between 0.0 and 1.0",
		},
		{
			name: "invalid sampling rate - too high",
			config: &Config{
				Level:  LogLevelInfo,
				Format: LogFormatJSON,
				Output: LogOutputStdout,
				Sampling: SamplingConfig{
					Enabled: true,
					Rate:    1.1,
				},
			},
			expectErr: true,
			errMsg:    "sampling rate must be between 0.0 and 1.0",
		},
		{
			name: "invalid sampling burst size",
			config: &Config{
				Level:  LogLevelInfo,
				Format: LogFormatJSON,
				Output: LogOutputStdout,
				Sampling: SamplingConfig{
					Enabled:   true,
					Rate:      0.5,
					BurstSize: -1,
				},
			},
			expectErr: true,
			errMsg:    "sampling burst size must be non-negative",
		},
		{
			name: "OTLP enabled without endpoint",
			config: &Config{
				Level:  LogLevelInfo,
				Format: LogFormatJSON,
				Output: LogOutputStdout,
				OTLP: OTLPConfig{
					Enabled: true,
				},
			},
			expectErr: true,
			errMsg:    "OTLP endpoint required",
		},
		{
			name: "OTLP enabled with endpoint",
			config: &Config{
				Level:  LogLevelInfo,
				Format: LogFormatJSON,
				Output: LogOutputStdout,
				OTLP: OTLPConfig{
					Enabled:             true,
					Endpoint:            "http://localhost:4317",
					Timeout:             30, // seconds
					BatchSize:           100,
					BatchTimeout:        5,
					QueueSize:           1000,
					HealthCheckInterval: 30,
				},
			},
			expectErr: false,
		},
		{
			name: "negative buffer size",
			config: &Config{
				Level:      LogLevelInfo,
				Format:     LogFormatJSON,
				Output:     LogOutputStdout,
				BufferSize: -1,
			},
			expectErr: true,
			errMsg:    "buffer size must be non-negative",
		},
		{
			name: "audit enabled without file path - should set default",
			config: &Config{
				Level:       LogLevelInfo,
				Format:      LogFormatJSON,
				Output:      LogOutputStdout,
				EnableAudit: true,
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()

			if tt.expectErr {
				if err == nil {
					t.Error("Expected validation error but got none")
				} else if tt.errMsg != "" && !contains(err.Error(), tt.errMsg) {
					t.Errorf("Expected error message to contain %q, got %q", tt.errMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no validation error but got: %v", err)
				}
			}

			// Special case: check audit file path default
			if tt.name == "audit enabled without file path - should set default" && err == nil {
				if tt.config.AuditFilePath != "audit.log" {
					t.Errorf("Expected default audit file path to be set to 'audit.log', got %s", tt.config.AuditFilePath)
				}
			}
		})
	}
}

func TestConfig_GetLevelForComponent(t *testing.T) {
	config := &Config{
		Level: LogLevelInfo,
		ComponentLevels: map[string]LogLevel{
			"database": LogLevelError,
			"auth":     LogLevelDebug,
		},
	}

	tests := []struct {
		component string
		expected  LogLevel
	}{
		{"database", LogLevelError},
		{"auth", LogLevelDebug},
		{"unknown", LogLevelInfo}, // Should return default level
		{"", LogLevelInfo},        // Empty component should return default
	}

	for _, tt := range tests {
		t.Run(tt.component, func(t *testing.T) {
			level := config.GetLevelForComponent(tt.component)
			if level != tt.expected {
				t.Errorf("Expected level %s for component %s, got %s", tt.expected, tt.component, level)
			}
		})
	}
}

func TestSamplingConfig(t *testing.T) {
	config := SamplingConfig{
		Enabled:      true,
		Rate:         0.5,
		BurstSize:    100,
		AlwaysErrors: true,
	}

	if !config.Enabled {
		t.Error("Expected sampling to be enabled")
	}

	if config.Rate != 0.5 {
		t.Errorf("Expected rate to be 0.5, got %f", config.Rate)
	}

	if config.BurstSize != 100 {
		t.Errorf("Expected burst size to be 100, got %d", config.BurstSize)
	}

	if !config.AlwaysErrors {
		t.Error("Expected always errors to be true")
	}
}

func TestMaskingConfig(t *testing.T) {
	config := MaskingConfig{
		Enabled:          true,
		Fields:           []string{"password", "secret"},
		Patterns:         []string{`\d{4}-\d{4}-\d{4}-\d{4}`},
		MaskEmails:       true,
		MaskPhoneNumbers: false,
		MaskCreditCards:  true,
		MaskSSN:          true,
		MaskAPIKeys:      true,
	}

	if !config.Enabled {
		t.Error("Expected masking to be enabled")
	}

	if len(config.Fields) != 2 {
		t.Errorf("Expected 2 fields to mask, got %d", len(config.Fields))
	}

	if config.Fields[0] != "password" || config.Fields[1] != "secret" {
		t.Errorf("Expected fields to be [password, secret], got %v", config.Fields)
	}

	if len(config.Patterns) != 1 {
		t.Errorf("Expected 1 pattern, got %d", len(config.Patterns))
	}

	if !config.MaskEmails {
		t.Error("Expected email masking to be enabled")
	}

	if config.MaskPhoneNumbers {
		t.Error("Expected phone number masking to be disabled")
	}
}

func TestOTLPConfig(t *testing.T) {
	config := OTLPConfig{
		Enabled:   true,
		Endpoint:  "http://localhost:4317",
		Headers:   map[string]string{"authorization": "bearer token"},
		Insecure:  false,
		Timeout:   30,
		BatchSize: 100,
		QueueSize: 1000,
	}

	if !config.Enabled {
		t.Error("Expected OTLP to be enabled")
	}

	if config.Endpoint != "http://localhost:4317" {
		t.Errorf("Expected endpoint to be http://localhost:4317, got %s", config.Endpoint)
	}

	if len(config.Headers) != 1 {
		t.Errorf("Expected 1 header, got %d", len(config.Headers))
	}

	if config.Headers["authorization"] != "bearer token" {
		t.Errorf("Expected authorization header, got %v", config.Headers)
	}

	if config.Insecure {
		t.Error("Expected insecure to be false")
	}

	if config.Timeout != 30 {
		t.Errorf("Expected timeout to be 30, got %d", config.Timeout)
	}
}

func TestDefaultMetricsConfig(t *testing.T) {
	config := DefaultMetricsConfig()

	if !config.Enabled {
		t.Error("Expected metrics to be enabled by default")
	}

	if config.ListenAddr != ":9090" {
		t.Errorf("Expected default listen addr to be :9090, got %s", config.ListenAddr)
	}

	if config.Path != "/metrics" {
		t.Errorf("Expected default path to be /metrics, got %s", config.Path)
	}

	if config.Namespace != "mcp_memory" {
		t.Errorf("Expected default namespace to be mcp_memory, got %s", config.Namespace)
	}

	if config.Subsystem != "core" {
		t.Errorf("Expected default subsystem to be core, got %s", config.Subsystem)
	}

	if !config.EnableRuntime {
		t.Error("Expected runtime metrics to be enabled by default")
	}

	if config.CollectInterval != 15*time.Second {
		t.Errorf("Expected default collect interval to be 15s, got %s", config.CollectInterval)
	}
}

func TestMetricsConfig(t *testing.T) {
	config := MetricsConfig{
		Enabled:         true,
		ListenAddr:      ":8080",
		Path:            "/custom-metrics",
		Namespace:       "custom",
		Subsystem:       "app",
		Labels:          map[string]string{"version": "1.0.0"},
		EnableRuntime:   false,
		CollectInterval: 30 * time.Second,
	}

	if !config.Enabled {
		t.Error("Expected metrics to be enabled")
	}

	if config.ListenAddr != ":8080" {
		t.Errorf("Expected listen addr to be :8080, got %s", config.ListenAddr)
	}

	if config.Path != "/custom-metrics" {
		t.Errorf("Expected path to be /custom-metrics, got %s", config.Path)
	}

	if config.Namespace != "custom" {
		t.Errorf("Expected namespace to be custom, got %s", config.Namespace)
	}

	if config.EnableRuntime {
		t.Error("Expected runtime metrics to be disabled")
	}

	if config.CollectInterval != 30*time.Second {
		t.Errorf("Expected collect interval to be 30s, got %s", config.CollectInterval)
	}

	if config.Labels["version"] != "1.0.0" {
		t.Errorf("Expected version label to be 1.0.0, got %s", config.Labels["version"])
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || 
		(len(s) > len(substr) && (s[:len(substr)] == substr || 
		s[len(s)-len(substr):] == substr || 
		func() bool {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
			return false
		}())))
}