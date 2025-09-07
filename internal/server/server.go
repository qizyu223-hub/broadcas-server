package server

import (
	"broadcas-server/internal/client"
	"context"
	"errors"
	"fmt"
	"github.com/gorilla/websocket"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
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
					log.Printf("write error, user %s will be removed: %v", c.Username, err)
					// 写入失败，意味着连接可能已断开
					// 启动一个新的 goroutine 来处理离开逻辑，以避免阻塞当前的消息广播循环
					go func(clientToLeave *client.Client) {
						s.Leave <- clientToLeave
					}(c)
				}
			}
		}
	}
}

func (s *Server) broadcastServerShutdown() {
	shutdownMsg := []byte("Server is shutting down. Disconnecting.")
	for c := range s.Clients {
		c.Socket.SetWriteDeadline(time.Now().Add(time.Second * 1))
		err := c.Socket.WriteMessage(websocket.TextMessage, shutdownMsg)
		if err != nil {
			log.Printf("write shutdown message error: %v", err)
		}
		c.Socket.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	}
}

func StartServer(port string) {
	server := NewServer()
	go server.Run()
	addr := ":" + port
	mux := http.NewServeMux()
	mux.HandleFunc("/{username}", server.socketHandler)
	httpServer := &http.Server{Addr: addr, Handler: mux}

	// 监听中断信号来优雅关闭
	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
		<-quit

		log.Println("Shutting down server...")

		// 创建一个有超时的 context
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// 通知所有客户端服务器将要关闭
		server.broadcastServerShutdown()

		if err := httpServer.Shutdown(ctx); err != nil {
			log.Fatalf("server shutdown error: %v", err)
		}
	}()

	log.Printf("start server on %v", port)
	if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("listen %s error: %v", port, err)
	}
	log.Println("server exiting")
}
