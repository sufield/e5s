package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

// Command represents a CLI command with common functionality
type Command struct {
	Name        string
	Description string
	Usage       string
	Examples    []string
	Run         func(args []string) error
}

// NewFlagSet creates a standardized flag set for a command
func (c *Command) NewFlagSet() *flag.FlagSet {
	fs := flag.NewFlagSet(c.Name, flag.ExitOnError)
	fs.Usage = func() { c.PrintUsage() }
	return fs
}

// PrintUsage prints standardized usage information
func (c *Command) PrintUsage() {
	fmt.Fprintf(os.Stderr, "%s\n\n", c.Description)
	fmt.Fprintf(os.Stderr, "USAGE:\n    %s\n\n", c.Usage)
	if len(c.Examples) > 0 {
		fmt.Fprintf(os.Stderr, "EXAMPLES:\n")
		for _, example := range c.Examples {
			fmt.Fprintf(os.Stderr, "    %s\n", example)
		}
	}
}

// CommandRegistry manages all CLI commands
type CommandRegistry struct {
	commands map[string]*Command
	version  VersionInfo
}

// VersionInfo holds build-time version information
type VersionInfo struct {
	Version string
	Commit  string
	Date    string
}

// NewCommandRegistry creates a new command registry
func NewCommandRegistry(v VersionInfo) *CommandRegistry {
	return &CommandRegistry{
		commands: make(map[string]*Command),
		version:  v,
	}
}

// Register adds a command to the registry
func (r *CommandRegistry) Register(cmd *Command) {
	r.commands[cmd.Name] = cmd
}

// Execute runs the appropriate command based on args
func (r *CommandRegistry) Execute(args []string) error {
	if len(args) < 1 {
		r.PrintHelp(os.Stdout)
		return fmt.Errorf("no command specified")
	}

	cmdName := args[0]

	// Handle special commands
	switch cmdName {
	case "help", "-h", "--help":
		r.PrintHelp(os.Stdout)
		return nil
	}

	// Execute registered command
	cmd, ok := r.commands[cmdName]
	if !ok {
		r.PrintHelp(os.Stderr)
		return fmt.Errorf("unknown command: %s", cmdName)
	}

	return cmd.Run(args[1:])
}

// PrintHelp prints overall CLI help
func (r *CommandRegistry) PrintHelp(w io.Writer) {
	fmt.Fprintln(w, "e5s - CLI tool for e5s mTLS library")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "USAGE:")
	fmt.Fprintln(w, "    e5s <command> [arguments]")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "COMMANDS:")

	// Print commands in a consistent order
	order := []string{"version", "spiffe-id", "discover", "validate", "help"}
	for _, name := range order {
		if cmd, ok := r.commands[name]; ok {
			fmt.Fprintf(w, "    %-12s %s\n", cmd.Name, cmd.Description)
		}
	}

	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Run 'e5s <command> --help' for more information on a command.")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "EXAMPLES:")
	fmt.Fprintln(w, "    # Construct a SPIFFE ID for Kubernetes service account")
	fmt.Fprintln(w, "    e5s spiffe-id k8s example.org default api-client")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "    # Discover SPIFFE ID from a running pod")
	fmt.Fprintln(w, "    e5s discover pod e5s-client")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "    # Validate configuration file")
	fmt.Fprintln(w, "    e5s validate e5s.yaml")
}

// TableWriter provides simple table formatting
type TableWriter struct {
	headers []string
	rows    [][]string
	widths  []int
}

// NewTableWriter creates a new table writer
func NewTableWriter(headers []string) *TableWriter {
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}
	return &TableWriter{
		headers: headers,
		widths:  widths,
	}
}

// AddRow adds a row to the table
func (t *TableWriter) AddRow(row []string) {
	t.rows = append(t.rows, row)
	for i, cell := range row {
		if i < len(t.widths) && len(cell) > t.widths[i] {
			t.widths[i] = len(cell)
		}
	}
}

// Print prints the table with borders
func (t *TableWriter) Print() {
	t.printSeparator("┌", "┬", "┐")
	t.printRow(t.headers)
	t.printSeparator("├", "┼", "┤")
	for _, row := range t.rows {
		t.printRow(row)
	}
	t.printSeparator("└", "┴", "┘")
}

func (t *TableWriter) printSeparator(left, mid, right string) {
	fmt.Print(left)
	for i, width := range t.widths {
		fmt.Print(strings.Repeat("─", width+2))
		if i < len(t.widths)-1 {
			fmt.Print(mid)
		}
	}
	fmt.Println(right)
}

func (t *TableWriter) printRow(row []string) {
	fmt.Print("│")
	for i, cell := range row {
		if i < len(t.widths) {
			fmt.Printf(" %-*s │", t.widths[i], cell)
		}
	}
	fmt.Println()
}
