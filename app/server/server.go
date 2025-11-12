package server

import (
	"context"
	"time"

	kupool "github.com/JellyTony/kupool"
	"github.com/JellyTony/kupool/tcp"
)

type AppServer struct {
    srv   kupool.Server
    coord *Coordinator
}

func NewAppServer(addr string, store StatsStore, mq MessageQueue, interval time.Duration) *AppServer {
	s := tcp.NewServer(addr)
	coord := NewCoordinator(s, store, mq, interval, 0)
	acc := NewAcceptor(coord)
	lst := NewListener(coord)
	st := NewState(coord)
	s.SetAcceptor(acc)
	s.SetMessageListener(lst)
	s.SetStateListener(st)
	return &AppServer{srv: s, coord: coord}
}

func (a *AppServer) Start() error {
    go func() {
        ch := a.coord.mq.Subscribe()
        for evt := range ch {
            _ = a.coord.store.Increment(evt.Username, evt.Time)
        }
    }()
    a.coord.StartBroadcast()
    return a.srv.Start()
}

func (a *AppServer) Shutdown() error {
    a.coord.Stop()
    _ = a.coord.mq.Close()
    return a.srv.Shutdown(context.Background())
}
