package main

import (
	"log"
	"net/http"
	"time"
	

	"github.com/gorilla/websocket"
)

const (
	// 書き込み待ち時間
	writeWait = 10 * time.Second
	// 最大メッセージサイズ (例: 10MB)
	maxMessageSize = 10 * 1024 * 1024
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// 開発中はすべての接続元を許可（本番では厳密にします）
	CheckOrigin: func(r *http.Request) bool { return true },
}

// Client は個々の接続ユーザーを表します
type Client struct {
	hub *Hub
	// WebSocket接続
	conn *websocket.Conn
	// 送信メッセージの一時バッファ
	send chan []byte
}

// readPump はクライアントからデータを受け取りHubへ送ります
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()
	c.conn.SetReadLimit(maxMessageSize)

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			break
		}
		// 受信したデータをHubのbroadcastへ流す
		c.hub.broadcast <- message
	}
}

// writePump はHubから来たデータをクライアントへ送ります
func (c *Client) writePump() {
	ticker := time.NewTicker(50 * time.Second) // Ping用
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// Hubがチャンネルを閉じた場合
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.BinaryMessage)
			if err != nil {
				return
			}
			w.Write(message)

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			// 接続維持のためのPing
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// serveWs はHTTPリクエストをWebSocket通信にアップグレードします
func serveWs(hub *Hub, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	client := &Client{hub: hub, conn: conn, send: make(chan []byte, 256)}
	client.hub.register <- client

	// 読み書きを同時に行うため、Goルーチン（並行処理）を2つ起動
	go client.writePump()
	go client.readPump()
}