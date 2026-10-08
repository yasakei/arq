package runtime

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"sync"
	"time"
)

type ProcessHandle struct {
	cmd      *exec.Cmd
	done     chan struct{}
	mu       sync.Mutex
	err      error
	started  bool
	timedOut bool
	cancel   context.CancelFunc
}

type Process = ProcessHandle

func (p *ProcessHandle) setResult(err error) {
	p.mu.Lock()
	p.err = err
	p.mu.Unlock()
	close(p.done)
}

func (p *ProcessHandle) Wait() error {
	if p == nil {
		return fmt.Errorf("nil process handle")
	}
	<-p.done
	p.mu.Lock()
	err := p.err
	p.mu.Unlock()
	return err
}

func (p *ProcessHandle) Done() <-chan struct{} {
	if p == nil {
		return nil
	}
	return p.done
}

func (p *ProcessHandle) Kill() error {
	if p == nil || p.cmd == nil || p.cmd.Process == nil {
		return fmt.Errorf("process is not running")
	}
	return p.cmd.Process.Kill()
}

func (p *ProcessHandle) ExitCode() int {
	if p == nil || p.cmd == nil || p.cmd.ProcessState == nil {
		return -1
	}
	return p.cmd.ProcessState.ExitCode()
}

func (r *Runner) spawn(args []any) (any, error) {
	return r.spawnAt(args, r.Root)
}

func (r *Runner) spawnAt(args []any, dir string) (any, error) {
	timeout := 0.0
	commandArgs := args
	if len(args) == 2 {
		if seconds, ok := args[1].(float64); ok {
			timeout = seconds
			commandArgs = args[:1]
		}
	}
	if len(args) >= 3 {
		if seconds, ok := args[len(args)-1].(float64); ok {
			timeout = seconds
			commandArgs = args[:len(args)-1]
		}
	}
	parts, shell, err := commandParts(commandArgs)
	if err != nil {
		return nil, err
	}
	ctx := context.Background()
	var cancel context.CancelFunc
	if timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, time.Duration(timeout*float64(time.Second)))
	}
	var cmd *exec.Cmd
	if shell {
		if cancel != nil {
			cmd = commandContextShell(ctx, parts[0])
		} else {
			cmd = shellCommand(parts[0])
		}
	} else {
		if cancel != nil {
			cmd = exec.CommandContext(ctx, parts[0], parts[1:]...)
		} else {
			cmd = exec.Command(parts[0], parts[1:]...)
		}
	}
	cmd.Dir, cmd.Stdout, cmd.Stderr = dir, r.Out, r.Err
	handle := &ProcessHandle{cmd: cmd, done: make(chan struct{}), cancel: cancel}
	if err := cmd.Start(); err != nil {
		if cancel != nil {
			cancel()
		}
		handle.setResult(err)
		return handle, nil
	}
	handle.started = true
	go func() {
		err := cmd.Wait()
		if ctx.Err() == context.DeadlineExceeded {
			handle.mu.Lock()
			handle.timedOut = true
			handle.mu.Unlock()
			err = fmt.Errorf("process timed out: %w", context.DeadlineExceeded)
		}
		if cancel != nil {
			cancel()
		}
		handle.setResult(err)
	}()
	return handle, nil
}

func commandContextShell(ctx context.Context, command string) *exec.Cmd {
	if runtime.GOOS == "windows" {
		return exec.CommandContext(ctx, "cmd", "/C", command)
	}
	return exec.CommandContext(ctx, "sh", "-c", command)
}

func waitProcess(args []any) (any, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("wait needs a process handle")
	}
	handle, ok := args[0].(*ProcessHandle)
	if !ok {
		return nil, fmt.Errorf("wait needs a process handle")
	}
	if len(args) > 1 {
		seconds, ok := args[1].(float64)
		if !ok || seconds < 0 {
			return nil, fmt.Errorf("wait timeout must be a non-negative number")
		}
		select {
		case <-handle.Done():
		case <-time.After(time.Duration(seconds * float64(time.Second))):
			return nil, fmt.Errorf("process wait timed out")
		}
	}
	if err := handle.Wait(); err != nil {
		return nil, err
	}
	return float64(handle.ExitCode()), nil
}

func killProcess(args []any) (any, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("kill needs a process handle")
	}
	handle, ok := args[0].(*ProcessHandle)
	if !ok {
		return nil, fmt.Errorf("kill needs a process handle")
	}
	if err := handle.Kill(); err != nil {
		return nil, err
	}
	return nil, nil
}

func (r *Runner) timeout(args []any) (any, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("timeout needs a command and seconds")
	}
	commandArgs := []any{args[0], args[1]}
	handle, err := r.spawn(commandArgs)
	if err != nil {
		return nil, err
	}
	return waitProcess([]any{handle})
}
