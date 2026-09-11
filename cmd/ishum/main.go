package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"ishum/internal/server"
	"ishum/internal/watch"
)

func main() {
	addr := flag.String("addr", env("ISHUM_ADDR", "127.0.0.1:8090"), "listen address")
	data := flag.String("data", env("ISHUM_DATA", "data"), "data directory")
	flag.Parse()

	s, err := server.New(*addr, *data)
	if err != nil {
		log.Fatal(err)
	}
	stop := make(chan struct{})
	go watch.Loop(stop)
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM)
		<-c
		close(stop)
		os.Exit(0)
	}()
	log.Printf("Ishum %s  data=%s  sequenced on Kaspa", *addr, *data)
	log.Fatal(http.ListenAndServe(*addr, s.Handler()))
}

func env(k, fallback string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return fallback
}
