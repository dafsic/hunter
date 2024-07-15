package ws

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/dafsic/hunter/pkg/log"
	"github.com/gorilla/websocket"
)

type Client interface {
	Close()
	IsDisconnect() <-chan struct{}
	SendMessage(msg []byte) error
}

type wsClient struct {
	ctx         context.Context
	conn        *websocket.Conn
	urlString   string
	proxy       string
	localIP     string
	disconnectC chan struct{}
	pingWait    time.Duration
	writeWait   time.Duration
	callback    CallbackFunc
	cancelFunc  context.CancelFunc
	isGoroutine bool
	wg          sync.WaitGroup
	l           log.Logger
}

func (m *WsManager) NewClient(url, localIP string, cb CallbackFunc, heartbeat int, isGoroutine bool) (Client, error) {
	var err error
	ctx, cancelFunc := context.WithCancel(context.Background())
	ws := &wsClient{
		urlString:   url,
		localIP:     localIP,
		pingWait:    time.Duration(heartbeat) * time.Second,
		disconnectC: make(chan struct{}, 8),
		callback:    cb,
		isGoroutine: isGoroutine,
		l:           m.logger,
		proxy:       m.proxy,
		writeWait:   m.writeWait,
		ctx:         ctx,
		cancelFunc:  cancelFunc,
	}

	ws.conn, err = ws.connect()
	if err != nil {
		return nil, err
	}

	ws.wg.Add(2)
	go ws.keepalibe()
	go ws.readLoop()

	return ws, nil
}

// SendMessage 向服务端发送消息
func (ws *wsClient) SendMessage(msg []byte) error {
	_ = ws.conn.SetWriteDeadline(time.Now().Add(ws.writeWait))
	return ws.conn.WriteMessage(websocket.TextMessage, msg)
}

// IsColsed 返回一个只读channel，如果能读取到值，代表已经关闭了
func (ws *wsClient) IsDisconnect() <-chan struct{} {
	return ws.disconnectC
}

// close 关闭websocket客户端
func (ws *wsClient) Close() {
	ws.cancelFunc()
	close(ws.disconnectC)
	ws.wg.Wait()
	_ = ws.conn.WriteControl(websocket.CloseMessage, []byte{}, time.Now().Add(ws.writeWait))
	_ = ws.conn.Close()
	ws.l.Info("closed ws", "url", ws.urlString)
}

// connect 连接WebSocket服务器
func (ws *wsClient) connect() (*websocket.Conn, error) {
	dialer := websocket.Dialer{
		Proxy:           http.ProxyFromEnvironment,
		ReadBufferSize:  16777216,
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	// 指定本地ip地址
	if ws.localIP != "" {
		dialer.NetDial = func(network, addr string) (net.Conn, error) {
			lAddr, err := net.ResolveTCPAddr(network, ws.localIP+":0")
			if err != nil {
				return nil, err
			}
			dialer := net.Dialer{
				LocalAddr: lAddr,
			}
			return dialer.Dial(network, addr)
		}
	}

	// 是否设置代理
	if ws.proxy != "" {
		proxyURL, err := url.Parse(ws.proxy)
		if err != nil {
			return nil, err
		}
		dialer.Proxy = http.ProxyURL(proxyURL)
	}

	conn, _, err := dialer.Dial(ws.urlString, nil)
	if err != nil {
		return conn, err
	}

	conn.SetCloseHandler(func(code int, text string) error {
		message := websocket.FormatCloseMessage(code, "")
		conn.WriteControl(websocket.CloseMessage, message, time.Now().Add(ws.writeWait))
		ws.disconnectC <- struct{}{}
		return nil
	})

	conn.SetPingHandler(func(message string) error {
		err := ws.conn.WriteControl(websocket.PongMessage, []byte(message), time.Now().Add(ws.writeWait))
		if err != nil {
			return err
		}
		return nil
	})

	return conn, nil
}

// keepalibe 保活
func (ws *wsClient) keepalibe() {
	defer ws.wg.Done()
	ticker := time.NewTicker(ws.pingWait)
	defer ticker.Stop()
	var (
		err error
	)

	for {
		select {
		case <-ws.ctx.Done():
			return
		case <-ticker.C:
			err = ws.conn.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(10*time.Second))
			if err != nil {
				ws.l.Warn("send ping error", "error", err)
				ws.disconnectC <- struct{}{}
			}
		}
	}
}

func (ws *wsClient) readLoop() {
	defer ws.wg.Done()
	for {
		select {
		case <-ws.ctx.Done():
			return
		default:
			_, message, err := ws.conn.ReadMessage()
			if err != nil {
				ws.l.Error("read ws message error", "message", string(message), "error", err)
				ws.disconnectC <- struct{}{}
			}

			if ws.isGoroutine {
				go ws.callback(message)
			} else {
				ws.callback(message)
			}
		}
	}
}
