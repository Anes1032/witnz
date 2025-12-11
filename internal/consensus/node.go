package consensus

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb"
	"github.com/witnz/witnz/internal/storage"
)

type NodeConfig struct {
	NodeID    string
	BindAddr  string
	DataDir   string
	Bootstrap bool
	Peers     []string
}

type Node struct {
	config  *NodeConfig
	raft    *raft.Raft
	fsm     *FSM
	storage *storage.Storage
}

func NewNode(cfg *NodeConfig, store *storage.Storage) (*Node, error) {
	return &Node{
		config:  cfg,
		storage: store,
	}, nil
}

func (n *Node) Start(ctx context.Context) error {
	raftConfig := raft.DefaultConfig()
	raftConfig.LocalID = raft.ServerID(n.config.NodeID)

	raftDir := filepath.Join(n.config.DataDir, "raft")
	if err := os.MkdirAll(raftDir, 0755); err != nil {
		return fmt.Errorf("failed to create raft directory: %w", err)
	}

	logStore, err := raftboltdb.NewBoltStore(filepath.Join(raftDir, "raft-log.db"))
	if err != nil {
		return fmt.Errorf("failed to create log store: %w", err)
	}

	stableStore, err := raftboltdb.NewBoltStore(filepath.Join(raftDir, "raft-stable.db"))
	if err != nil {
		return fmt.Errorf("failed to create stable store: %w", err)
	}

	snapshotStore, err := raft.NewFileSnapshotStore(raftDir, 2, os.Stderr)
	if err != nil {
		return fmt.Errorf("failed to create snapshot store: %w", err)
	}

	addr, err := net.ResolveTCPAddr("tcp", n.config.BindAddr)
	if err != nil {
		return fmt.Errorf("failed to resolve address: %w", err)
	}

	transport, err := raft.NewTCPTransport(n.config.BindAddr, addr, 3, 10*time.Second, os.Stderr)
	if err != nil {
		return fmt.Errorf("failed to create transport: %w", err)
	}

	n.fsm = NewFSM(n.storage)

	ra, err := raft.NewRaft(raftConfig, n.fsm, logStore, stableStore, snapshotStore, transport)
	if err != nil {
		return fmt.Errorf("failed to create raft: %w", err)
	}

	n.raft = ra

	if n.config.Bootstrap {
		configuration := raft.Configuration{
			Servers: []raft.Server{
				{
					ID:      raftConfig.LocalID,
					Address: transport.LocalAddr(),
				},
			},
		}

		future := ra.BootstrapCluster(configuration)
		if err := future.Error(); err != nil {
			return fmt.Errorf("failed to bootstrap cluster: %w", err)
		}
	}

	return nil
}

func (n *Node) Stop() error {
	if n.raft != nil {
		future := n.raft.Shutdown()
		if err := future.Error(); err != nil {
			return fmt.Errorf("failed to shutdown raft: %w", err)
		}
	}
	return nil
}

func (n *Node) ApplyLog(entry *LogEntry) error {
	if n.raft.State() != raft.Leader {
		return fmt.Errorf("not the leader")
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal log entry: %w", err)
	}

	future := n.raft.Apply(data, 10*time.Second)
	if err := future.Error(); err != nil {
		return fmt.Errorf("failed to apply log: %w", err)
	}

	return nil
}

func (n *Node) IsLeader() bool {
	return n.raft != nil && n.raft.State() == raft.Leader
}

func (n *Node) Leader() string {
	if n.raft == nil {
		return ""
	}
	addr, _ := n.raft.LeaderWithID()
	return string(addr)
}

func (n *Node) AddPeer(id, addr string) error {
	if n.raft == nil {
		return fmt.Errorf("raft not initialized")
	}

	future := n.raft.AddVoter(raft.ServerID(id), raft.ServerAddress(addr), 0, 0)
	return future.Error()
}

func (n *Node) RemovePeer(id string) error {
	if n.raft == nil {
		return fmt.Errorf("raft not initialized")
	}

	future := n.raft.RemoveServer(raft.ServerID(id), 0, 0)
	return future.Error()
}

func (n *Node) Stats() map[string]string {
	if n.raft == nil {
		return map[string]string{"state": "not initialized"}
	}
	return n.raft.Stats()
}
