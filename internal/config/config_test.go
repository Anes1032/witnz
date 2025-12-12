package config

import (
	"os"
	"testing"
)

func TestLoad(t *testing.T) {
	configContent := `
database:
  host: localhost
  port: 5432
  database: testdb
  user: testuser
  password: testpass

node:
  id: node1
  bind_addr: 0.0.0.0:7000
  grpc_addr: 0.0.0.0:8000
  data_dir: /tmp/data
  peers:
    - node2:7000

protected_tables:
  - name: audit_log

alerts:
  enabled: false
`

	tmpfile, err := os.CreateTemp("", "witnz-test-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write([]byte(configContent)); err != nil {
		t.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(tmpfile.Name())
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Database.Host != "localhost" {
		t.Errorf("expected host=localhost, got %s", cfg.Database.Host)
	}
	if cfg.Node.ID != "node1" {
		t.Errorf("expected node.id=node1, got %s", cfg.Node.ID)
	}
	if len(cfg.ProtectedTables) != 1 {
		t.Errorf("expected 1 protected table, got %d", len(cfg.ProtectedTables))
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: Config{
				Database: DatabaseConfig{
					Host:     "localhost",
					Database: "testdb",
					User:     "testuser",
				},
				Node: NodeConfig{
					ID:       "node1",
					BindAddr: "0.0.0.0:7000",
					DataDir:  "/data",
				},
			},
			wantErr: false,
		},
		{
			name: "missing database host",
			config: Config{
				Database: DatabaseConfig{
					Database: "testdb",
					User:     "testuser",
				},
				Node: NodeConfig{
					ID:       "node1",
					BindAddr: "0.0.0.0:7000",
					DataDir:  "/data",
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestConnectionString(t *testing.T) {
	db := DatabaseConfig{
		Host:     "localhost",
		Port:     5432,
		Database: "testdb",
		User:     "testuser",
		Password: "testpass",
	}

	connStr := db.ConnectionString()
	expected := "host=localhost port=5432 dbname=testdb user=testuser password=testpass sslmode=disable"

	if connStr != expected {
		t.Errorf("ConnectionString() = %v, want %v", connStr, expected)
	}
}
