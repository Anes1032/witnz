package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	Database       DatabaseConfig       `mapstructure:"database"`
	Node           NodeConfig           `mapstructure:"node"`
	ProtectedTables []ProtectedTableConfig `mapstructure:"protected_tables"`
	Alerts         AlertsConfig         `mapstructure:"alerts"`
}

type DatabaseConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Database string `mapstructure:"database"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
}

type NodeConfig struct {
	ID       string   `mapstructure:"id"`
	BindAddr string   `mapstructure:"bind_addr"`
	GRPCAddr string   `mapstructure:"grpc_addr"`
	DataDir  string   `mapstructure:"data_dir"`
	Peers    []string `mapstructure:"peers"`
}

type ProtectedTableConfig struct {
	Name           string `mapstructure:"name"`
	Mode           string `mapstructure:"mode"`
	VerifyInterval string `mapstructure:"verify_interval"`
}

type AlertsConfig struct {
	Enabled       bool   `mapstructure:"enabled"`
	SlackWebhook  string `mapstructure:"slack_webhook"`
	PagerDutyKey  string `mapstructure:"pagerduty_key"`
}

func Load(configPath string) (*Config, error) {
	v := viper.New()

	v.SetConfigFile(configPath)
	v.SetConfigType("yaml")

	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	for _, key := range v.AllKeys() {
		val := v.GetString(key)
		if expanded := os.ExpandEnv(val); expanded != val {
			v.Set(key, expanded)
		}
	}

	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &config, nil
}

func (c *Config) Validate() error {
	if c.Database.Host == "" {
		return fmt.Errorf("database.host is required")
	}
	if c.Database.Database == "" {
		return fmt.Errorf("database.database is required")
	}
	if c.Database.User == "" {
		return fmt.Errorf("database.user is required")
	}
	if c.Node.ID == "" {
		return fmt.Errorf("node.id is required")
	}
	if c.Node.BindAddr == "" {
		return fmt.Errorf("node.bind_addr is required")
	}
	if c.Node.DataDir == "" {
		return fmt.Errorf("node.data_dir is required")
	}

	for _, table := range c.ProtectedTables {
		if table.Mode != "append_only" && table.Mode != "state_integrity" {
			return fmt.Errorf("invalid mode for table %s: %s (must be append_only or state_integrity)", table.Name, table.Mode)
		}
	}

	return nil
}

func (d *DatabaseConfig) ConnectionString() string {
	return fmt.Sprintf("host=%s port=%d dbname=%s user=%s password=%s sslmode=disable",
		d.Host, d.Port, d.Database, d.User, d.Password)
}
