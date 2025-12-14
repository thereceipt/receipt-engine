package api

import (
	"encoding/json"
	"fmt"
	"sync"
	
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/thereceipt/receipt-engine/internal/parser"
	"github.com/thereceipt/receipt-engine/internal/printer"
	"github.com/thereceipt/receipt-engine/pkg/receiptformat"
)

// WebSocket message types
const (
	EventPrint          = "print"
	EventPrinterAdded   = "printer_added"
	EventPrinterRemoved = "printer_removed"
	EventResponse       = "response"
	EventError          = "error"
)

// WSMessage represents a WebSocket message
type WSMessage struct {
	Event string                 `json:"event"`
	Data  map[string]interface{} `json:"data"`
}

// WSClient represents a connected WebSocket client
type WSClient struct {
	conn   *websocket.Conn
	send   chan WSMessage
	server *Server
	mu     sync.Mutex
}

// handleWebSocket handles WebSocket connections
func (s *Server) handleWebSocket(c *gin.Context) {
	conn, err := s.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		fmt.Printf("WebSocket upgrade failed: %v\n", err)
		return
	}
	
	client := &WSClient{
		conn:   conn,
		send:   make(chan WSMessage, 256),
		server: s,
	}
	
	fmt.Println("📡 WebSocket client connected")
	
	// Start goroutines
	go client.readPump()
	go client.writePump()
}

func (c *WSClient) writePump() {
	defer c.conn.Close()
	
	for msg := range c.send {
		c.mu.Lock()
		err := c.conn.WriteJSON(msg)
		c.mu.Unlock()
		
		if err != nil {
			fmt.Printf("WebSocket write error: %v\n", err)
			return
		}
	}
}

func (c *WSClient) handleMessage(msg *WSMessage) {
	switch msg.Event {
	case EventPrint:
		c.handlePrintEvent(msg.Data)
	default:
		c.sendError(fmt.Sprintf("unknown event: %s", msg.Event))
	}
}

func (c *WSClient) handlePrintEvent(data map[string]interface{}) {
	// Extract printer_id
	printerID, ok := data["printer_id"].(string)
	if !ok {
		c.sendError("printer_id is required")
		return
	}
	
	// Load receipt from path/URL if provided, otherwise use direct receipt
	var receipt *receiptformat.Receipt
	var err error
	
	// Check for receipt_url first
	if receiptURL, ok := data["receipt_url"].(string); ok && receiptURL != "" {
		receipt, err = loadReceiptFromPathOrURL(receiptURL)
		if err != nil {
			c.sendError(fmt.Sprintf("failed to load receipt from URL: %v", err))
			return
		}
	} else if receiptPath, ok := data["receipt_path"].(string); ok && receiptPath != "" {
		// Check for receipt_path
		receipt, err = loadReceiptFromPathOrURL(receiptPath)
		if err != nil {
			c.sendError(fmt.Sprintf("failed to load receipt from path: %v", err))
			return
		}
	} else if receiptData, ok := data["receipt"]; ok {
		// Use direct receipt JSON
		receiptBytes, _ := json.Marshal(receiptData)
		var receiptObj receiptformat.Receipt
		if err := json.Unmarshal(receiptBytes, &receiptObj); err != nil {
			c.sendError(fmt.Sprintf("invalid receipt: %v", err))
			return
		}
		receipt = &receiptObj
	} else {
		c.sendError("receipt, receipt_path, or receipt_url is required")
		return
	}
	
	// Validate
	if err := receiptformat.Validate(receipt); err != nil {
		c.sendError(fmt.Sprintf("receipt validation failed: %v", err))
		return
	}
	
	// Parse variable data
	var variableData map[string]interface{}
	if vd, ok := data["variableData"]; ok {
		variableData, _ = vd.(map[string]interface{})
	}
	
	var variableArrayData map[string][]map[string]interface{}
	if vad, ok := data["variableArrayData"]; ok {
		// Convert from interface{} to proper type
		if vadMap, ok := vad.(map[string]interface{}); ok {
			variableArrayData = make(map[string][]map[string]interface{})
			for key, val := range vadMap {
				if arr, ok := val.([]interface{}); ok {
					entries := make([]map[string]interface{}, len(arr))
					for i, entry := range arr {
						if entryMap, ok := entry.(map[string]interface{}); ok {
							entries[i] = entryMap
						}
					}
					variableArrayData[key] = entries
				}
			}
		}
	}
	
	// Create parser
	paperWidth := receipt.PaperWidth
	if paperWidth == "" {
		paperWidth = "80mm"
	}
	
	p, err := parser.New(receipt, paperWidth)
	if err != nil {
		c.sendError(fmt.Sprintf("failed to create parser: %v", err))
		return
	}
	
	if variableData != nil {
		p.SetVariableData(variableData)
	}
	
	if variableArrayData != nil {
		p.SetVariableArrayData(variableArrayData)
	}
	
	// Execute
	img, err := p.Execute()
	if err != nil {
		c.sendError(fmt.Sprintf("failed to render receipt: %v", err))
		return
	}
	
	// Enqueue print job
	jobID := c.server.queue.Enqueue(printerID, img)
	
	c.sendResponse(map[string]interface{}{
		"success": true,
		"job_id":  jobID,
	})
}

func (c *WSClient) sendResponse(data map[string]interface{}) {
	c.send <- WSMessage{
		Event: EventResponse,
		Data:  data,
	}
}

// Client tracking for broadcasts
var (
	clients   = make(map[*WSClient]bool)
	clientsMu sync.RWMutex
)

func addClient(client *WSClient) {
	clientsMu.Lock()
	clients[client] = true
	clientsMu.Unlock()
}

func removeClient(client *WSClient) {
	clientsMu.Lock()
	delete(clients, client)
	clientsMu.Unlock()
}

func (c *WSClient) readPump() {
	defer func() {
		removeClient(c)
		c.conn.Close()
		fmt.Println("📡 WebSocket client disconnected")
	}()
	
	addClient(c)
	
	for {
		var msg WSMessage
		err := c.conn.ReadJSON(&msg)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				fmt.Printf("WebSocket error: %v\n", err)
			}
			break
		}
		
		c.handleMessage(&msg)
	}
}

func (c *WSClient) sendError(message string) {
	c.send <- WSMessage{
		Event: EventError,
		Data: map[string]interface{}{
			"error": message,
		},
	}
}

// BroadcastPrinterAdded broadcasts a printer added event to all connected clients
func (s *Server) BroadcastPrinterAdded(printer *printer.Printer) {
	clientsMu.RLock()
	defer clientsMu.RUnlock()
	
	message := WSMessage{
		Event: EventPrinterAdded,
		Data: map[string]interface{}{
			"id":          printer.ID,
			"type":        printer.Type,
			"description": printer.Description,
			"name":        printer.Name,
		},
	}
	
	for client := range clients {
		select {
		case client.send <- message:
		default:
			// Client send buffer full, skip
		}
	}
	
	fmt.Printf("📡 Broadcast: Printer added - %s\n", printer.Description)
}

// BroadcastPrinterRemoved broadcasts a printer removed event to all connected clients
func (s *Server) BroadcastPrinterRemoved(printerID string) {
	clientsMu.RLock()
	defer clientsMu.RUnlock()
	
	message := WSMessage{
		Event: EventPrinterRemoved,
		Data: map[string]interface{}{
			"id": printerID,
		},
	}
	
	for client := range clients {
		select {
		case client.send <- message:
		default:
			// Client send buffer full, skip
		}
	}
	
	fmt.Printf("📡 Broadcast: Printer removed - %s\n", printerID)
}
