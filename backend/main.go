package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
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

	_ "github.com/go-sql-driver/mysql"
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

type DeleteVHostRequest struct {
	Domain   string `json:"domain"`
	Username string `json:"username"`
}

type CreateDBRequest struct {
	Domain string `json:"domain"`
	DBName string `json:"db_name"`
	DBUser string `json:"db_user"`
}

type DeleteDBRequest struct {
	DBName string `json:"db_name"`
	DBUser string `json:"db_user"`
}

type SaveSSLRequest struct {
	Domain string `json:"domain"`
	Cert   string `json:"cert"`
	Key    string `json:"key"`
}

type DomainInfo struct {
	Domain   string `json:"domain"`
	Username string `json:"username"`
	Path     string `json:"path"`
	SiteType string `json:"site_type"`
	DBName   string `json:"db_name"`
	DBUser   string `json:"db_user"`
}

func enableCORS(w *http.ResponseWriter) {
	(*w).Header().Set("Access-Control-Allow-Origin", "*")
	(*w).Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
	(*w).Header().Set("Access-Control-Allow-Headers", "Content-Type")
}

func generateRandomPassword(length int) string {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return "DefaultPass123!"
	}
	return hex.EncodeToString(b)[:length]
}

func getMySQLDB() (*sql.DB, error) {
	return sql.Open("mysql", "root@unix(/run/mysqld/mysqld.sock)/")
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

type FirewallRule struct {
	Port     string `json:"port"`
	Protocol string `json:"protocol"`
	Action   string `json:"action"`
	Comment  string `json:"comment"`
}

func firewallHandler(w http.ResponseWriter, r *http.Request) {
	enableCORS(&w)
	if r.Method == "OPTIONS" {
		return
	}

	// 1. GET: Ambil daftar port UFW yang terbuka
	if r.Method == http.MethodGet {
		var rules []FirewallRule

		if runtime.GOOS == "linux" {
			out, err := exec.Command("sudo", "ufw", "status", "numbered").Output()
			if err == nil {
				lines := strings.Split(string(out), "\n")
				for _, line := range lines {
					line = strings.TrimSpace(line)
					if strings.HasPrefix(line, "[") {
						// Format baris UFW: [ 1] 80/tcp ALLOW IN Anywhere # comment
						parts := strings.Fields(line)
						if len(parts) >= 3 {
							portProto := parts[1]
							action := parts[2]

							portParts := strings.Split(portProto, "/")
							port := portParts[0]
							proto := "tcp"
							if len(portParts) > 1 {
								proto = portParts[1]
							}

							comment := ""
							if idx := strings.Index(line, "#"); idx != -1 {
								comment = strings.TrimSpace(line[idx+1:])
							}

							// Hindari duplikat IPv6 jika port sama
							exists := false
							for _, r := range rules {
								if r.Port == port && r.Protocol == proto {
									exists = true
									break
								}
							}

							if !exists && port != "" {
								rules = append(rules, FirewallRule{
									Port:     port,
									Protocol: proto,
									Action:   action,
									Comment:  comment,
								})
							}
						}
					}
				}
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(rules)
		return
	}

	// 2. POST: Tambahkan port UFW baru
	if r.Method == http.MethodPost {
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
		return
	}

	// 3. DELETE: Hapus port dari UFW
	if r.Method == http.MethodDelete {
		var req FirewallRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		if err != nil || req.Port == "" {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if runtime.GOOS == "linux" {
			exec.Command("sudo", "ufw", "delete", "allow", req.Port+"/tcp").Run()
			exec.Command("sudo", "ufw", "delete", "deny", req.Port+"/tcp").Run()
			exec.Command("sudo", "ufw", "delete", "allow", req.Port).Run()
			exec.Command("sudo", "ufw", "delete", "deny", req.Port).Run()
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"message": fmt.Sprintf("Port %s successfully deleted from firewall!", req.Port),
		})
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

func vhostHandler(w http.ResponseWriter, r *http.Request) {
	enableCORS(&w)
	if r.Method == "OPTIONS" {
		return
	}

	if r.Method == http.MethodPost {
		var req CreateVHostRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		if err != nil || req.Domain == "" || req.Username == "" {
			http.Error(w, "Invalid request parameters", http.StatusBadRequest)
			return
		}

		domain := strings.ToLower(strings.TrimSpace(req.Domain))
		username := strings.ToLower(strings.TrimSpace(req.Username))

		if runtime.GOOS == "linux" {
			exec.Command("useradd", "-m", "-s", "/usr/sbin/nologin", username).Run()

			userHome := fmt.Sprintf("/home/%s", username)
			os.Chmod(userHome, 0711)

			vhRoot := fmt.Sprintf("%s/domains/%s", userHome, domain)
			docRoot := filepath.Join(vhRoot, "html")
			logsDir := filepath.Join(vhRoot, "logs")

			os.MkdirAll(docRoot, 0755)
			os.MkdirAll(logsDir, 0755)

			indexContent := fmt.Sprintf("<h1>Welcome to %s</h1><p>Powered by OLS Panel (Secured Environment)</p>", domain)
			ioutil.WriteFile(filepath.Join(docRoot, "index.html"), []byte(indexContent), 0644)

			exec.Command("chown", "-R", fmt.Sprintf("%s:%s", username, username), fmt.Sprintf("%s/domains", userHome)).Run()

			templatePath := "/opt/ols-panel/scripts/configs/vhost-generic.conf"
			if req.SiteType == "wordpress" {
				templatePath = "/opt/ols-panel/scripts/configs/vhost-wordpress.conf"
			}

			configData, err := ioutil.ReadFile(templatePath)
			if err != nil {
				http.Error(w, "Failed to load VHost template", http.StatusInternalServerError)
				return
			}

			vhConfig := strings.ReplaceAll(string(configData), "$VH_NAME", domain)
			vhConfig = strings.ReplaceAll(vhConfig, "$VH_ROOT", vhRoot)

			olsConfDir := fmt.Sprintf("/usr/local/lsws/conf/vhosts/%s", domain)
			os.MkdirAll(olsConfDir, 0755)
			olsConfPath := filepath.Join(olsConfDir, "vhconf.conf")
			ioutil.WriteFile(olsConfPath, []byte(vhConfig), 0644)

			mainConfigPath := "/usr/local/lsws/conf/httpd_config.conf"
			vhostMapping := fmt.Sprintf("\nvirtualhost %s {\n  vhRoot %s\n  configFile %s\n  allowSymbolLink 1\n  enableScript 1\n  restrained 1\n}\n", domain, vhRoot, olsConfPath)

			f, err := os.OpenFile(mainConfigPath, os.O_APPEND|os.O_WRONLY, 0644)
			if err == nil {
				f.WriteString(vhostMapping)
				f.Close()
			}

			exec.Command("sudo", "/usr/local/lsws/bin/lswsctrl", "reload").Run()
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"message": fmt.Sprintf("Secured VirtualHost for %s (User: %s) successfully created!", domain, username),
		})
		return
	}

	if r.Method == http.MethodDelete {
		var req DeleteVHostRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		if err != nil || req.Domain == "" {
			http.Error(w, "Invalid request parameters", http.StatusBadRequest)
			return
		}

		domain := strings.ToLower(strings.TrimSpace(req.Domain))

		if runtime.GOOS == "linux" {
			olsConfDir := fmt.Sprintf("/usr/local/lsws/conf/vhosts/%s", domain)
			os.RemoveAll(olsConfDir)

			mainConfigPath := "/usr/local/lsws/conf/httpd_config.conf"
			if content, err := ioutil.ReadFile(mainConfigPath); err == nil {
				lines := strings.Split(string(content), "\n")
				var newLines []string
				inBlock := false

				for _, line := range lines {
					if strings.HasPrefix(strings.TrimSpace(line), fmt.Sprintf("virtualhost %s {", domain)) {
						inBlock = true
						continue
					}
					if inBlock && strings.TrimSpace(line) == "}" {
						inBlock = false
						continue
					}
					if !inBlock {
						newLines = append(newLines, line)
					}
				}
				ioutil.WriteFile(mainConfigPath, []byte(strings.Join(newLines, "\n")), 0644)
			}

			if req.Username != "" {
				exec.Command("userdel", "-r", req.Username).Run()
			}

			exec.Command("sudo", "/usr/local/lsws/bin/lswsctrl", "reload").Run()
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"message": fmt.Sprintf("VirtualHost %s successfully deleted!", domain),
		})
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

func getVHostsHandler(w http.ResponseWriter, r *http.Request) {
	enableCORS(&w)
	if r.Method == "OPTIONS" {
		return
	}

	var domains []DomainInfo

	if runtime.GOOS == "linux" {
		vhostDir := "/usr/local/lsws/conf/vhosts"
		files, err := ioutil.ReadDir(vhostDir)
		if err == nil {
			dbMap := make(map[string]map[string]string)

			dbMetaPath := "/opt/ols-panel/db_meta.json"
			if metaData, err := ioutil.ReadFile(dbMetaPath); err == nil {
				json.Unmarshal(metaData, &dbMap)
			}

			for _, file := range files {
				if file.IsDir() {
					domainName := file.Name()
					confPath := filepath.Join(vhostDir, domainName, "vhconf.conf")

					siteType := "generic"
					username := "unknown"

					if content, err := ioutil.ReadFile(confPath); err == nil {
						strContent := string(content)
						if strings.Contains(strContent, "autoLoadHtaccess") {
							siteType = "wordpress"
						}

						for _, line := range strings.Split(strContent, "\n") {
							if strings.HasPrefix(strings.TrimSpace(line), "docRoot") {
								parts := strings.Split(line, "/")
								if len(parts) >= 3 && parts[1] == "home" {
									username = parts[2]
								}
							}
						}
					}

					dbName := "-"
					dbUser := "-"
					if metaInfo, exists := dbMap[domainName]; exists {
						dbName = metaInfo["db_name"]
						dbUser = metaInfo["db_user"]
					}

					domains = append(domains, DomainInfo{
						Domain:   domainName,
						Username: username,
						Path:     fmt.Sprintf("/home/%s/domains/%s/html", username, domainName),
						SiteType: siteType,
						DBName:   dbName,
						DBUser:   dbUser,
					})
				}
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(domains)
}

func databaseHandler(w http.ResponseWriter, r *http.Request) {
	enableCORS(&w)
	if r.Method == "OPTIONS" {
		return
	}

	if r.Method == http.MethodPost {
		var req CreateDBRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		if err != nil || req.Domain == "" || req.DBName == "" || req.DBUser == "" {
			http.Error(w, "Invalid parameters", http.StatusBadRequest)
			return
		}

		dbName := strings.TrimSpace(req.DBName)
		dbUser := strings.TrimSpace(req.DBUser)
		dbPass := generateRandomPassword(16)

		if runtime.GOOS == "linux" {
			db, err := getMySQLDB()
			if err != nil {
				http.Error(w, fmt.Sprintf("Database connection error: %v", err), http.StatusInternalServerError)
				return
			}
			defer db.Close()

			_, err = db.Exec(fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s`;", dbName))
			if err != nil {
				http.Error(w, fmt.Sprintf("Failed to create DB: %v", err), http.StatusInternalServerError)
				return
			}

			db.Exec(fmt.Sprintf("CREATE USER IF NOT EXISTS '%s'@'localhost' IDENTIFIED BY '%s';", dbUser, dbPass))
			db.Exec(fmt.Sprintf("GRANT ALL PRIVILEGES ON `%s`.* TO '%s'@'localhost';", dbName, dbUser))
			db.Exec("FLUSH PRIVILEGES;")

			dbMetaPath := "/opt/ols-panel/db_meta.json"
			dbMap := make(map[string]map[string]string)

			if metaData, err := ioutil.ReadFile(dbMetaPath); err == nil {
				json.Unmarshal(metaData, &dbMap)
			}

			dbMap[req.Domain] = map[string]string{
				"db_name": dbName,
				"db_user": dbUser,
			}

			updatedMeta, _ := json.MarshalIndent(dbMap, "", "  ")
			ioutil.WriteFile(dbMetaPath, updatedMeta, 0644)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"message":  fmt.Sprintf("Database '%s' and User '%s' created successfully!", dbName, dbUser),
			"db_name":  dbName,
			"db_user":  dbUser,
			"db_pass":  dbPass,
		})
		return
	}

	if r.Method == http.MethodDelete {
		var req DeleteDBRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		if err != nil || req.DBName == "" {
			http.Error(w, "Invalid parameters", http.StatusBadRequest)
			return
		}

		if runtime.GOOS == "linux" {
			db, err := getMySQLDB()
			if err != nil {
				http.Error(w, fmt.Sprintf("Database connection error: %v", err), http.StatusInternalServerError)
				return
			}
			defer db.Close()

			db.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS `%s`;", req.DBName))
			if req.DBUser != "" {
				db.Exec(fmt.Sprintf("DROP USER IF EXISTS '%s'@'localhost';", req.DBUser))
			}
			db.Exec("FLUSH PRIVILEGES;")

			dbMetaPath := "/opt/ols-panel/db_meta.json"
			dbMap := make(map[string]map[string]string)

			if metaData, err := ioutil.ReadFile(dbMetaPath); err == nil {
				json.Unmarshal(metaData, &dbMap)
				for domain, meta := range dbMap {
					if meta["db_name"] == req.DBName {
						delete(dbMap, domain)
					}
				}
				updatedMeta, _ := json.MarshalIndent(dbMap, "", "  ")
				ioutil.WriteFile(dbMetaPath, updatedMeta, 0644)
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"message": fmt.Sprintf("Database '%s' successfully deleted!", req.DBName),
		})
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

func sslHandler(w http.ResponseWriter, r *http.Request) {
	enableCORS(&w)
	if r.Method == "OPTIONS" {
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req SaveSSLRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil || req.Domain == "" || req.Cert == "" || req.Key == "" {
		http.Error(w, "Domain, Certificate, and Key are required", http.StatusBadRequest)
		return
	}

	domain := strings.ToLower(strings.TrimSpace(req.Domain))

	if runtime.GOOS == "linux" {
		sslDir := fmt.Sprintf("/usr/local/lsws/conf/vhosts/%s/ssl", domain)
		os.MkdirAll(sslDir, 0700)

		certPath := filepath.Join(sslDir, "fullchain.pem")
		keyPath := filepath.Join(sslDir, "privkey.pem")

		errCert := ioutil.WriteFile(certPath, []byte(strings.TrimSpace(req.Cert)), 0600)
		errKey := ioutil.WriteFile(keyPath, []byte(strings.TrimSpace(req.Key)), 0600)

		if errCert != nil || errKey != nil {
			http.Error(w, "Failed to write SSL certificate files", http.StatusInternalServerError)
			return
		}

		confPath := fmt.Sprintf("/usr/local/lsws/conf/vhosts/%s/vhconf.conf", domain)
		content, err := ioutil.ReadFile(confPath)
		if err == nil {
			strContent := string(content)

			if !strings.Contains(strContent, "vhssl") {
				sslConfig := fmt.Sprintf("\nvhssl  {\n  keyFile                 %s\n  certFile                %s\n  certChain               1\n}\n", keyPath, certPath)
				strContent += sslConfig
				ioutil.WriteFile(confPath, []byte(strContent), 0644)
			}
		}

		exec.Command("sudo", "/usr/local/lsws/bin/lswsctrl", "reload").Run()
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": fmt.Sprintf("Cloudflare SSL Certificate successfully installed for %s!", domain),
	})
}

func main() {
	fs := http.FileServer(http.Dir("../frontend"))
	http.Handle("/", fs)

	http.HandleFunc("/api/status", statusHandler)
	http.HandleFunc("/api/firewall", firewallHandler)
	http.HandleFunc("/api/vhost", vhostHandler)
	http.HandleFunc("/api/vhosts", getVHostsHandler)
	http.HandleFunc("/api/database", databaseHandler)
	http.HandleFunc("/api/ssl", sslHandler)

	port := ":8080"
	fmt.Printf("[OLS-Panel Backend] Server started on http://localhost%s\n", port)
	log.Fatal(http.ListenAndServe(port, nil))
}