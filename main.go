package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"

	"github.com/joho/godotenv"
	"golang.org/x/crypto/ssh"
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

	fmt.Println("Server running at http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
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

	host := os.Getenv("NODE_HOST")
	if host == "" {
		log.Println("NODE_HOST not set")
		http.Error(w, "NODE_HOST not set", http.StatusInternalServerError)
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

	err = sess.Run("sudo shutdown now")
	if err != nil {
		fmt.Println("Shutdown command failed", http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"status": "shutdown command NOT sent"})
		return
	}

	json.NewEncoder(w).Encode(map[string]string{"status": "shutdown command sent"})
}
