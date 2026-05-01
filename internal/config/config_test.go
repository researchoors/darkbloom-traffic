package config

import (
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := Default()

	if cfg.BaseURL != "https://inference.darkbloom.ai" {
		t.Errorf("Default BaseURL = %q, want %q", cfg.BaseURL, "https://inference.darkbloom.ai")
	}
	if cfg.TargetTPS != 500 {
		t.Errorf("Default TargetTPS = %f, want 500", cfg.TargetTPS)
	}
	if cfg.MaxRPS != 4.5 {
		t.Errorf("Default MaxRPS = %f, want 4.5", cfg.MaxRPS)
	}
	if cfg.ModelRefreshInterval != 5*time.Minute {
		t.Errorf("Default ModelRefreshInterval = %v, want %v", cfg.ModelRefreshInterval, 5*time.Minute)
	}
	if cfg.RequestTimeout != 120*time.Second {
		t.Errorf("Default RequestTimeout = %v, want %v", cfg.RequestTimeout, 120*time.Second)
	}
	if cfg.StatsInterval != 10*time.Second {
		t.Errorf("Default StatsInterval = %v, want %v", cfg.StatsInterval, 10*time.Second)
	}
}

func TestValidateRejectsEmpty(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name:    "empty BaseURL",
			cfg:     Config{BaseURL: "", APIKey: "key", TargetTPS: 1, MaxRPS: 1},
			wantErr: true,
		},
		{
			name:    "empty APIKey",
			cfg:     Config{BaseURL: "https://example.com", APIKey: "", TargetTPS: 1, MaxRPS: 1},
			wantErr: true,
		},
		{
			name:    "non-positive TargetTPS",
			cfg:     Config{BaseURL: "https://example.com", APIKey: "key", TargetTPS: 0, MaxRPS: 1},
			wantErr: true,
		},
		{
			name:    "negative TargetTPS",
			cfg:     Config{BaseURL: "https://example.com", APIKey: "key", TargetTPS: -1, MaxRPS: 1},
			wantErr: true,
		},
		{
			name:    "MaxRPS zero",
			cfg:     Config{BaseURL: "https://example.com", APIKey: "key", TargetTPS: 1, MaxRPS: 0},
			wantErr: true,
		},
		{
			name:    "MaxRPS exceeds 10",
			cfg:     Config{BaseURL: "https://example.com", APIKey: "key", TargetTPS: 1, MaxRPS: 10.1},
			wantErr: true,
		},
		{
			name:    "MaxRPS negative",
			cfg:     Config{BaseURL: "https://example.com", APIKey: "key", TargetTPS: 1, MaxRPS: -1},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateAcceptsValid(t *testing.T) {
	cfg := Config{
		BaseURL: "https://inference.darkbloom.ai",
		APIKey:  "test-key",
		TargetTPS: 500,
		MaxRPS:  4.5,
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate() unexpected error: %v", err)
	}

	// MaxRPS boundary: exactly 10 is valid
	cfg2 := Config{
		BaseURL: "https://inference.darkbloom.ai",
		APIKey:  "test-key",
		TargetTPS: 1,
		MaxRPS:  10,
	}
	if err := cfg2.Validate(); err != nil {
		t.Errorf("Validate() with MaxRPS=10 unexpected error: %v", err)
	}
}

func TestFromEnv(t *testing.T) {
	t.Setenv("DARKBLOOM_BASE_URL", "https://custom.example.com")
	t.Setenv("DARKBLOOM_API_KEY", "env-key")
	t.Setenv("DARKBLOOM_TARGET_TPS", "1000")
	t.Setenv("DARKBLOOM_MAX_RPS", "2.5")

	cfg := Default()
	cfg.FromEnv()

	if cfg.BaseURL != "https://custom.example.com" {
		t.Errorf("FromEnv BaseURL = %q, want %q", cfg.BaseURL, "https://custom.example.com")
	}
	if cfg.APIKey != "env-key" {
		t.Errorf("FromEnv APIKey = %q, want %q", cfg.APIKey, "env-key")
	}
	if cfg.TargetTPS != 1000 {
		t.Errorf("FromEnv TargetTPS = %f, want 1000", cfg.TargetTPS)
	}
	if cfg.MaxRPS != 2.5 {
		t.Errorf("FromEnv MaxRPS = %f, want 2.5", cfg.MaxRPS)
	}
}
