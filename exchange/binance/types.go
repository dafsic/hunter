package binance

import (
	"sync"
	"time"

	"github.com/dafsic/hunter/pkg/log"
	"github.com/dafsic/hunter/pkg/ws"
)

type WsClient struct {
	client ws.Client
	stopC  chan struct{}
	rLock  sync.RWMutex
	logger log.Logger
}

func NewWsClientWithReconnect(l log.Logger, mgr ws.Manager, url, localIP string, callback ws.CallbackFunc, heartBeat int, isGoroutine bool) (*WsClient, error) {
	client, err := mgr.NewClient(url, localIP, callback, heartBeat, isGoroutine)
	if err != nil {
		return nil, err
	}

	c := &WsClient{
		client: client,
		stopC:  make(chan struct{}),
		rLock:  sync.RWMutex{},
		logger: l,
	}

	// 处理断线重连
	go func() {
		var (
			disconnectC <-chan struct{}
			err         error
		)
		disconnectC = c.client.IsDisconnect()
		for {
			select {
			case <-c.stopC:
				return
			case <-disconnectC:
				c.rLock.Lock()
				for {
					client, err = mgr.NewClient(url, localIP, callback, heartBeat, isGoroutine)
					if err != nil {
						c.logger.Warn("websocket reconnected fail", "url", url, "error", err)
						time.Sleep(1 * time.Second)
						continue
					}
					break
				}
				disconnectC = client.IsDisconnect()
				c.rLock.Unlock()
			}
		}
	}()

	return c, nil
}

func (c *WsClient) Close() {
	c.stopC <- struct{}{}
	c.client.Close() // 等待websket连接关闭。要退出了，就不用加锁了，最保险就加一个waitgroup，等待处理断线重连程序结束
	close(c.stopC)
}

func (c *WsClient) SendMessage(msg []byte) error {
	c.rLock.RLock()
	defer c.rLock.RUnlock()

	return c.client.SendMessage(msg)
}
