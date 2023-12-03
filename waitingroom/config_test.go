package waitingroom

import (
	"testing"

	"github.com/go-playground/validator/v10"
)

func validateConfig(config Config) error {
	validate := validator.New()
	return validate.Struct(config)
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: Config{
				LogLevel:            "debug",
				Listener:            "localhost:8080",
				PermittedAccessSec:  300,
				EntryDelaySec:       60,
				QueueEnableSec:      1200,
				PermitIntervalSec:   60,
				PermitUnitNumber:    5,
				CacheTTLSec:         30,
				NegativeCacheTTLSec: 10,
				PublicHost:          "https://example.com",
				SlackApiToken:       "fake-token",
				SlackChannel:        "general",
			},
			wantErr: false,
		},
		{
			name: "invalid config - PermitIntervalSec less than CacheTTLSec",
			config: Config{
				LogLevel:            "debug",
				Listener:            "localhost:8080",
				PermittedAccessSec:  5,
				EntryDelaySec:       60,
				QueueEnableSec:      1200,
				PermitIntervalSec:   5,
				PermitUnitNumber:    5,
				CacheTTLSec:         10,
				NegativeCacheTTLSec: 1,
				PublicHost:          "https://example.com",
				SlackApiToken:       "fake-token",
				SlackChannel:        "general",
			},
			wantErr: true,
		},
		{
			name: "invalid config - PermitIntervalSec less than NegativeCacheTTLSec",
			config: Config{
				LogLevel:            "debug",
				Listener:            "localhost:8080",
				PermittedAccessSec:  5,
				EntryDelaySec:       60,
				QueueEnableSec:      1200,
				PermitIntervalSec:   5,
				PermitUnitNumber:    5,
				CacheTTLSec:         1,
				NegativeCacheTTLSec: 10,
				PublicHost:          "https://example.com",
				SlackApiToken:       "fake-token",
				SlackChannel:        "general",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateConfig(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
