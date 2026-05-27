package main

import (
	"flag"
	"log"
	"os"

	"github.com/openmemind/memind-go/engine"
	"github.com/openmemind/memind-go/server"
	"github.com/openmemind/memind-go/store"
)

func main() {
	addr := flag.String("addr", ":8080", "server listen address")
	flag.Parse()

	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Printf("memind-go starting...")

	memory := engine.Builder().
		Store(store.NewInMemoryStore()).
		Build()

	if addrEnv := os.Getenv("MEMIND_ADDR"); addrEnv != "" {
		*addr = addrEnv
	}

	srv := server.New(memory, *addr)
	log.Printf("listening on %s", *addr)
	if err := srv.Start(); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
