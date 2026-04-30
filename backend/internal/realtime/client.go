package realtime

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/coder/websocket"
)

const (
	writeTimeout = 5 * time.Second
)

type Client struct {
	conn        *websocket.Conn
	hub         *Hub
	log         *slog.Logger
	workspaceID string
	send        chan Envelope
	closeOnce   sync.Once
}

func NewClient(conn *websocket.Conn, hub *Hub, log *slog.Logger, workspaceID string) *Client {
	return &Client{
		conn:        conn,
		hub:         hub,
		log:         log,
		workspaceID: workspaceID,
		send:        make(chan Envelope, 32),
	}
}

func (c *Client) Run(ctx context.Context) {
	c.hub.Subscribe(c.workspaceID, c)
	defer c.Close()

	go c.writeLoop(ctx)
	c.readLoop(ctx)
}

func (c *Client) Send(envelope Envelope) {
	select {
	case c.send <- envelope:
	default:
		c.log.Warn("dropping websocket message for slow client", "workspaceId", c.workspaceID, "type", envelope.Type)
	}
}

func (c *Client) Close() {
	c.closeOnce.Do(func() {
		c.hub.Unsubscribe(c.workspaceID, c)
		close(c.send)
		_ = c.conn.Close(websocket.StatusNormalClosure, "closed")
	})
}

func (c *Client) readLoop(ctx context.Context) {
	for {
		_, data, err := c.conn.Read(ctx)
		if err != nil {
			if !isExpectedClose(err) {
				c.log.Info("websocket read stopped", "workspaceId", c.workspaceID, "error", err)
			}
			return
		}

		var envelope Envelope
		if err := json.Unmarshal(data, &envelope); err != nil {
			c.Send(errorEnvelope("", "invalid_json", "Message must be valid JSON."))
			continue
		}
		c.handleEnvelope(ctx, envelope)
	}
}

func (c *Client) writeLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case envelope, ok := <-c.send:
			if !ok {
				return
			}
			data, err := json.Marshal(envelope)
			if err != nil {
				c.log.Error("failed to marshal websocket envelope", "error", err)
				continue
			}

			writeCtx, cancel := context.WithTimeout(ctx, writeTimeout)
			err = c.conn.Write(writeCtx, websocket.MessageText, data)
			cancel()
			if err != nil {
				c.log.Info("websocket write stopped", "workspaceId", c.workspaceID, "error", err)
				return
			}
		}
	}
}

func (c *Client) handleEnvelope(ctx context.Context, envelope Envelope) {
	switch envelope.Type {
	case MessagePing:
		c.Send(Envelope{Type: MessagePong, RequestID: envelope.RequestID})
	case MessageDesignUpsert:
		var payload UpsertDesignPayload
		if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
			c.Send(errorEnvelope(envelope.RequestID, "invalid_payload", "Invalid design.upsert payload."))
			return
		}
		design, err := c.hub.UpsertDesign(ctx, c.workspaceID, payload)
		if err != nil {
			c.Send(errorEnvelope(envelope.RequestID, "upsert_failed", err.Error()))
			return
		}
		encoded, err := json.Marshal(DesignUpdatedPayload{Design: design})
		if err != nil {
			c.Send(errorEnvelope(envelope.RequestID, "encode_failed", "Could not encode updated design."))
			return
		}
		c.hub.Broadcast(ctx, c.workspaceID, Envelope{
			Type:      MessageDesignUpdated,
			RequestID: envelope.RequestID,
			Payload:   encoded,
		})
	default:
		c.Send(errorEnvelope(envelope.RequestID, "unknown_message_type", "Unknown message type."))
	}
}

func errorEnvelope(requestID string, code string, message string) Envelope {
	return Envelope{
		Type:      MessageError,
		RequestID: requestID,
		Error: &ProtocolError{
			Code:    code,
			Message: message,
		},
	}
}

func isExpectedClose(err error) bool {
	return errors.Is(err, context.Canceled) ||
		websocket.CloseStatus(err) == websocket.StatusNormalClosure ||
		websocket.CloseStatus(err) == websocket.StatusGoingAway
}
