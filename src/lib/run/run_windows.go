package run

import (
	"context"
	"fmt"
	"os"
	"os/exec"
)

// start starts the command. It calls cmd.Start() on behalf of the requested
// command, it doesn't wait for the process and sets the process' pid in the
// [Result]'s Pid field.
func start(ctx context.Context, opts Options) (*Result, error) {
	var cmd *exec.Cmd

	// If we are running on detach mode we can't use the passed down context as
	// it would kill the child process if the context is canceled.
	if opts.ExecMode == ExecModeDetach {
		cmd = exec.Command(opts.Name, opts.Args...)
	} else {
		cmd = exec.CommandContext(ctx, opts.Name, opts.Args...)
	}

	cmd.Dir = opts.Dir
	if opts.InheritEnv {
		cmd.Env = os.Environ()
	}

	if err := writeToStdin(cmd, ""); err != nil {
		return nil, fmt.Errorf("failed to write input in start: %v", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	return &Result{OutputType: OutputNone, Pid: cmd.Process.Pid}, nil
}
