package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	clientapp "github.com/JellyTony/kupool/app/client"
	"github.com/JellyTony/kupool/logger"
)

func main() {
	addr := flag.String("addr", "localhost:8080", "server addr")
	username := flag.String("username", "admin", "username")
	flag.Parse()
	_ = logger.Init(logger.Settings{Format: "json"})
	c := clientapp.NewClient(*username)
	if err := c.Connect(*addr); err != nil {
		logger.WithError(err).Fatal("connect failed")
	}

	ctx, cancel := context.WithCancelCause(context.Background())

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel(fmt.Errorf("signal done"))
		c.Close()
	}()
	_ = c.Run(ctx)
}
