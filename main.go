package main

import (
    "encoding/json"
    "log"
    "net/http"
    "sync"
    "time"

    "github.com/gorilla/websocket"
)

type Location struct {
    Lat      float64 `json:"lat"`
    Lng      float64 `json:"lng"`
    Status   string  `json:"status"`
    Name     string  `json:"name"`
    Initials string  `json:"initials"`
}

type Client struct {
    conn *websocket.Conn
    id   string
}

type Server struct {
    clients   map[string]*Client
    locations map[string]Location
    mutex     sync.Mutex
}

var (
    upgrader = websocket.Upgrader{
        CheckOrigin: func(r *http.Request) bool { return true },
    }
    server = Server{
        clients:   make(map[string]*Client),
        locations: make(map[string]Location),
    }
)

func wsHandler(w http.ResponseWriter, r *http.Request) {
    conn, err := upgrader.Upgrade(w, r, nil)
    if err != nil {
        http.Error(w, "Could not upgrade", http.StatusInternalServerError)
        return
    }

    // Use remote address as simple client ID
    clientID := r.RemoteAddr
    client := &Client{conn: conn, id: clientID}

    server.mutex.Lock()
    server.clients[clientID] = client
    server.mutex.Unlock()

    defer func() {
        server.mutex.Lock()
        delete(server.clients, clientID)
        delete(server.locations, clientID)
        server.mutex.Unlock()
        conn.Close()
        broadcastLocations()
    }()

    for {
        var loc Location
        if err := conn.ReadJSON(&loc); err != nil {
            log.Println("ReadJSON error:", err)
            break
        }

        server.mutex.Lock()
        server.locations[clientID] = loc
        server.mutex.Unlock()
        broadcastLocations()
    }
}

func broadcastLocations() {
    server.mutex.Lock()
    defer server.mutex.Unlock()

    payload, err := json.Marshal(server.locations)
    if err != nil {
        log.Println("JSON marshal error:", err)
        return
    }

    for _, c := range server.clients {
        if err := c.conn.WriteMessage(websocket.TextMessage, payload); err != nil {
            log.Println("WriteMessage error:", err)
        }
    }
}

func main() {
    go func() {
        for {
            // keepalive for render.com
            _, _ = http.Get("https://safely.today/randomurl")
            time.Sleep(10 * time.Second)
        }
    }()
    
    http.HandleFunc("/ws", wsHandler)
    log.Println("Server started on :8080")
    log.Fatal(http.ListenAndServe(":8080", nil))
}
