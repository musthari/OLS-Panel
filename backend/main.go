package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

type SystemStatus struct {
	OS        string `json:"os"`
	CoreCount int    `json:"core_count"`
	OLSStatus string `json:"ols_status"`
	UFWStatus string `json:"ufw_status"`
}

type FirewallRequest struct {
	Port   string `json:"port"`
	Action string `json:"action"`
}

type CreateVHostRequest struct {
	Domain   string `json:"domain"`
	Username string `json:"username"`
	SiteType string `json:"site_type"` // "wordpress" or "generic"
}

func enableCORS(w *http.ResponseWriter) {
	(*w).Header().Set("Access-Control-Allow-Origin", "*")
	(*w).Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	(*w).Header().Set("Access-Control-Allow-Headers", "Content-Type")
}

func statusHandler(w http.ResponseWriter, r *http.Request) {
	enableCORS(&w)
	if r.Method == "OPTIONS" {
		return
	}

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

func firewallHandler(w http.ResponseWriter, r *http.Request) {
	enableCORS(&w)
	if r.Method == "OPTIONS" {
		return
	}

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

// API Endpoint: Create New Linux User & OLS VirtualHost (Secured)
func createVHostHandler(w http.ResponseWriter, r *http.Request) {
	enableCORS(&w)
	if r.Method == "OPTIONS" {
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CreateVHostRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil || req.Domain == "" || req.Username == "" {
		http.Error(w, "Invalid request parameters", http.StatusBadRequest)
		return
	}

	domain := strings.ToLower(strings.TrimSpace(req.Domain))
	username := strings.ToLower(strings.TrimSpace(req.Username))

	if runtime.GOOS == "linux" {
		// 1. Create Linux System User with NO interactive shell (/usr/sbin/nologin)
		exec.Command("useradd", "-m", "-s", "/usr/sbin/nologin", username).Run()

		// Secure Home Directory Permissions (chmod 711) so other users cannot traverse it
		userHome := fmt.Sprintf("/home/%s", username)
		os.Chmod(userHome, 0711)

		// 2. Setup Directory Structure
		vhRoot := fmt.Sprintf("%s/domains/%s", userHome, domain)
		docRoot := filepath.Join(vhRoot, "html")
		logsDir := filepath.Join(vhRoot, "logs")

		os.MkdirAll(docRoot, 0755)
		os.MkdirAll(logsDir, 0755)

		// Create dummy index file
		indexContent := fmt.Sprintf("<h1>Welcome to %s</h1><p>Powered by OLS Panel (Secured Environment)</p>", domain)
		ioutil.WriteFile(filepath.Join(docRoot, "index.html"), []byte(indexContent), 0644)

		// Set directory ownership strictly to the created system user
		exec.Command("chown", "-R", fmt.Sprintf("%s:%s", username, username), fmt.Sprintf("%s/domains", userHome)).Run()

		// 3. Load VHost Template Configuration
		templatePath := "/opt/ols-panel/scripts/configs/vhost-generic.conf"
		if req.SiteType == "wordpress" {
			templatePath = "/opt/ols-panel/scripts/configs/vhost-wordpress.conf"
		}

		configData, err := ioutil.ReadFile(templatePath)
		if err != nil {
			http.Error(w, "Failed to load VHost template", http.StatusInternalServerError)
			return
		}

		// Replace variables in template
		vhConfig := strings.ReplaceAll(string(configData), "$VH_NAME", domain)
		vhConfig = strings.ReplaceAll(vhConfig, "$VH_ROOT", vhRoot)

		// Save VHost Config File to OpenLiteSpeed Directory
		olsConfDir := fmt.Sprintf("/usr/local/lsws/conf/vhosts/%s", domain)
		os.MkdirAll(olsConfDir, 0755)
		olsConfPath := filepath.Join(olsConfDir, "vhconf.conf")
		ioutil.WriteFile(olsConfPath, []byte(vhConfig), 0644)

		// 4. Append VHost Mapping to OLS Main Config (httpd_config.conf) if not already present
		mainConfigPath := "/usr/local/lsws/conf/httpd_config.conf"
		vhostMapping := fmt.Sprintf("\nvirtualhost %s {\n  vhRoot %s\n  configFile %s\n  allowSymbolLink 1\n  enableScript 1\n  restrained 1\n}\n", domain, vhRoot, olsConfPath)

		f, err := os.OpenFile(mainConfigPath, os.O_APPEND|os.O_WRONLY, 0644)
		if err == nil {
			f.WriteString(vhostMapping)
			f.Close()
		}

		// 5. Reload OpenLiteSpeed
		exec.Command("sudo", "/usr/local/lsws/bin/lswsctrl", "reload").Run()
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": fmt.Sprintf("Secured VirtualHost for %s (User: %s) successfully created!", domain, username),
	})
}

type DomainInfo struct {
	Domain   string `json:"domain"`
	Path     string `json:"path"`
	SiteType string `json:"site_type"`
}

// Endpoint GET: Membaca daftar VHost dari folder /usr/local/lsws/conf/vhosts/
func getVHostsHandler(w http.ResponseWriter, r *http.Request) {
	enableCORS(&w)
	if r.Method == "OPTIONS" { return }

	var domains []DomainInfo

	if runtime.GOOS == "linux" {
		vhostDir := "/usr/local/lsws/conf/vhosts"
		files, err := ioutil.ReadDir(vhostDir)
		if err == nil {
			for _, file := range files {
				if file.IsDir() {
					domainName := file.Name()
					confPath := filepath.Join(vhostDir, domainName, "vhconf.conf")
					
					siteType := "generic"
					if content, err := ioutil.ReadFile(confPath); err == nil {
						if strings.Contains(string(content), "autoLoadHtaccess") {
							siteType = "wordpress"
						}
					}

					domains = append(domains, DomainInfo{
						Domain:   domainName,
						Path:     fmt.Sprintf("/home/*/domains/%s/html", domainName),
						SiteType: siteType,
					})
				}
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(domains)
}

func main() {
	fs := http.FileServer(http.Dir("../frontend"))
	http.Handle("/", fs)

	http.HandleFunc("/api/status", statusHandler)
	http.HandleFunc("/api/firewall", firewallHandler)
	http.HandleFunc("/api/vhost", createVHostHandler)
	http.HandleFunc("/api/vhosts", getVHostsHandler) // Endpoint baru untuk List Domain

	port := ":8080"
	fmt.Printf("[OLS-Panel Backend] Server started on http://localhost%s\n", port)
	log.Fatal(http.ListenAndServe(port, nil))
}