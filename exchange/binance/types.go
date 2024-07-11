package binance

import (
	"log/slog"
	"sync"
	"time"

	"github.com/dafsic/hunter/pkg/ws"
)

type BinanceWsClient struct {
	client ws.Client
	stopC  chan struct{}
	rLock  sync.RWMutex
}

func (e *BinanceSpotExchange) NewWsClient(url, localIP string, callback ws.CallbackFunc, isGoroutine bool) (*BinanceWsClient, error) {
	client, err := e.wsManager.NewClient(url, localIP, callback, e.wsHeartBeat, isGoroutine)
	if err != nil {
		return nil, err
	}

	c := &BinanceWsClient{
		client: client,
		stopC:  make(chan struct{}),
		rLock:  sync.RWMutex{},
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
					client, err = e.wsManager.NewClient(url, localIP, callback, e.wsHeartBeat, isGoroutine)
					if err != nil {
						e.logger.Log(slog.LevelWarn, "websocket reconnected fail", "url", url, "error", err)
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

func (c *BinanceWsClient) Close() {
	c.stopC <- struct{}{}
	c.client.Close() // 等待websket连接关闭。要退出了，就不用加锁了，最保险就加一个waitgroup，等待处理断线重连程序结束
	close(c.stopC)
}

func (c *BinanceWsClient) SendMessage(msg []byte) error {
	c.rLock.RLock()
	defer c.rLock.RUnlock()

	return c.client.SendMessage(msg)
}
