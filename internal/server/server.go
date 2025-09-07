package server

import (
	"broadcas-server/internal/client"
	"fmt"
	"github.com/gorilla/websocket"
	"log"
	"net/http"
)

var upgrader = websocket.Upgrader{}

type Server struct {
	Clients map[*client.Client]bool
	Join    chan *client.Client
	Forward chan []byte
	Leave   chan *client.Client
}

func NewServer() *Server {
	return &Server{
		Clients: make(map[*client.Client]bool),
		Join:    make(chan *client.Client),
		Forward: make(chan []byte, 100), // 带缓冲，防止Run函数阻塞
		Leave:   make(chan *client.Client),
	}
}

func (s *Server) socketHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("upgrade error: %v", err)
		return
	}
	defer conn.Close()

	c := &client.Client{
		Socket:   conn,
		Username: r.PathValue("username"),
	}

	left := false
	defer func() {
		if !left {
			left = true
			s.Leave <- c
		}
	}()

	s.Join <- c

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			return
		}
		s.Forward <- []byte(fmt.Sprintf("%s: %s", c.Username, string(msg)))
	}
}

func (s *Server) Run() {
	for {
		select {
		case c := <-s.Join:
			s.Clients[c] = true
			s.Forward <- []byte(fmt.Sprintf("user %s joined", c.Username))
			log.Printf("user %s joined\n", c.Username)
		case c := <-s.Leave:
			if s.Clients[c] {
				delete(s.Clients, c)
				s.Forward <- []byte(fmt.Sprintf("user %s left", c.Username))
				log.Printf("user %s left\n", c.Username)
			}
		case msg := <-s.Forward:
			for c := range s.Clients {
				err := c.Socket.WriteMessage(websocket.TextMessage, msg)
				if err != nil {
					log.Printf("write error: %v", err)
				}
			}
		}
	}
}

func StartServer(port string) {
	server := NewServer()
	addr := ":" + port
	go server.Run()
	log.Printf("start server on %v", port)
	http.HandleFunc("/{username}", server.socketHandler)
	err := http.ListenAndServe(addr, nil)
	if err != nil {
		log.Fatalf("start server error: %v", err)
	}
}
