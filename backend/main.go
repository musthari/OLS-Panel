package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
)

// Response Structure
type SystemStatus struct {
	OS          string `json:"os"`
	CoreCount   int    `json:"core_count"`
	OLSStatus   string `json:"ols_status"`
	UFWStatus   string `json:"ufw_status"`
}

type FirewallRequest struct {
	Port   string `json:"port"`
	Action string `json:"action"` // "allow" or "deny"
}

func enableCORS(w *http.ResponseWriter) {
	(*w).Header().Set("Access-Control-Allow-Origin", "*")
	(*w).Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	(*w).Header().Set("Access-Control-Allow-Headers", "Content-Type")
}

// API Endpoint: Health Check & System Status
func statusHandler(w http.ResponseWriter, r *http.Request) {
	enableCORS(&w)
	if r.Method == "OPTIONS" { return }

	// Check OpenLiteSpeed Status (Linux only)
	olsStatus := "running"
	if runtime.GOOS == "linux" {
		out, err := exec.Command("systemctl", "is-active", "lsws").Output()
		if err != nil || strings.TrimSpace(string(out)) != "active" {
			olsStatus = "stopped"
		}
	}

	status := SystemStatus{
		OS:        runtime.GOOS,
		CoreCount: runtime.NumCPU(),
		OLSStatus: olsStatus,
		UFWStatus: "active",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// API Endpoint: Manage Firewall Port (Allow / Deny)
func firewallHandler(w http.ResponseWriter, r *http.Request) {
	enableCORS(&w)
	if r.Method == "OPTIONS" { return }

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req FirewallRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil || req.Port == "" || (req.Action != "allow" && req.Action != "deny") {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Execute ufw command via sudo
	if runtime.GOOS == "linux" {
		cmd := exec.Command("sudo", "ufw", req.Action, req.Port+"/tcp")
		err := cmd.Run()
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to update firewall: %v", err), http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": fmt.Sprintf("Port %s successfully set to %s", req.Port, req.Action),
	})
}

func main() {
// Serve static UI files from frontend directory
	fs := http.FileServer(http.Dir("../frontend"))
	http.Handle("/", fs)

	// API Routes
	http.HandleFunc("/api/status", statusHandler)
	http.HandleFunc("/api/firewall", firewallHandler)

	port := ":8080"
	fmt.Printf("[OLS-Panel Backend] Server started on http://localhost%s\n", port)
	log.Fatal(http.ListenAndServe(port, nil))
}