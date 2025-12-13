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
		fmt.Println("witnz v0.2.0")
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
			fmt.Printf("Protecting table: %s\n", tableConfig.Name)

			verifyConfig := &verify.TableConfig{
				Name: tableConfig.Name,
			}

			if err := baseHandler.AddTable(verifyConfig); err != nil {
				return fmt.Errorf("failed to add table %s: %w", tableConfig.Name, err)
			}
		}

		if len(cfg.Node.PeerAddrs) > 0 || cfg.Node.Bootstrap {
			fmt.Println("Starting Raft consensus...")
			raftConfig := &consensus.NodeConfig{
				NodeID:    cfg.Node.ID,
				BindAddr:  cfg.Node.BindAddr,
				DataDir:   cfg.Node.DataDir,
				Bootstrap: cfg.Node.Bootstrap,
				PeerAddrs: cfg.Node.PeerAddrs,
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

		dbConnStr := fmt.Sprintf("host=%s port=%d dbname=%s user=%s password=%s sslmode=disable",
			cfg.Database.Host, cfg.Database.Port, cfg.Database.Database,
			cfg.Database.User, cfg.Database.Password)
		merkleVerifier := verify.NewMerkleVerifier(store, dbConnStr)

		for _, tableConfig := range cfg.ProtectedTables {
			merkleVerifier.AddTable(&verify.TableConfig{
				Name:           tableConfig.Name,
				VerifyInterval: tableConfig.VerifyInterval,
			})
		}

		if err := merkleVerifier.Start(ctx); err != nil {
			return fmt.Errorf("failed to start Merkle verifier: %w", err)
		}
		defer merkleVerifier.Stop()

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
	Long: `Display node status including protected tables and hash chain state.
Note: Raft cluster state cannot be shown via this command as it would conflict with running nodes.
Use 'docker-compose logs' or check application logs to see Raft leader election status.`,
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
		fmt.Printf("Bind Address: %s\n", cfg.Node.BindAddr)

		if len(cfg.Node.PeerAddrs) > 0 || cfg.Node.Bootstrap {
			fmt.Printf("Cluster Mode: Raft (bootstrap: %v)\n", cfg.Node.Bootstrap)
			fmt.Printf("Peers: %d configured\n", len(cfg.Node.PeerAddrs))
			fmt.Printf("\nNote: To check Raft leader status, use 'docker-compose logs' and look for:\n")
			fmt.Printf("  - 'entering leader state' (this node is leader)\n")
			fmt.Printf("  - 'entering follower state' (this node is follower)\n")
		} else {
			fmt.Printf("Cluster Mode: single-node (no Raft)\n")
		}

		fmt.Printf("\nProtected Tables:\n")

		for _, tableConfig := range cfg.ProtectedTables {
			fmt.Printf("  - %s\n", tableConfig.Name)

			latest, err := store.GetLatestHashEntry(tableConfig.Name)
			if err == nil {
				fmt.Printf("    Latest sequence: %d\n", latest.SequenceNum)
				fmt.Printf("    Latest hash: %s...\n", latest.Hash[:16])
				fmt.Printf("    Timestamp: %s\n", latest.Timestamp.Format(time.RFC3339))
			} else {
				fmt.Printf("    No entries yet\n")
			}

			checkpoint, err := store.GetLatestMerkleCheckpoint(tableConfig.Name)
			if err == nil {
				fmt.Printf("    Latest checkpoint: seq=%d, records=%d\n",
					checkpoint.SequenceNum, checkpoint.RecordCount)
			}
		}

		totalEntries := 0
		for _, tableConfig := range cfg.ProtectedTables {
			entries, err := store.GetAllHashEntries(tableConfig.Name)
			if err == nil {
				totalEntries += len(entries)
			}
		}

		fmt.Printf("\nStorage Statistics:\n")
		fmt.Printf("  Total hash entries: %d\n", totalEntries)

		return nil
	},
}

var verifyCmd = &cobra.Command{
	Use:   "verify [table]",
	Short: "Verify Merkle Root integrity",
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

		ctx := context.Background()

		dbConnStr := fmt.Sprintf("host=%s port=%d dbname=%s user=%s password=%s sslmode=disable",
			cfg.Database.Host, cfg.Database.Port, cfg.Database.Database,
			cfg.Database.User, cfg.Database.Password)

		merkleVerifier := verify.NewMerkleVerifier(store, dbConnStr)

		tablesToVerify := []string{}
		if len(args) > 0 {
			tablesToVerify = append(tablesToVerify, args[0])
		} else {
			for _, tc := range cfg.ProtectedTables {
				tablesToVerify = append(tablesToVerify, tc.Name)
			}
		}

		for _, table := range tablesToVerify {
			fmt.Printf("Verifying table: %s\n", table)
			if err := merkleVerifier.VerifyTable(ctx, table); err != nil {
				fmt.Printf("  ❌ FAILED: %v\n", err)
			} else {
				fmt.Printf("  ✅ OK: Merkle Root is intact\n")
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
