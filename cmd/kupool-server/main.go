package main

import (
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/JellyTony/kupool/app/server"
	"github.com/JellyTony/kupool/logger"
	"github.com/JellyTony/kupool/mq"
	"github.com/JellyTony/kupool/stats"
)

func main() {
	addr := flag.String("addr", ":8080", "listen addr")
    interval := flag.Duration("interval", 30*time.Second, "nonce update interval")
    expire := flag.Duration("expire", 0, "task expire duration (0=disabled)")
	storeKind := flag.String("store", "memory", "store backend: memory|pg")
	pgDsn := flag.String("pg_dsn", "", "postgres dsn")
	mqKind := flag.String("mq", "memory", "mq backend: memory|rabbit")
	mqUrl := flag.String("mq_url", "amqp://guest:guest@localhost:5672/", "rabbitmq url")
	mqQueue := flag.String("mq_queue", "kupool_submissions", "rabbitmq queue name")
	flag.Parse()
	if v := os.Getenv("KUP_ADDR"); v != "" {
		*addr = v
	}
    if v := os.Getenv("KUP_INTERVAL"); v != "" {
        if d, err := time.ParseDuration(v); err == nil {
            *interval = d
        }
    }
    if v := os.Getenv("KUP_EXPIRE"); v != "" {
        if d, err := time.ParseDuration(v); err == nil { *expire = d }
    }
	if v := os.Getenv("KUP_STORE"); v != "" {
		*storeKind = v
	}
	if v := os.Getenv("KUP_PG_DSN"); v != "" {
		*pgDsn = v
	}
	if v := os.Getenv("KUP_MQ"); v != "" {
		*mqKind = v
	}
	if v := os.Getenv("KUP_MQ_URL"); v != "" {
		*mqUrl = v
	}
	if v := os.Getenv("KUP_MQ_QUEUE"); v != "" {
		*mqQueue = v
	}
	if err := logger.Init(logger.Settings{Format: "json", Level: os.Getenv("KUP_LOG_LEVEL")}); err != nil {
		logger.WithError(err).Fatal("logger init failed")
	}
	var store server.StatsStore
	if *storeKind == "pg" {
		pg, err := stats.NewPGStore(*pgDsn)
		if err != nil {
			logger.WithError(err).Fatal("pg init failed")
		}
		store = pg
	} else {
		store = stats.NewMemoryStore()
	}
	var queue server.MessageQueue
	if *mqKind == "rabbit" {
		r, err := mq.NewRabbitMQ(*mqUrl, *mqQueue)
		if err != nil {
			logger.WithError(err).Fatal("rabbitmq init failed")
		}
		queue = r
	} else {
		queue = mq.NewMemoryQueue(1024)
	}
    app := server.NewAppServer(*addr, store, queue, *interval, *expire)
	go func() { _ = app.Start() }()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	_ = app.Shutdown()
}
