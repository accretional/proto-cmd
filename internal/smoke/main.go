// smoke is a tiny Commander client used by LET_IT_RIP.sh to exercise a
// running commanderd. It is not part of the public surface.
package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/accretional/proto-cmd/commander"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("usage: smoke <addr>")
	}
	addr := os.Args[1]

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	client := commander.NewCommanderClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	stream, err := client.Shell(ctx, &commander.Command{
		Command: "echo smoke-test && echo oops 1>&2",
	})
	if err != nil {
		log.Fatalf("Shell: %v", err)
	}

	gotStdout, gotStderr := false, false
	for {
		msg, rerr := stream.Recv()
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			log.Fatalf("recv: %v", rerr)
		}
		if msg.Stdout {
			fmt.Printf("stdout: %s", msg.Data)
			gotStdout = true
		} else {
			fmt.Printf("stderr: %s", msg.Data)
			gotStderr = true
		}
	}
	if !gotStdout || !gotStderr {
		log.Fatalf("expected both stdout and stderr (stdout=%v stderr=%v)", gotStdout, gotStderr)
	}
	fmt.Println("smoke test ok")
}
