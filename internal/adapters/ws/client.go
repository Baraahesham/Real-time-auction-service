package ws

import (
	"context"
	"fmt"
	"sync"
	"time"
	"troffee-auction-service/internal/config"

	"github.com/alitto/pond"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
)

type WsClient struct {
	id         string
	userID     uuid.UUID
	conn       *websocket.Conn
	sendChan   chan *ServerMessage
	ctx        context.Context
	cancel     context.CancelFunc
	handler    *WsHandler
	workerPool *pond.WorkerPool
	stopped    bool
	mu         sync.Mutex
	logger     zerolog.Logger
}
type WsClientParams struct {
	UserID  uuid.UUID
	Conn    *websocket.Conn
	Handler *WsHandler
}

// NewClient creates a new WebSocket client
func NewClient(params WsClientParams) *WsClient {
	ctx, cancel := context.WithCancel(context.Background())

	pool := pond.New(
		config.WSMaxWorkers,
		config.WSMaxCapacity,
		pond.Context(ctx),
		pond.Strategy(pond.Balanced()),
	)
	client := &WsClient{
		id:         uuid.New().String(),
		userID:     params.UserID,
		conn:       params.Conn,
		sendChan:   make(chan *ServerMessage, 100), // Buffered channel to handle multiple events
		ctx:        ctx,
		cancel:     cancel,
		handler:    params.Handler,
		workerPool: pool,
		logger:     zerolog.New(nil).With().Str("client_id", uuid.New().String()).Str("user_id", params.UserID.String()).Logger(),
	}

	return client
}

func (c *WsClient) Start() {
	go c.messageSender()
	go c.messageReceiver()
}

func (client *WsClient) Stop() {
	client.mu.Lock()
	defer client.mu.Unlock()

	// Prevent double closing
	if client.stopped {
		return
	}
	client.stopped = true

	client.cancel()
	client.conn.Close()
	close(client.sendChan)

	// Stop the worker pool
	if client.workerPool != nil {
		client.workerPool.Stop()
	}
}

// Send sends a message to the client
func (client *WsClient) Send(msg *ServerMessage) error {
	client.mu.Lock()
	if client.stopped {
		client.mu.Unlock()
		return fmt.Errorf("client is stopped")
	}
	client.mu.Unlock()

	select {
	case client.sendChan <- msg:
		return nil
	default:
		// Channel is full, try to send with a timeout
		select {
		case client.sendChan <- msg:
			return nil
		case <-time.After(100 * time.Millisecond):
			return fmt.Errorf("client send channel is full")
		}
	}
}

func (client *WsClient) messageSender() {
	for {
		select {
		case msg := <-client.sendChan:
			if err := client.sendMessage(msg); err != nil {
				client.logger.Error().Err(err).Msg("Failed to send message to client")
				return
			}
		case <-client.ctx.Done():
			return
		}
	}
}

func (client *WsClient) messageReceiver() {
	// Temporarily disabled read deadline for testing
	/*client.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	client.conn.SetPongHandler(func(string) error {
		client.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		client.logger.Info().Msg("Ping received from client, extending deadline")
		return nil
	})*/

	for {
		select {
		case <-client.ctx.Done():
			return
		default:
			client.logger.Debug().Msg("Reading message from client")
			_, message, err := client.conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					client.logger.Error().Err(err).Msg("WebSocket read error for client")
				} else {
					client.logger.Info().Str("error", err.Error()).Msg("WebSocket connection closed for client")
				}
				// Cancel context to notify handler about disconnection
				client.cancel()
				return
			}
			client.logger.Debug().Str("message", string(message)).Msg("Message received from client")

			client.workerPool.Submit(func() {
				if err := client.handleMessage(message); err != nil {
					client.logger.Error().Err(err).Msg("Failed to handle message in worker pool")
					errorMsg := NewErrorMessage(err.Error(), nil)
					client.sendMessage(errorMsg)
				}
			})
		}
	}
}

func (client *WsClient) sendMessage(msg *ServerMessage) error {
	return client.conn.WriteJSON(msg)
}

func (client *WsClient) handleMessage(data []byte) error {
	msg, err := ParseClientMessage(data)
	if err != nil {
		return fmt.Errorf("invalid message format: %w", err)
	}

	// Validate the message
	if err := msg.Validate(); err != nil {
		return fmt.Errorf("message validation failed: %w", err)
	}

	if msg.Type == MessageTypePing {
		response := NewServerMessage(MessageTypePong)
		return client.Send(response)
	}

	if client.handler != nil {
		return client.handler.HandleClientMessage(client, msg)
	}
	return fmt.Errorf("handler not available")
}
