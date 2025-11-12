package main

import (
    "flag"

    clientapp "github.com/JellyTony/kupool/client/app"
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
    _ = c.Run()
}
