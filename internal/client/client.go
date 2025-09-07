package client

import (
	"bufio"
	"context"
	"fmt"
	"github.com/gorilla/websocket"
	"log"
	"os"
	"os/signal"
	"sync"
	"time"
)

type Client struct {
	Socket   *websocket.Conn
	Username string
}

func StartClient(port, username string) {
	serverUrl := fmt.Sprintf("ws://localhost:%s/%s", port, username)

	conn, _, err := websocket.DefaultDialer.Dial(serverUrl, nil)
	if err != nil {
		log.Fatalf("dial error: %v", err)
	}
	log.Println("Connected to server")

	ctx, cancel := context.WithCancel(context.Background())
	var once sync.Once

	// 保证conn只会被关闭一次
	cleanup := func(reason string) {
		once.Do(func() {
			log.Println("cleanup", reason)
			_ = conn.WriteControl(
				websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
				time.Now().Add(time.Second))
			conn.Close()
			cancel()
			os.Exit(0)
		})
	}

	// 处理Ctrl+C退出
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt)
		<-c
		cleanup("interrupt")
	}()

	// 读数据
	go func() {
		defer cleanup("read exit")
		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				return
			}
			log.Printf("recv: %s\n", message)
		}
	}()

	// 写数据
	go func() {
		defer cleanup("write exit")
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			select {
			case <-ctx.Done():
				return
			default:
			}
			if err := conn.WriteMessage(websocket.TextMessage, []byte(scanner.Text())); err != nil {
				return
			}
		}
	}()

	<-ctx.Done()
}
