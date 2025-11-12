package server

import (
	"context"
	"sync"
	"time"

	kupool "github.com/JellyTony/kupool"
	"github.com/JellyTony/kupool/logger"
	"github.com/JellyTony/kupool/tcp"
)

type ShutdownStatus struct {
	StartAt      time.Time
	EndAt        time.Time
	MQStopped    bool
	MQPending    int
	MQErrors     int
	StoreClosed  bool
	ServerClosed bool
	Duration     time.Duration
}

type AppServer struct {
	srv         kupool.Server
	coord       *Coordinator
	stopConsume chan struct{}
	mqWG        sync.WaitGroup
	status      ShutdownStatus
}

func NewAppServer(addr string, store StatsStore, state StateStore, mq MessageQueue, interval time.Duration, expire time.Duration, historyWindow time.Duration) *AppServer {
    s := tcp.NewServer(addr)
    coord := NewCoordinator(s, store, state, mq, interval, expire, historyWindow)
    acc := NewAcceptor(coord)
    lst := NewListener(coord)
    st := NewState(coord)
    s.SetAcceptor(acc)
    s.SetMessageListener(lst)
    s.SetStateListener(st)
    return &AppServer{srv: s, coord: coord, stopConsume: make(chan struct{})}
}

func (a *AppServer) Start(ctx context.Context) error {
	go func() {
		ch := a.coord.mq.Subscribe()
		for {
			select {
			case <-ctx.Done():
				return
			case <-a.stopConsume:
				return
			case evt, ok := <-ch:
				if !ok {
					return
				}
				a.mqWG.Add(1)
				if err := a.coord.store.Increment(evt.Username, evt.Time); err != nil {
					logger.WithFields(logger.Fields{"module": "server", "username": evt.Username, "time": evt.Time}).Errorf("store increment failed: %v", err)
					a.status.MQErrors++
				}
				a.mqWG.Done()
			}
		}
	}()
	a.coord.StartBroadcast()
	return a.srv.Start()
}

func (a *AppServer) Shutdown(ctx context.Context) error {
	a.status.StartAt = time.Now()
	close(a.stopConsume)
	a.status.MQStopped = true
	done := make(chan struct{})
	go func() { a.mqWG.Wait(); close(done) }()
	wait := 10 * time.Second
	timeout := 30 * time.Second
	select {
	case <-done:
	case <-time.After(wait):
	}
	a.coord.Stop()
	if err := retry(3, 2*time.Second, func() error { return a.coord.mq.Close() }); err != nil {
		logger.WithError(err).Error("mq close failed")
	}
	if err := retry(3, 2*time.Second, func() error { return a.coord.store.Close() }); err != nil {
		logger.WithError(err).Error("store close failed")
	} else {
		a.status.StoreClosed = true
	}
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if err := a.srv.Shutdown(cctx); err != nil {
		logger.WithError(err).Error("server shutdown failed")
	} else {
		a.status.ServerClosed = true
	}
	a.status.EndAt = time.Now()
	a.status.Duration = a.status.EndAt.Sub(a.status.StartAt)
	logger.WithFields(logger.Fields{"module": "server", "duration": a.status.Duration}).Info("shutdown done")
	return nil
}

func retry(n int, backoff time.Duration, f func() error) error {
	var err error
	for i := 0; i < n; i++ {
		if err = f(); err == nil {
			return nil
		}
		time.Sleep(backoff)
	}
	return err
}

func (a *AppServer) Status() ShutdownStatus { return a.status }
