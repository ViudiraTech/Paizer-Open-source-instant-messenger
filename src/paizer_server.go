/*
 *
 *      paizer_server.go
 *      Paizer instant chat server (with WebSocket support)
 *
 *      Based on MIT open source agreement
 *      Copyright © 2020 ViudiraTech, based on the MIT agreement.
 *
 */

package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

var (
	clients    = make(map[string]*Client)
	clientsMux sync.Mutex
	uidCounter uint32
	consoleMux sync.Mutex
	upgrader   = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
)

type Client struct {
	UID        string
	Username   string
	IP         string
	Conn       net.Conn        // For TCP client
	WsConn     *websocket.Conn // For WebSocket client
	LastBeat   time.Time
	ClientType string // "tcp" or "websocket"
}

type Message struct {
	Type    string `json:"type"` // "chat", "join", "leave", "heartbeat"
	UID     string `json:"uid,omitempty"`
	User    string `json:"user,omitempty"`
	Content string `json:"content,omitempty"`
	IP      string `json:"ip,omitempty"`
}

/* Server main function */
func main() {
	fmt.Print("Please enter the listening port (press enter, default is 32768): ")
	inputReader := bufio.NewReader(os.Stdin)
	inputPort, _ := inputReader.ReadString('\n')
	inputPort = strings.TrimSpace(inputPort)

	port := 32768
	if inputPort != "" {
		if p, err := strconv.Atoi(inputPort); err == nil {
			port = p
		} else {
			fmt.Printf("The port format is incorrect, using the default port: %d\n", port)
		}
	}

	go startTCPServer(port)
	go startHTTPServer(8080)

	select {}
}

func startTCPServer(port int) {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatalf("Unable to start TCP server: %v", err)
	}
	defer listener.Close()

	currentTime := time.Now().Format("15:04:05")
	fmt.Printf("[%s] TCP server starts and listens on port: %d\n", currentTime, port)

	go checkHeartbeats()

	for {
		conn, err := listener.Accept()
		if err != nil {
			currentTime := time.Now().Format("15:04:05")
			fmt.Printf("[%s] Failed to accept TCP connection: %v\n", currentTime, err)
			continue
		}
		go handleTCPConnection(conn)
	}
}

func startHTTPServer(port int) {
	fs := http.FileServer(http.Dir("./web"))
	http.Handle("/", fs)

	http.HandleFunc("/ws", handleWebSocket)

	currentTime := time.Now().Format("15:04:05")
	fmt.Printf("[%s] HTTP server starts and listens on port: %d\n", currentTime, port)

	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), nil))
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	_, msgBytes, err := conn.ReadMessage()
	if err != nil {
		log.Printf("Failed to read username from WebSocket: %v", err)
		return
	}

	var msg Message
	if err := json.Unmarshal(msgBytes, &msg); err != nil || msg.Type != "join" {
		log.Printf("Invalid join message from WebSocket")
		return
	}

	username := msg.User
	ip := strings.Split(r.RemoteAddr, ":")[0]

	uid := fmt.Sprintf("%d", atomic.AddUint32(&uidCounter, 1))

	client := &Client{
		UID:        uid,
		Username:   username,
		IP:         ip,
		WsConn:     conn,
		LastBeat:   time.Now(),
		ClientType: "websocket",
	}

	clientsMux.Lock()
	clients[uid] = client
	clientsMux.Unlock()

	currentTime := time.Now().Format("15:04:05")
	joinMsg := fmt.Sprintf("[%s] %s@%s Join the server (via Web).", currentTime, username, ip)

	consoleMux.Lock()
	fmt.Println(joinMsg)
	consoleMux.Unlock()

	broadcast(Message{
		Type:    "join",
		UID:     uid,
		User:    username,
		IP:      ip,
		Content: "joined the server",
	}, uid)

	welcomeMsg := Message{
		Type:    "system",
		Content: "You have successfully joined the server via Web!",
	}
	conn.WriteJSON(welcomeMsg)

	for {
		_, msgBytes, err := conn.ReadMessage()
		if err != nil {
			currentTime := time.Now().Format("15:04:05")
			fmt.Printf("[%s] %s@%s WebSocket read error: %v\n", currentTime, username, ip, err)
			removeClient(uid)
			return
		}

		var msg Message
		if err := json.Unmarshal(msgBytes, &msg); err != nil {
			log.Printf("Invalid message format from WebSocket: %v", err)
			continue
		}

		switch msg.Type {
		case "chat":
			currentTime := time.Now().Format("15:04:05")
			formattedMsg := fmt.Sprintf("[%s] [%s@%s] %s", currentTime, username, ip, msg.Content)

			consoleMux.Lock()
			fmt.Println(formattedMsg)
			consoleMux.Unlock()

			broadcast(Message{
				Type:    "chat",
				UID:     uid,
				User:    username,
				IP:      ip,
				Content: msg.Content,
			}, uid)

		case "heartbeat":
			client.LastBeat = time.Now()
		}
	}
}

func handleTCPConnection(conn net.Conn) {
	defer conn.Close()

	remoteAddr := conn.RemoteAddr().String()
	ip, _, err := net.SplitHostPort(remoteAddr)

	if err != nil {
		ip = remoteAddr
	} else {
		parsedIP := net.ParseIP(ip)
		if parsedIP != nil {
			ip = parsedIP.String()
		}
	}

	reader := bufio.NewReader(conn)
	username, err := reader.ReadString('\n')
	if err != nil {
		currentTime := time.Now().Format("15:04:05")
		fmt.Printf("[%s] Failed to read username: %v\n", currentTime, err)
		return
	}
	username = strings.TrimSpace(username)

	uid := fmt.Sprintf("%d", atomic.AddUint32(&uidCounter, 1))

	client := &Client{
		UID:        uid,
		Username:   username,
		IP:         ip,
		Conn:       conn,
		LastBeat:   time.Now(),
		ClientType: "tcp",
	}

	clientsMux.Lock()
	clients[uid] = client
	clientsMux.Unlock()

	currentTime := time.Now().Format("15:04:05")
	joinMsg := fmt.Sprintf("[%s] %s@%s Join the server.", currentTime, username, ip)

	consoleMux.Lock()
	fmt.Println(joinMsg)
	consoleMux.Unlock()

	broadcast(Message{
		Type:    "join",
		UID:     uid,
		User:    username,
		IP:      ip,
		Content: "joined the server",
	}, uid)

	conn.Write([]byte("You have successfully joined the server!\n"))

	messageChan := make(chan string)
	errorChan := make(chan error)

	go func() {
		for {
			msg, err := reader.ReadString('\n')
			if err != nil {
				errorChan <- err
				return
			}
			messageChan <- strings.TrimSpace(msg)
		}
	}()

	heartbeatTicker := time.NewTicker(5 * time.Second)
	defer heartbeatTicker.Stop()
	timeoutTimer := time.NewTimer(10 * time.Second)
	defer timeoutTimer.Stop()

	for {
		select {
		case msg := <-messageChan:
			if msg == "HEARTBEAT" {
				client.LastBeat = time.Now()
				timeoutTimer.Reset(10 * time.Second)
			} else {
				currentTime := time.Now().Format("15:04:05")
				formattedMsg := fmt.Sprintf("[%s] [%s@%s] %s", currentTime, username, ip, msg)

				consoleMux.Lock()
				fmt.Println(formattedMsg)
				consoleMux.Unlock()

				broadcast(Message{
					Type:    "chat",
					UID:     uid,
					User:    username,
					IP:      ip,
					Content: msg,
				}, uid)
			}
		case <-heartbeatTicker.C:
			if _, err := conn.Write([]byte("HEARTBEAT\n")); err != nil {
				currentTime := time.Now().Format("15:04:05")
				fmt.Printf("[%s] %s@%s Failed to send heartbeat: %v\n", currentTime, username, ip, err)
				removeClient(uid)
				return
			}
		case <-timeoutTimer.C:
			currentTime := time.Now().Format("15:04:05")
			fmt.Printf("[%s] %s@%s Heartbeat timeout.\n", currentTime, username, ip)
			removeClient(uid)
			return
		case err := <-errorChan:
			currentTime := time.Now().Format("15:04:05")
			fmt.Printf("[%s] %s@%s Connection Error: %v\n", currentTime, username, ip, err)
			removeClient(uid)
			return
		}
	}
}

func broadcast(msg Message, excludeUID string) {
	clientsMux.Lock()
	defer clientsMux.Unlock()

	for uid, client := range clients {
		if uid == excludeUID {
			continue
		}

		switch client.ClientType {
		case "tcp":
			var output string
			switch msg.Type {
			case "chat":
				output = fmt.Sprintf("[%s] [%s@%s] %s\n",
					time.Now().Format("15:04:05"), msg.User, shortIP(msg.IP), msg.Content)
			case "join":
				output = fmt.Sprintf("[%s] %s@%s joined the chat\n",
					time.Now().Format("15:04:05"), msg.User, shortIP(msg.IP))
			case "leave":
				output = fmt.Sprintf("[%s] %s@%s left the chat\n",
					time.Now().Format("15:04:05"), msg.User, shortIP(msg.IP))
			case "system":
				output = msg.Content + "\n"
			}

			if _, err := client.Conn.Write([]byte(output)); err != nil {
				currentTime := time.Now().Format("15:04:05")
				fmt.Printf("[%s] Failed to broadcast to TCP client: %v\n", currentTime, err)
			}

		case "websocket":
			if err := client.WsConn.WriteJSON(msg); err != nil {
				currentTime := time.Now().Format("15:04:05")
				fmt.Printf("[%s] Failed to broadcast to WebSocket client: %v\n", currentTime, err)
			}
		}
	}
}

/* Address shortening function */
func shortIP(ip string) string {
	if len(ip) > 20 {
		return ip[:10] + "..." + ip[len(ip)-6:]
	}
	return ip
}

func removeClient(uid string) {
	clientsMux.Lock()
	client, exists := clients[uid]
	clientsMux.Unlock()

	if !exists {
		return
	}

	currentTime := time.Now().Format("15:04:05")
	leaveMsg := fmt.Sprintf("[%s] %s@%s Disconnected.", currentTime, client.Username, client.IP)

	broadcastMsg := Message{
		Type:    "leave",
		UID:     client.UID,
		User:    client.Username,
		IP:      client.IP,
		Content: "disconnected",
	}

	broadcast(broadcastMsg, uid)

	consoleMux.Lock()
	fmt.Println(leaveMsg)
	consoleMux.Unlock()

	clientsMux.Lock()
	delete(clients, uid)
	clientsMux.Unlock()

	if client.Conn != nil {
		client.Conn.Close()
	}
	if client.WsConn != nil {
		client.WsConn.Close()
	}
}

func checkHeartbeats() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		clientsMux.Lock()
		for uid, client := range clients {
			if time.Since(client.LastBeat) > 10*time.Second {
				currentTime := time.Now().Format("15:04:05")
				fmt.Printf("[%s] %s@%s Heartbeat detection failed.\n", currentTime, client.Username, client.IP)

				leaveMsg := Message{
					Type:    "leave",
					UID:     client.UID,
					User:    client.Username,
					IP:      client.IP,
					Content: "disconnected",
				}

				if client.Conn != nil {
					client.Conn.Close()
				}
				if client.WsConn != nil {
					client.WsConn.Close()
				}
				delete(clients, uid)

				consoleMsg := fmt.Sprintf("[%s] %s@%s Disconnected.", currentTime, client.Username, client.IP)
				consoleMux.Lock()
				fmt.Println(consoleMsg)
				consoleMux.Unlock()

				broadcast(leaveMsg, "")
			}
		}
		clientsMux.Unlock()
	}
}
