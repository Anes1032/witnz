package cdc

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/jackc/pglogrepl"
	"github.com/jackc/pgx/v5"
	"github.com/witnz/witnz/internal/alert"
)

type Manager struct {
	config     *ReplicationConfig
	client     *ReplicationClient
	handlers     []EventHandler
	mu           sync.RWMutex
	currentLSN   pglogrepl.LSN
	running      bool
	stopCh       chan struct{}
	wg           sync.WaitGroup
	alertManager *alert.Manager
}

func NewManager(config *ReplicationConfig) *Manager {
	return &Manager{
		config:   config,
		handlers: make([]EventHandler, 0),
		stopCh:   make(chan struct{}),
	}
}

func (m *Manager) AddHandler(handler EventHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.handlers = append(m.handlers, handler)
}

func (m *Manager) SetAlertManager(am *alert.Manager) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.alertManager = am
}

func (m *Manager) Initialize(ctx context.Context) error {
	if err := m.createPublicationIfNotExists(ctx); err != nil {
		return fmt.Errorf("failed to create publication: %w", err)
	}

	client := NewReplicationClient(m.config, m)
	if err := client.Connect(ctx); err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	if err := client.CreateSlotIfNotExists(ctx); err != nil {
		client.Close(ctx)
		return fmt.Errorf("failed to create slot: %w", err)
	}

	m.client = client
	return nil
}

func (m *Manager) Start(ctx context.Context) error {
	if m.running {
		return fmt.Errorf("manager already running")
	}

	if m.client == nil {
		return fmt.Errorf("manager not initialized")
	}

	if err := m.client.StartReplication(ctx, m.currentLSN); err != nil {
		return fmt.Errorf("failed to start replication: %w", err)
	}

	m.running = true
	m.wg.Add(1)

	go m.receiveLoop(ctx)

	return nil
}

func (m *Manager) Stop(ctx context.Context) error {
	if !m.running {
		return nil
	}

	close(m.stopCh)
	m.wg.Wait()
	m.running = false

	if m.client != nil {
		return m.client.Close(ctx)
	}

	return nil
}

func (m *Manager) receiveLoop(ctx context.Context) {
	defer m.wg.Done()

	errorCount := 0
	const maxBackoff = 30 * time.Second

	for {
		select {
		case <-m.stopCh:
			return
		case <-ctx.Done():
			return
		default:
			if err := m.client.ReceiveMessage(ctx); err != nil {
				fmt.Printf("Error receiving message: %v\n", err)
				errorCount++

				// Exponential backoff
				backoff := time.Duration(math.Pow(2, float64(errorCount))) * time.Second
				if backoff > maxBackoff {
					backoff = maxBackoff
				}

				// Send alert if configured
				m.mu.RLock()
				if m.alertManager != nil {
					_ = m.alertManager.SendSystemAlert(
						"Replication Connection Lost",
						fmt.Sprintf("Failed to receive replication message: %v. Retrying in %v...", err, backoff),
						"danger",
					)
				}
				m.mu.RUnlock()

				select {
				case <-time.After(backoff):
				case <-m.stopCh:
					return
				case <-ctx.Done():
					return
				}
			} else {
				// Reset error count on success
				errorCount = 0
			}
		}
	}
}

func (m *Manager) HandleChange(event *ChangeEvent) error {
	m.mu.RLock()
	handlers := make([]EventHandler, len(m.handlers))
	copy(handlers, m.handlers)
	m.mu.RUnlock()

	for _, handler := range handlers {
		if err := handler.HandleChange(event); err != nil {
			return fmt.Errorf("handler failed: %w", err)
		}
	}

	return nil
}

func (m *Manager) createPublicationIfNotExists(ctx context.Context) error {
	connString := fmt.Sprintf("host=%s port=%d dbname=%s user=%s password=%s",
		m.config.Host,
		m.config.Port,
		m.config.Database,
		m.config.User,
		m.config.Password,
	)

	conn, err := pgx.Connect(ctx, connString)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer conn.Close(ctx)

	var exists bool
	err = conn.QueryRow(ctx,
		"SELECT EXISTS (SELECT 1 FROM pg_publication WHERE pubname = $1)",
		m.config.PublicationName,
	).Scan(&exists)

	if err != nil {
		return fmt.Errorf("failed to check publication: %w", err)
	}

	if !exists {
		_, err = conn.Exec(ctx,
			fmt.Sprintf("CREATE PUBLICATION %s FOR ALL TABLES", m.config.PublicationName),
		)
		if err != nil {
			return fmt.Errorf("failed to create publication: %w", err)
		}
		fmt.Printf("Created publication: %s\n", m.config.PublicationName)
	}

	return nil
}

func (m *Manager) SetLSN(lsn pglogrepl.LSN) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.currentLSN = lsn
}

func (m *Manager) GetLSN() pglogrepl.LSN {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.currentLSN
}
