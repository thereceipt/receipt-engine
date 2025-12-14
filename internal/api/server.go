// Package api handles HTTP and WebSocket API endpoints
package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/yourusername/receipt-engine/internal/parser"
	"github.com/yourusername/receipt-engine/internal/printer"
	"github.com/yourusername/receipt-engine/pkg/receiptformat"
)

// Server is the API server
type Server struct {
	router      *gin.Engine
	manager     *printer.Manager
	pool        *printer.ConnectionPool
	queue       *printer.PrintQueue
	upgrader    websocket.Upgrader
}

// NewServer creates a new API server
func NewServer(manager *printer.Manager, pool *printer.ConnectionPool, queue *printer.PrintQueue) *Server {
	// Set Gin to release mode
	gin.SetMode(gin.ReleaseMode)
	
	router := gin.Default()
	
	// CORS middleware
	router.Use(corsMiddleware())
	
	server := &Server{
		router:  router,
		manager: manager,
		pool:    pool,
		queue:   queue,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins
			},
		},
	}
	
	server.setupRoutes()
	
	return server
}

func (s *Server) setupRoutes() {
	// HTTP API
	s.router.GET("/printers", s.handleGetPrinters)
	s.router.POST("/printer/:id/name", s.handleSetPrinterName)
	s.router.POST("/print", s.handlePrint)
	s.router.GET("/jobs", s.handleGetJobs)
	s.router.GET("/job/:id", s.handleGetJob)
	
	// WebSocket
	s.router.GET("/ws", s.handleWebSocket)
	
	// Health check
	s.router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})
}

// handleGetPrinters returns all detected printers
func (s *Server) handleGetPrinters(c *gin.Context) {
	printers := s.manager.GetAllPrinters()
	
	c.JSON(200, gin.H{
		"printers": printers,
	})
}

// handleSetPrinterName sets a custom name for a printer
func (s *Server) handleSetPrinterName(c *gin.Context) {
	printerID := c.Param("id")
	
	var req struct {
		Name string `json:"name" binding:"required"`
	}
	
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "name is required"})
		return
	}
	
	success := s.manager.SetPrinterName(printerID, req.Name)
	
	if !success {
		c.JSON(404, gin.H{"error": "printer not found"})
		return
	}
	
	c.JSON(200, gin.H{"success": true})
}

// handlePrint handles a print request
func (s *Server) handlePrint(c *gin.Context) {
	var req struct {
		PrinterID         string                            `json:"printer_id" binding:"required"`
		Receipt           *receiptformat.Receipt            `json:"receipt"`
		VariableData      map[string]interface{}            `json:"variableData"`
		VariableArrayData map[string][]map[string]interface{} `json:"variableArrayData"`
	}
	
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	
	// Validate receipt
	if err := receiptformat.Validate(req.Receipt); err != nil {
		c.JSON(400, gin.H{"error": fmt.Sprintf("invalid receipt: %v", err)})
		return
	}
	
	// Create parser
	paperWidth := req.Receipt.PaperWidth
	if paperWidth == "" {
		paperWidth = "80mm"
	}
	
	p, err := parser.New(req.Receipt, paperWidth)
	if err != nil {
		c.JSON(500, gin.H{"error": fmt.Sprintf("failed to create parser: %v", err)})
		return
	}
	
	// Set variable data
	if req.VariableData != nil {
		p.SetVariableData(req.VariableData)
	}
	
	if req.VariableArrayData != nil {
		p.SetVariableArrayData(req.VariableArrayData)
	}
	
	// Execute
	img, err := p.Execute()
	if err != nil {
		c.JSON(500, gin.H{"error": fmt.Sprintf("failed to render receipt: %v", err)})
		return
	}
	
	// Enqueue print job
	jobID := s.queue.Enqueue(req.PrinterID, img)
	
	c.JSON(200, gin.H{
		"success": true,
		"job_id":  jobID,
	})
}

// handleGetJobs returns all print jobs
func (s *Server) handleGetJobs(c *gin.Context) {
	jobs := s.queue.GetAllJobs()
	
	// Convert to JSON-safe format
	jobsData := make([]map[string]interface{}, len(jobs))
	for i, job := range jobs {
		jobsData[i] = map[string]interface{}{
			"id":         job.ID,
			"printer_id": job.PrinterID,
			"status":     job.Status,
			"retries":    job.Retries,
			"created_at": job.CreatedAt,
		}
		if job.Error != nil {
			jobsData[i]["error"] = job.Error.Error()
		}
	}
	
	c.JSON(200, gin.H{"jobs": jobsData})
}

// handleGetJob returns a specific print job
func (s *Server) handleGetJob(c *gin.Context) {
	jobID := c.Param("id")
	
	job := s.queue.GetJob(jobID)
	if job == nil {
		c.JSON(404, gin.H{"error": "job not found"})
		return
	}
	
	jobData := map[string]interface{}{
		"id":         job.ID,
		"printer_id": job.PrinterID,
		"status":     job.Status,
		"retries":    job.Retries,
		"created_at": job.CreatedAt,
	}
	if job.Error != nil {
		jobData["error"] = job.Error.Error()
	}
	
	c.JSON(200, jobData)
}

// Run starts the API server
func (s *Server) Run(addr string) error {
	fmt.Printf("🚀 API Server running on %s\n", addr)
	return s.router.Run(addr)
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		
		c.Next()
	}
}
