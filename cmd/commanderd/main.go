// commanderd is a standalone gRPC server that exposes the Commander service.
//
// SECURITY WARNING: Commander executes arbitrary shell commands. Do not expose
// this service to a network without a sandbox. Treat it as remote code execution.
package main

import (
	"flag"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/accretional/proto-cmd/commander"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:50551", "listen address (host:port)")
	flag.Parse()

	lis, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatalf("listen %s: %v", *addr, err)
	}

	srv := grpc.NewServer()
	commander.RegisterCommanderServer(srv, commander.NewCommanderServer())
	reflection.Register(srv)

	log.Printf("commanderd listening on %s", lis.Addr())

	// Graceful shutdown on SIGINT/SIGTERM.
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Printf("commanderd: shutting down")
		srv.GracefulStop()
	}()

	if err := srv.Serve(lis); err != nil {
		log.Fatalf("serve: %v", err)
	}
}
