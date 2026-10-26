package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"github.com/joho/godotenv"
	"golang.org/x/crypto/ssh"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

//go:embed static/*
var content embed.FS

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file found, continuing...")
	}

	http.HandleFunc("/wake", handleWake)
	http.HandleFunc("/shutdown", handleShutdown)
	staticFs, err := fs.Sub(content, "static")
	if err != nil {
		log.Fatal(err)
	}
	http.Handle("/", http.FileServer(http.FS(staticFs)))

	handler, err := allowSources(http.DefaultServeMux, os.Getenv("ALLOWED_SOURCES"))
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Server running at http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", handler))
}

// allowSources only lets requests from the given comma-separated IPs/CIDRs
// through. The pod runs with hostNetwork, so without it anyone on the LAN can
// wake or shut down the node. An empty list allows everyone.
func allowSources(next http.Handler, list string) (http.Handler, error) {
	var nets []*net.IPNet
	for _, item := range strings.Split(list, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if !strings.Contains(item, "/") {
			if strings.Contains(item, ":") {
				item += "/128"
			} else {
				item += "/32"
			}
		}
		_, n, err := net.ParseCIDR(item)
		if err != nil {
			return nil, fmt.Errorf("invalid ALLOWED_SOURCES entry %q: %w", item, err)
		}
		nets = append(nets, n)
	}
	if len(nets) == 0 {
		return next, nil
	}
	log.Printf("Only accepting requests from %s", list)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		ip := net.ParseIP(host)
		if err == nil && ip != nil {
			for _, n := range nets {
				if n.Contains(ip) {
					next.ServeHTTP(w, r)
					return
				}
			}
		}
		log.Println("Rejected request from", r.RemoteAddr, r.Method, r.URL.Path)
		http.Error(w, "Forbidden", http.StatusForbidden)
	}), nil
}

func handleWake(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		log.Println("Invalid method for /wake endpoint")
		http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
		return
	}

	mac := os.Getenv("NODE_MAC")
	if mac == "" {
		log.Println("NODE_MAC not set")
		http.Error(w, "NODE_MAC not set", http.StatusInternalServerError)
		return
	}

	host := os.Getenv("WAKE_HOST")
	if host == "" {
		log.Println("WAKE_HOST not set")
		http.Error(w, "WAKE_HOST not set", http.StatusInternalServerError)
		return
	}

	log.Println("Sending WOL packet to MAC:", mac, "on host:", host)

	err := exec.Command("wakeonlan", "-i", host, mac).Run()
	if err != nil {
		log.Println("Failed to send WOL packet:", err)
		http.Error(w, "Failed to send WOL packet", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{"status": "wake signal sent"})
}

func handleShutdown(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
		return
	}

	host := os.Getenv("NODE_HOST")
	user := os.Getenv("NODE_USER")
	keyPath := os.Getenv("SSH_KEY_PATH")

	if host == "" || user == "" || keyPath == "" {
		log.Println("NODE_HOST, NODE_USER, or SSH_KEY_PATH not set")
		http.Error(w, "NODE_HOST, NODE_USER, or SSH_KEY_PATH not set", http.StatusInternalServerError)
		return
	}

	key, err := os.ReadFile(keyPath)
	if err != nil {
		log.Println("Failed to read SSH key:", err)
		http.Error(w, "Failed to read SSH key", http.StatusInternalServerError)
		return
	}

	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		log.Println("Invalid private key:", err)
		http.Error(w, "Invalid private key", http.StatusInternalServerError)
		return
	}

	config := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         time.Second, // 1 second
	}

	client, err := ssh.Dial("tcp", host+":22", config)
	if err != nil {
		fmt.Println("Failed to SSH to host", http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"status": "shutdown command NOT sent"})
		return
	}
	defer client.Close()

	sess, err := client.NewSession()
	if err != nil {
		fmt.Println("Failed to create SSH session", http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"status": "shutdown command NOT sent"})
		return
	}
	defer sess.Close()

	// SHUTDOWN_COMMAND lets the node be suspended instead of powered off
	// (e.g. "sudo systemctl suspend"), which wakes much faster via WoL.
	cmd := os.Getenv("SHUTDOWN_COMMAND")
	if cmd == "" {
		cmd = "sudo shutdown now"
	}
	log.Println("Running on", host+":", cmd)
	err = sess.Run(cmd)
	if err != nil {
		fmt.Println("Shutdown command failed", http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"status": "shutdown command NOT sent"})
		return
	}

	json.NewEncoder(w).Encode(map[string]string{"status": "shutdown command sent"})
}
