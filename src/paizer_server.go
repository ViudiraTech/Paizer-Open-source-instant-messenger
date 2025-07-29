/*
 *
 *      paizer_server.go
 *      Paizer instant chat server
 *
 *      Based on MIT open source agreement
 *      Copyright © 2020 ViudiraTech, based on the MIT agreement.
 *
 */

package main

import (
    "bufio"
    "fmt"
    "log"
    "net"
    "os"
    "strconv"
    "strings"
    "sync"
    "sync/atomic"
    "time"
)

var (
    clients    = make(map[string]*Client)
    clientsMux sync.Mutex
    uidCounter uint32
    consoleMux sync.Mutex
)

type Client struct {
    UID      string
    Username string
    IP       string
    Conn     net.Conn
    LastBeat time.Time
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

    listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
    if err != nil {
        log.Fatalf("Unable to start server: %v", err)
    }
    defer listener.Close()

    currentTime := time.Now().Format("15:04:05")
    fmt.Printf("[%s] The server starts and listens on the port: %d\n", currentTime, port)

    go checkHeartbeats()

    for {
        conn, err := listener.Accept()
        if err != nil {
            currentTime := time.Now().Format("15:04:05")
            fmt.Printf("[%s] Failed to accept connection: %v\n", currentTime, err)
            continue
        }
        go handleConnection(conn)
    }
}

/* Address shortening function */
func shortIP(ip string) string {
    if len(ip) > 20 {
        return ip[:10] + "..." + ip[len(ip)-6:]
    }
    return ip
}

/* Connection processing function */
func handleConnection(conn net.Conn) {
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

    /* Read user name */
    reader := bufio.NewReader(conn)
    username, err := reader.ReadString('\n')
    if err != nil {
        currentTime := time.Now().Format("15:04:05")
        fmt.Printf("[%s] Failed to read username: %v\n", currentTime, err)
        return
    }
    username = strings.TrimSpace(username)

    /* Generate a pure numeric UID */
    uid := fmt.Sprintf("%d", atomic.AddUint32(&uidCounter, 1))

    client := &Client{
        UID:      uid,
        Username: username,
        IP:       ip,
        Conn:     conn,
        LastBeat: time.Now(),
    }

    /* Add to client list */
    clientsMux.Lock()
    clients[uid] = client
    clientsMux.Unlock()

    currentTime := time.Now().Format("15:04:05")
    joinMsg := fmt.Sprintf("[%s] %s@%s Join the server.", currentTime, username, ip)

    /* The server prints the join message */
    consoleMux.Lock()
    fmt.Println(joinMsg)
    consoleMux.Unlock()

    /* Broadcast join message */
    broadcast(joinMsg, uid)

    /* Send a welcome message */
    conn.Write([]byte(fmt.Sprintf("You have successfully joined the server!\n")))

    /* Processing messages and heartbeats */
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

                /* The server prints chat messages */
                consoleMux.Lock()
                fmt.Println(formattedMsg)
                consoleMux.Unlock()

                broadcast(formattedMsg, uid)
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

func broadcast(message, excludeUID string) {
    clientsMux.Lock()
    defer clientsMux.Unlock()

    for uid, client := range clients {
        if uid == excludeUID {
            continue
        }
        if _, err := client.Conn.Write([]byte(message + "\n")); err != nil {
            currentTime := time.Now().Format("15:04:05")
            fmt.Printf("[%s] Failed to broadcast message: %v\n", currentTime, err)
        }
    }
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
    broadcast(leaveMsg, uid)

    consoleMux.Lock()
    fmt.Println(leaveMsg)
    consoleMux.Unlock()

    clientsMux.Lock()
    delete(clients, uid)
    clientsMux.Unlock()

    client.Conn.Close()
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
                client.Conn.Close()
                delete(clients, uid)

                leaveMsg := fmt.Sprintf("[%s] %s@%s Disconnected.", currentTime, client.Username, client.IP)

                /* The server prints a leave message */
                consoleMux.Lock()
                fmt.Println(leaveMsg)
                consoleMux.Unlock()

                broadcast(leaveMsg, "")
            }
        }
        clientsMux.Unlock()
    }
}
