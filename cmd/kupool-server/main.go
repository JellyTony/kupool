package main

import (
    "context"
    "encoding/json"
    "flag"
    "net/http"
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
		if d, err := time.ParseDuration(v); err == nil {
			*expire = d
		}
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
    historyWindow := 24 * time.Hour
    if v := os.Getenv("KUP_HISTORY_WINDOW"); v != "" {
        if d, err := time.ParseDuration(v); err == nil { historyWindow = d }
    }
    // state store 与 stats store共享（PG 模式）；内存模式用同一个 MemoryStore 实现两者接口
    var state server.StateStore
    if *storeKind == "pg" {
        state = store.(server.StateStore)
    } else {
        state = store.(server.StateStore)
    }
    app := server.NewAppServer(*addr, store, state, queue, *interval, *expire, historyWindow)
    rootCtx, rootCancel := context.WithCancel(context.Background())
    go func() { _ = app.Start(rootCtx) }()

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
    mux.HandleFunc("/stats", func(w http.ResponseWriter, r *http.Request) {
		u := r.URL.Query().Get("username")
		ms := r.URL.Query().Get("minute")
		m := time.Now()
		if ms != "" {
			if t, err := time.Parse(time.RFC3339, ms); err == nil {
				m = t
			}
		}
		cnt, err := store.Get(u, m)
		if err != nil {
			w.WriteHeader(500)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"username": u, "minute": m.Truncate(time.Minute).Format(time.RFC3339), "submission_count": cnt})
    })
    mux.HandleFunc("/shutdown/status", func(w http.ResponseWriter, r *http.Request){
        st := app.Status()
        _ = json.NewEncoder(w).Encode(map[string]any{
            "start_at": st.StartAt.Format(time.RFC3339),
            "end_at": st.EndAt.Format(time.RFC3339),
            "mq_stopped": st.MQStopped,
            "store_closed": st.StoreClosed,
            "server_closed": st.ServerClosed,
            "duration": st.Duration.String(),
        })
    })
	go func() { _ = http.ListenAndServe(":8081", mux) }()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
    <-sigCh
    rootCancel()
    _ = app.Shutdown(rootCtx)
}
