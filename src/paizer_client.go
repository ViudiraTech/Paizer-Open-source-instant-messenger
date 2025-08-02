/*
 *
 *      paizer_client.go
 *      Paizer instant chat client
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
    "strings"
    "time"
    "github.com/chzyer/readline"
)

const (
    defaultPort    = "32768"
    heartbeatMsg   = "HEARTBEAT"
    heartbeatDelay = 5 * time.Second
    timeoutLimit   = 10 * time.Second
)

/* Address resolution function */
func parseAddress(addr string) (string, error) {
    if strings.Contains(addr, "[") && strings.Contains(addr, "]") {
        return addr, nil
    }
    if strings.Contains(addr, ":") {
        host, port, err := net.SplitHostPort(addr)
        if err != nil {
            return "", err
        }
        if strings.Contains(host, ":") {
            return "[" + host + "]:" + port, nil
        }
        return addr, nil
    }
    return net.JoinHostPort(addr, defaultPort), nil
}

/* Client main function */
func main() {
    rl, err := readline.New("> ")
    if err != nil {
        log.Fatalf("Readline initialization failed: %v", err)
    }
    defer rl.Close()

    fmt.Print("Server IP address (default 127.0.0.1): ")
    var addr string
    fmt.Scanln(&addr)
    if addr == "" {
        addr = "127.0.0.1"
    }

    serverAddr, err := parseAddress(addr)
    if err != nil {
        log.Fatalf("Wrong address format: %v", err)
    }

    conn, err := net.Dial("tcp", serverAddr)
    if err != nil {
        log.Fatalf("Unable to connect to server: %v", err)
    }
    defer conn.Close()

    fmt.Print("Please enter your nickname: ")
    var username string
    fmt.Scanln(&username)

    _, err = conn.Write([]byte(username + "\n"))
    if err != nil {
        log.Fatalf("Failed to send nickname: %v", err)
    }

    welcome, err := bufio.NewReader(conn).ReadString('\n')
    if err != nil {
        log.Fatalf("Failed to read the welcome message: %v", err)
    }
    fmt.Println(strings.TrimSpace(welcome))

    messageChan := make(chan string)
    errorChan := make(chan error)
    inputChan := make(chan string)

    heartbeatTicker := time.NewTicker(heartbeatDelay)
    defer heartbeatTicker.Stop()

    timeoutTimer := time.NewTimer(timeoutLimit)
    defer timeoutTimer.Stop()

    /* Network message receiving coroutine */
    go func() {
        reader := bufio.NewReader(conn)
        for {
            msg, err := reader.ReadString('\n')
            if err != nil {
                errorChan <- err
                return
            }
            msg = strings.TrimSpace(msg)
            messageChan <- msg
        }
    }()

    /* Heartbeat packet sending coroutine */
    go func() {
        for range heartbeatTicker.C {
            _, err := conn.Write([]byte(heartbeatMsg + "\n"))
            if err != nil {
                errorChan <- err
                return
            }
        }
    }()

    /* User input reading coroutine, responsible for calling the blocking rl.Readline() */
    go func() {
        for {
            line, err := rl.Readline()
            if err != nil {
                errorChan <- err
                return
            }
            inputChan <- strings.TrimSpace(line)
        }
    }()

    for {
        select {
        case msg := <-messageChan:
            if msg == heartbeatMsg {
                timeoutTimer.Reset(timeoutLimit)
                continue
            }
            rl.Write([]byte(msg + "\n"))
            timeoutTimer.Reset(timeoutLimit)
        case input := <-inputChan:
            if input != "" {
                _, err := conn.Write([]byte(input + "\n"))
                if err != nil {
                    errorChan <- err
                    return
                }
                timeoutTimer.Reset(timeoutLimit)
            }
            rl.SetPrompt("> ")
            rl.Refresh()
        case err := <-errorChan:
            fmt.Println("\nConnection error or input error: ", err)
            return
        case <-timeoutTimer.C:
            fmt.Println("\nThe connection to the server timed out.")
            return
        }
    }
}
