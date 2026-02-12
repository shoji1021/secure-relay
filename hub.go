package main

// Hub は接続しているクライアントを管理し、メッセージを分配します
type Hub struct {
	// 登録されているクライアントのリスト (メモリ上のみ)
	clients map[*Client]bool

	// メッセージ転送用チャンネル (データが通る土管)
	broadcast chan []byte

	// クライアントの登録・削除リクエスト
	register   chan *Client
	unregister chan *Client
}

func NewHub() *Hub {
	return &Hub{
		broadcast:  make(chan []byte),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[*Client]bool),
	}
}

// Run はサーバーが起動している間、永遠に回り続けるループです
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			// 新しいデバイスが接続
			h.clients[client] = true

		case client := <-h.unregister:
			// デバイスが切断
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}

		case message := <-h.broadcast:
			// 【重要】データを受信したら、即座に全クライアントへ転送
			// ここでデータベース保存処理を書かない限り、ディスクには残りません
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
			// ここを抜けると message 変数はメモリから破棄されます
		}
	}
}