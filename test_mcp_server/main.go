// Simple MCP HTTP Server for testing
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
)

var (
	requestID int64
	mu        sync.Mutex
)

// JSON-RPC types
type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type JSONRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int64       `json:"id,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   interface{} `json:"error,omitempty"`
}

// MCP Tool definitions
var tools = []map[string]interface{}{
	{
		"name":        "echo",
		"description": "Echo back the input message",
		"inputSchema": map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"message": map[string]interface{}{
					"type":        "string",
					"description": "Message to echo back",
				},
			},
			"required": []string{"message"},
		},
	},
	{
		"name":        "add",
		"description": "Add two numbers",
		"inputSchema": map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"a": map[string]interface{}{
					"type":        "number",
					"description": "First number",
				},
				"b": map[string]interface{}{
					"type":        "number",
					"description": "Second number",
				},
			},
			"required": []string{"a", "b"},
		},
	},
	{
		"name":        "get_time",
		"description": "Get current server time",
		"inputSchema": map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
	},
}

func handleMCP(w http.ResponseWriter, r *http.Request) {
	// Set CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Parse request
	var req JSONRPCRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	log.Printf("Received: method=%s id=%d", req.Method, req.ID)

	var result interface{}
	var rpcErr interface{}

	switch req.Method {
	case "initialize":
		result = map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities": map[string]interface{}{
				"tools": map[string]interface{}{},
			},
			"serverInfo": map[string]interface{}{
				"name":    "test-mcp-http-server",
				"version": "1.0.0",
			},
		}

	case "notifications/initialized":
		// Notification, no response needed
		w.WriteHeader(http.StatusOK)
		return

	case "tools/list":
		result = map[string]interface{}{
			"tools": tools,
		}

	case "tools/call":
		var params struct {
			Name      string                 `json:"name"`
			Arguments map[string]interface{} `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &params); err != nil {
			rpcErr = map[string]interface{}{
				"code":    -32602,
				"message": "Invalid params",
			}
		} else {
			result = handleToolCall(params.Name, params.Arguments)
		}

	default:
		rpcErr = map[string]interface{}{
			"code":    -32601,
			"message": fmt.Sprintf("Method not found: %s", req.Method),
		}
	}

	// Send response
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
		Error:   rpcErr,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func handleToolCall(name string, args map[string]interface{}) map[string]interface{} {
	var content string

	switch name {
	case "echo":
		msg, _ := args["message"].(string)
		content = fmt.Sprintf("Echo: %s", msg)

	case "add":
		a, _ := args["a"].(float64)
		b, _ := args["b"].(float64)
		content = fmt.Sprintf("Result: %.0f + %.0f = %.0f", a, b, a+b)

	case "get_time":
		content = fmt.Sprintf("Current server time: %s", "2026-02-16 07:15:00")

	default:
		content = fmt.Sprintf("Unknown tool: %s", name)
	}

	return map[string]interface{}{
		"content": []map[string]interface{}{
			{
				"type": "text",
				"text": content,
			},
		},
	}
}

// SSE endpoint for streaming
func handleSSE(w http.ResponseWriter, r *http.Request) {
	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Send endpoint event
	fmt.Fprintf(w, "event: endpoint\ndata: /mcp\n\n")
	w.(http.Flusher).Flush()

	// Keep connection alive
	<-r.Context().Done()
}

func main() {
	http.HandleFunc("/mcp", handleMCP)
	http.HandleFunc("/sse", handleSSE)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprintln(w, "MCP HTTP Test Server")
		fmt.Fprintln(w, "")
		fmt.Fprintln(w, "Endpoints:")
		fmt.Fprintln(w, "  POST /mcp - JSON-RPC endpoint")
		fmt.Fprintln(w, "  GET  /sse - SSE endpoint")
	})

	port := 8765
	log.Printf("MCP HTTP Test Server starting on port %d...", port)
	log.Printf("Endpoints: http://localhost:%d/mcp, http://localhost:%d/sse", port, port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), nil))
}
