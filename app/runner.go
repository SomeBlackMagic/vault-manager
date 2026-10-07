package app

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/jhunt/go-ansi"

	"github.com/SomeBlackMagic/vault-manager/logging"
)

// UsageError is a sentinel error type for usage/argument errors.
// It allows callers to detect usage errors via errors.As rather than
// comparing error strings.
type UsageError struct {
	msg string
}

func (e *UsageError) Error() string { return e.msg }

// NewUsageError creates a new UsageError with a formatted message.
func NewUsageError(format string, args ...interface{}) *UsageError {
	return &UsageError{msg: fmt.Sprintf(format, args...)}
}

const (
	DestructiveCommand    string = "@R"
	NonDestructiveCommand        = "@G"
	AdministrativeCommand        = "@W"
	MiscellaneousCommand         = "@W"
	HiddenCommand                = "HIDEME"
)

type Help struct {
	Summary     string
	Usage       string
	Description string
	Type        string
}

type Handler func(command string, args ...string) error

type Runner struct {
	Handlers map[string]Handler
	Topics   map[string]*Help

	// Logger receives diagnostic messages. It is replaced once the global
	// logging options have been parsed; it never writes to stdout.
	Logger *slog.Logger
}

func NewRunner() *Runner {
	return &Runner{
		Handlers: make(map[string]Handler),
		Topics:   make(map[string]*Help),
		Logger:   logging.Discard(),
	}
}

// Log returns the runner's logger, never nil.
func (r *Runner) Log() *slog.Logger {
	return logging.OrDiscard(r.Logger)
}

func (r *Runner) Dispatch(command string, help *Help, fn Handler) {
	if help != nil {
		help.Description = strings.Trim(help.Description, "\n")
	}

	r.Handlers[command] = fn
	if help != nil && help.Type != HiddenCommand {
		r.Topics[command] = help
	}
}

func (r *Runner) HelpTopic(topic string, help string) {
	r.Topics[topic] = &Help{Description: strings.Trim(help, "\n")}
}

func (r *Runner) Help(out io.Writer, topic string) {
	if topic == "commands" {
		fmt.Fprintf(out, "Valid commands are:\n\n")

		ll := make([]string, 0)
		for cmd := range r.Handlers {
			ll = append(ll, cmd)
		}

		sort.Strings(ll)
		for _, cmd := range ll {
			if h := r.Topics[cmd]; h != nil {
				f := h.Type
				if f == "" {
					f = "@W"
				}
				ansi.Fprintf(out, "    "+f+"{%-10s}  %s\n", cmd, h.Summary)
			}
		}

		fmt.Fprintf(out, "\nTry `safe envvars' for information on available environment variables\n")
		fmt.Fprintf(out, "Try 'safe help <command>' for detailed information on specific commands\n")
		return
	}

	if help, ok := r.Topics[topic]; ok && help != nil {
		if help.Summary != "" {
			/* this is a command, print it like one */
			ansi.Fprintf(out, "safe @G{%s} - @C{%s}\n", topic, help.Summary)
			if help.Usage != "" {
				ansi.Fprintf(out, "USAGE: "+help.Usage+"\n")
			}
			if help.Description != "" {
				ansi.Fprintf(out, "\n")
			}
		}
		if help.Description != "" {
			ansi.Fprintf(out, help.Description+"\n")
		}
		return
	}

	ansi.Fprintf(out, "@R{Unrecognized command or help topic '%s'}\n", topic)
	fmt.Fprintf(out, "Try 'safe help' to get started with safe,\n")
	fmt.Fprintf(out, " or 'safe commands' for a list of valid commands\n")
	os.Exit(1)
}

func (r *Runner) ExitWithUsage(topic string) {
	if help, ok := r.Topics[topic]; ok && help != nil {
		if help.Summary != "" {
			/* this is a command, print it like one */
			ansi.Fprintf(os.Stderr, "safe @G{%s} - @C{%s}\n", topic, help.Summary)
			if help.Usage != "" {
				ansi.Fprintf(os.Stderr, "USAGE: "+help.Usage+"\n")
			}
		}
	}
	os.Exit(1)
}

// Execute runs the handler registered for command. Only the command name and
// the number of arguments are logged: arguments may carry secrets.
func (r *Runner) Execute(command string, args ...string) error {
	fn, ok := r.Handlers[command]
	if !ok {
		return fmt.Errorf("unknown command '%s'", command)
	}

	log := r.Log().With(logging.KeyCommand, command)
	log.Debug("command started", "args", len(args))
	start := time.Now()

	err := fn(command, args...)

	duration := time.Since(start)
	if err != nil {
		log.Debug("command failed",
			"error_type", ErrorType(err),
			logging.KeyError, err.Error(),
			logging.KeyDuration, duration)
		return err
	}
	log.Debug("command completed", logging.KeyDuration, duration)
	return nil
}

// ErrorType classifies err for diagnostic logs.
func ErrorType(err error) string {
	var usageErr *UsageError
	if errors.As(err, &usageErr) {
		return "usage"
	}
	return "runtime"
}
