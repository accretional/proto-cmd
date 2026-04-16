package commander

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type commanderServer struct {
	UnimplementedCommanderServer
}

// NewCommanderServer returns a Commander server implementation that executes
// shell commands and streams their stdout/stderr to clients. The returned
// server is stateless and safe for concurrent use.
func NewCommanderServer() CommanderServer {
	return &commanderServer{}
}

// Shell runs a command and streams its output. When cmd.Args is empty the
// command string is executed via cmd.Shell (default /bin/sh -c); when Args is
// set, cmd.Command is exec'd directly with those args and cmd.Shell is ignored.
//
// Non-zero exit is NOT surfaced as a gRPC error — the server writes a final
// "exit status N" chunk to stderr and closes the stream cleanly. Clients must
// inspect the stream for that marker rather than rely on status.FromError.
//
// Cancellation comes from the stream context or cmd.TimeoutSeconds (whichever
// fires first); both kill the child via exec.CommandContext.
func (s *commanderServer) Shell(cmd *Command, stream grpc.ServerStreamingServer[Output]) error {
	if cmd.Command == "" {
		return status.Error(codes.InvalidArgument, "command cannot be empty")
	}

	shell := cmd.Shell
	if shell == "" {
		shell = "/bin/sh"
	}

	workDir := cmd.WorkingDir
	if workDir == "" {
		var err error
		workDir, err = os.Getwd()
		if err != nil {
			return status.Errorf(codes.Internal, "getwd: %v", err)
		}
	}

	ctx := stream.Context()
	if cmd.TimeoutSeconds > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(cmd.TimeoutSeconds)*time.Second)
		defer cancel()
	}

	// If args are provided, run the command directly; otherwise use a shell.
	var execCmd *exec.Cmd
	if len(cmd.Args) > 0 {
		execCmd = exec.CommandContext(ctx, cmd.Command, cmd.Args...)
	} else {
		execCmd = exec.CommandContext(ctx, shell, "-c", cmd.Command)
	}

	execCmd.Dir = workDir

	if len(cmd.Env) > 0 {
		env := os.Environ()
		for key, value := range cmd.Env {
			env = append(env, fmt.Sprintf("%s=%s", key, value))
		}
		execCmd.Env = env
	}

	stdoutPipe, err := execCmd.StdoutPipe()
	if err != nil {
		return status.Errorf(codes.Internal, "stdout pipe: %v", err)
	}
	stderrPipe, err := execCmd.StderrPipe()
	if err != nil {
		return status.Errorf(codes.Internal, "stderr pipe: %v", err)
	}

	if err := execCmd.Start(); err != nil {
		return status.Errorf(codes.Internal, "start: %v", err)
	}

	return s.streamOutput(execCmd, stdoutPipe, stderrPipe, stream)
}

func (s *commanderServer) streamOutput(
	cmd *exec.Cmd,
	stdoutPipe, stderrPipe io.ReadCloser,
	stream grpc.ServerStreamingServer[Output],
) error {
	type chunk struct {
		stdout bool
		data   []byte
	}
	ch := make(chan chunk, 16)
	done := make(chan struct{}, 2)

	readPipe := func(r io.Reader, isStdout bool) {
		defer func() { done <- struct{}{} }()
		buf := make([]byte, 4096)
		for {
			n, err := r.Read(buf)
			if n > 0 {
				data := make([]byte, n)
				copy(data, buf[:n])
				ch <- chunk{stdout: isStdout, data: data}
			}
			if err != nil {
				return
			}
		}
	}

	go readPipe(stdoutPipe, true)
	go readPipe(stderrPipe, false)

	// Wait for both reader goroutines to finish, then close the channel.
	go func() {
		<-done
		<-done
		close(ch)
	}()

	// Single goroutine drains the channel and sends on the stream.
	// This ensures no concurrent stream.Send calls.
	sendDone := make(chan error, 1)
	go func() {
		for c := range ch {
			if err := stream.Send(&Output{Stdout: c.stdout, Data: c.data}); err != nil {
				sendDone <- err
				return
			}
		}
		sendDone <- nil
	}()

	waitErr := cmd.Wait()
	sendErr := <-sendDone

	if sendErr != nil {
		return status.Errorf(codes.Internal, "stream: %v", sendErr)
	}
	if waitErr != nil {
		// Non-zero exit is not a server error — send exit info as
		// a final stderr chunk so the client sees it.
		if exitErr, ok := waitErr.(*exec.ExitError); ok {
			msg := fmt.Sprintf("exit status %d", exitErr.ExitCode())
			if err := stream.Send(&Output{Stdout: false, Data: []byte(msg)}); err != nil {
				return status.Errorf(codes.Internal, "stream: %v", err)
			}
			return nil
		}
		return status.Errorf(codes.Internal, "wait: %v", waitErr)
	}
	return nil
}
