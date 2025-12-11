package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/witnz/witnz/internal/cdc"
	"github.com/witnz/witnz/internal/config"
	"github.com/witnz/witnz/internal/consensus"
	"github.com/witnz/witnz/internal/storage"
	"github.com/witnz/witnz/internal/verify"
)

var (
	cfgFile string
	cfg     *config.Config
)

var rootCmd = &cobra.Command{
	Use:   "witnz",
	Short: "Witnz - PostgreSQL Tamper Detection System",
	Long:  `A distributed database tampering detection system for PostgreSQL`,
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "witnz.yaml", "config file path")
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(verifyCmd)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("witnz v0.1.0-alpha")
		fmt.Println("PostgreSQL Tamper Detection System")
	},
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize witnz node",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(cfgFile)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		dbPath := filepath.Join(cfg.Node.DataDir, "witnz.db")
		if err := os.MkdirAll(cfg.Node.DataDir, 0755); err != nil {
			return fmt.Errorf("failed to create data directory: %w", err)
		}

		store, err := storage.New(dbPath)
		if err != nil {
			return fmt.Errorf("failed to initialize storage: %w", err)
		}
		defer store.Close()

		fmt.Printf("Initialized witnz node: %s\n", cfg.Node.ID)
		fmt.Printf("Data directory: %s\n", cfg.Node.DataDir)
		fmt.Printf("Database path: %s\n", dbPath)

		return nil
	},
}

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start witnz node",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(cfgFile)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		fmt.Printf("Starting witnz node: %s\n", cfg.Node.ID)
		fmt.Printf("Connecting to PostgreSQL: %s:%d/%s\n",
			cfg.Database.Host, cfg.Database.Port, cfg.Database.Database)

		dbPath := filepath.Join(cfg.Node.DataDir, "witnz.db")
		store, err := storage.New(dbPath)
		if err != nil {
			return fmt.Errorf("failed to initialize storage: %w", err)
		}
		defer store.Close()

		var raftNode *consensus.Node
		var handler cdc.EventHandler

		baseHandler := verify.NewHashChainHandler(store)

		for _, tableConfig := range cfg.ProtectedTables {
			fmt.Printf("Protecting table: %s (mode: %s)\n", tableConfig.Name, tableConfig.Mode)

			verifyConfig := &verify.TableConfig{
				Name: tableConfig.Name,
				Mode: verify.ProtectionMode(tableConfig.Mode),
			}

			if err := baseHandler.AddTable(verifyConfig); err != nil {
				return fmt.Errorf("failed to add table %s: %w", tableConfig.Name, err)
			}
		}

		if len(cfg.Node.Peers) > 0 {
			fmt.Println("Starting Raft consensus...")
			raftConfig := &consensus.NodeConfig{
				NodeID:    cfg.Node.ID,
				BindAddr:  cfg.Node.BindAddr,
				DataDir:   cfg.Node.DataDir,
				Bootstrap: len(cfg.Node.Peers) == 0,
				Peers:     cfg.Node.Peers,
			}

			raftNode, err = consensus.NewNode(raftConfig, store)
			if err != nil {
				return fmt.Errorf("failed to create raft node: %w", err)
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			if err := raftNode.Start(ctx); err != nil {
				return fmt.Errorf("failed to start raft node: %w", err)
			}
			defer raftNode.Stop()

			handler = verify.NewRaftHashChainHandler(baseHandler, raftNode)
			fmt.Printf("Raft node started, leader: %s\n", raftNode.Leader())
		} else {
			fmt.Println("Running in single-node mode (no Raft)")
			handler = baseHandler
		}

		cdcConfig := &cdc.ReplicationConfig{
			Host:            cfg.Database.Host,
			Port:            cfg.Database.Port,
			Database:        cfg.Database.Database,
			User:            cfg.Database.User,
			Password:        cfg.Database.Password,
			SlotName:        fmt.Sprintf("witnz_%s", cfg.Node.ID),
			PublicationName: "witnz_publication",
		}

		manager := cdc.NewManager(cdcConfig)
		manager.AddHandler(handler)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		fmt.Println("Initializing CDC manager...")
		if err := manager.Initialize(ctx); err != nil {
			return fmt.Errorf("failed to initialize CDC manager: %w", err)
		}

		fmt.Println("Starting replication...")
		if err := manager.Start(ctx); err != nil {
			return fmt.Errorf("failed to start CDC manager: %w", err)
		}

		fmt.Println("Witnz node is running. Press Ctrl+C to stop.")

		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh

		fmt.Println("\nShutting down...")
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()

		if err := manager.Stop(shutdownCtx); err != nil {
			return fmt.Errorf("failed to stop CDC manager: %w", err)
		}

		fmt.Println("Witnz node stopped")
		return nil
	},
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Display node status",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(cfgFile)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		dbPath := filepath.Join(cfg.Node.DataDir, "witnz.db")
		store, err := storage.New(dbPath)
		if err != nil {
			return fmt.Errorf("failed to open storage: %w", err)
		}
		defer store.Close()

		fmt.Printf("Node ID: %s\n", cfg.Node.ID)
		fmt.Printf("Data Directory: %s\n", cfg.Node.DataDir)
		fmt.Printf("\nProtected Tables:\n")

		for _, tableConfig := range cfg.ProtectedTables {
			fmt.Printf("  - %s (mode: %s)\n", tableConfig.Name, tableConfig.Mode)

			if tableConfig.Mode == "append_only" {
				latest, err := store.GetLatestHashEntry(tableConfig.Name)
				if err == nil {
					fmt.Printf("    Latest sequence: %d\n", latest.SequenceNum)
					fmt.Printf("    Latest hash: %s\n", latest.Hash[:16])
				} else {
					fmt.Printf("    No entries yet\n")
				}
			}
		}

		return nil
	},
}

var verifyCmd = &cobra.Command{
	Use:   "verify [table]",
	Short: "Verify hash chain integrity",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(cfgFile)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		dbPath := filepath.Join(cfg.Node.DataDir, "witnz.db")
		store, err := storage.New(dbPath)
		if err != nil {
			return fmt.Errorf("failed to open storage: %w", err)
		}
		defer store.Close()

		handler := verify.NewHashChainHandler(store)

		for _, tableConfig := range cfg.ProtectedTables {
			verifyConfig := &verify.TableConfig{
				Name: tableConfig.Name,
				Mode: verify.ProtectionMode(tableConfig.Mode),
			}
			handler.AddTable(verifyConfig)
		}

		tablesToVerify := []string{}
		if len(args) > 0 {
			tablesToVerify = append(tablesToVerify, args[0])
		} else {
			for _, tc := range cfg.ProtectedTables {
				if tc.Mode == "append_only" {
					tablesToVerify = append(tablesToVerify, tc.Name)
				}
			}
		}

		for _, table := range tablesToVerify {
			fmt.Printf("Verifying table: %s\n", table)
			if err := handler.VerifyHashChain(table); err != nil {
				fmt.Printf("  ❌ FAILED: %v\n", err)
			} else {
				fmt.Printf("  ✅ OK: Hash chain is intact\n")
			}
		}

		return nil
	},
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
