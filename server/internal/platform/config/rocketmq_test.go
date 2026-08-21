package config

import (
	"strings"
	"testing"
)

func TestValidateRequiresCompleteRocketMQConfiguration(t *testing.T) {
	tests := []struct {
		name    string
		want    string
		prepare func(*Messaging)
	}{
		{name: "endpoint", want: "rocketmq_endpoint", prepare: func(m *Messaging) { m.RocketMQEndpoint = "" }},
		{name: "topics", want: "rocketmq_topics", prepare: func(m *Messaging) { m.RocketMQTopics = nil }},
		{name: "access key", want: "VELORA_ROCKETMQ_ACCESS_KEY", prepare: func(m *Messaging) { m.RocketMQAccessKey = "" }},
		{name: "secret key", want: "VELORA_ROCKETMQ_SECRET_KEY", prepare: func(m *Messaging) { m.RocketMQSecretKey = "" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Default()
			cfg.Database.DSN = "postgres://user:secret@db/app?sslmode=disable"
			cfg.Messaging = Messaging{
				Provider:          "rocketmq",
				RocketMQEndpoint:  "rocketmq-proxy:8081",
				RocketMQTopics:    []string{"business-events"},
				RocketMQAccessKey: "access",
				RocketMQSecretKey: "secret",
			}
			tt.prepare(&cfg.Messaging)
			if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Validate() error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestValidateAllowsCompleteRocketMQConfiguration(t *testing.T) {
	cfg := Default()
	cfg.Database.DSN = "postgres://user:secret@db/app?sslmode=disable"
	cfg.Messaging = Messaging{
		Provider:          "rocketmq",
		RocketMQEndpoint:  "rocketmq-proxy:8081",
		RocketMQTopics:    []string{"business-events"},
		RocketMQAccessKey: "access",
		RocketMQSecretKey: "secret",
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() rejected RocketMQ configuration: %v", err)
	}
}
