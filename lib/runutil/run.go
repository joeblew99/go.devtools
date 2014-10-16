// Package runutil provides functions for running commands and
// functions and logging their outcome.
package runutil

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"tools/lib/envutil"
)

const (
	prefix = ">>"
)

type Run struct {
	indent  int
	Stdout  io.Writer
	Verbose bool
}

func New(verbose bool, stdout io.Writer) *Run {
	return &Run{
		indent:  0,
		Stdout:  stdout,
		Verbose: verbose,
	}
}

// Command runs the given command and logs its outcome using the default verbosity.
func (r *Run) Command(stdout, stderr io.Writer, env map[string]string, path string, args ...string) error {
	return r.CommandWithVerbosity(r.Verbose, stdout, stderr, env, path, args...)
}

// CommandWithVerbosity runs the given command and logs its outcome using the given verbosity.
func (r *Run) CommandWithVerbosity(verbose bool, stdout, stderr io.Writer, env map[string]string, path string, args ...string) error {
	r.increaseIndent()
	defer r.decreaseIndent()
	command := exec.Command(path, args...)
	command.Stdin = os.Stdin
	command.Stdout = stdout
	command.Stderr = stderr
	command.Env = envutil.ToSlice(env)
	if verbose {
		r.printf(r.Stdout, strings.Join(command.Args, " "))
	}
	var err error
	if err = command.Run(); err != nil {
		if verbose {
			r.printf(r.Stdout, "FAILED")
		}
	} else {
		if verbose {
			r.printf(r.Stdout, "OK")
		}
	}
	return err
}

// Function runs the given function and logs its outcome using the
// default verbosity.
func (r *Run) Function(fn func() error, format string, args ...interface{}) error {
	return r.FunctionWithVerbosity(r.Verbose, fn, format, args...)
}

// FunctionWithVerbosity runs the given function and logs its outcome
// using the given verbosity.
func (r *Run) FunctionWithVerbosity(verbose bool, fn func() error, format string, args ...interface{}) error {
	r.increaseIndent()
	defer r.decreaseIndent()
	if verbose {
		r.printf(r.Stdout, format, args...)
	}
	err := fn()
	if err != nil {
		if verbose {
			r.printf(r.Stdout, "FAILED")
		}
	} else {
		if verbose {
			r.printf(r.Stdout, "OK")
		}
	}
	return err
}

// Output logs the given list of lines using the default verbosity.
func (r *Run) Output(output []string) {
	r.OutputWithVerbosity(r.Verbose, output)
}

// OutputWithVerbosity logs the given list of lines using the given
// verbosity.
func (r *Run) OutputWithVerbosity(verbose bool, output []string) {
	if verbose {
		for _, line := range output {
			r.logLine(line)
		}
	}
}

func (r *Run) decreaseIndent() {
	r.indent--
}

func (r *Run) increaseIndent() {
	r.indent++
}

func (r *Run) logLine(line string) {
	if !strings.HasPrefix(line, prefix) {
		r.increaseIndent()
		defer r.decreaseIndent()
	}
	r.printf(r.Stdout, "%v", line)
}

func (r *Run) printf(stdout io.Writer, format string, args ...interface{}) {
	args = append([]interface{}{strings.Repeat(prefix, r.indent)}, args...)
	fmt.Fprintf(stdout, "%v "+format+"\n", args...)
}