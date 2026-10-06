package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"time"

	"github.com/ziggornif/gimme/internal/publish"
)

var version = "dev"

func main() {
	os.Exit(run(os.Args[1:], os.Getenv, os.Stdout, os.Stderr))
}

func run(args []string, env func(string) string, stdout, stderr io.Writer) int {
	if len(args) == 1 && args[0] == "version" {
		_, _ = fmt.Fprintln(stdout, version)
		return 0
	}
	if len(args) == 0 || args[0] != "push" {
		if len(args) > 0 {
			_, _ = fmt.Fprintf(stderr, "gimme-cli: unknown subcommand %q\n", args[0])
		}
		printUsage(stderr)
		return 2
	}
	name, packageVersion, url, token, dir, err := parsePush(args[1:])
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "gimme-cli: %v\n", err)
		printUsage(stderr)
		return 2
	}
	if url == "" {
		url = env("GIMME_URL")
	}
	if token == "" {
		token = env("GIMME_TOKEN")
	}
	for _, required := range []struct{ label, value string }{
		{"--name", name},
		{"--url or GIMME_URL", url},
		{"--token or GIMME_TOKEN", token},
	} {
		if required.value == "" {
			_, _ = fmt.Fprintf(stderr, "gimme-cli: %s is required\n", required.label)
			printUsage(stderr)
			return 2
		}
	}
	workingDir, err := os.Getwd()
	if err != nil {
		return fail(stderr, fmt.Errorf("get working directory: %w", err))
	}
	resolved, source, err := publish.ResolveVersion(packageVersion, publish.VersionSources{
		WorkingDir: workingDir,
		LookupEnv:  env,
		RunGit: func() (string, error) {
			output, commandErr := exec.Command("git", "describe", "--exact-match", "--tags", "HEAD").Output()
			return string(output), commandErr
		},
	})
	if err != nil {
		return fail(stderr, err)
	}
	_, _ = fmt.Fprintf(stdout, "version %s (from %s)\n", resolved, source)
	archivePath, err := publish.Archive(dir)
	if err != nil {
		return fail(stderr, err)
	}
	defer func() { _ = os.Remove(archivePath) }()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	client := &http.Client{Timeout: 10 * time.Minute}
	if err = publish.Push(ctx, client, publish.Options{URL: url, Token: token, Name: name, Version: resolved, ArchivePath: archivePath}); err != nil {
		return fail(stderr, err)
	}
	base := strings.TrimRight(url, "/")
	_, _ = fmt.Fprintf(stdout, "published %s@%s to %s/gimme/%s@%s/\n", name, resolved, base, name, resolved)
	return 0
}

func parsePush(args []string) (name, packageVersion, url, token, dir string, err error) {
	flags := flag.NewFlagSet("push", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.StringVar(&name, "name", "", "package name")
	flags.StringVar(&packageVersion, "version", "", "package version")
	flags.StringVar(&url, "url", "", "gimme URL")
	flags.StringVar(&token, "token", "", "gimme token")
	reordered := make([]string, 0, len(args))
	positionals := make([]string, 0, 1)
	for len(args) > 0 {
		arg := args[0]
		args = args[1:]
		if strings.HasPrefix(arg, "-") {
			reordered = append(reordered, arg)
			if !strings.Contains(arg, "=") {
				if len(args) == 0 || strings.HasPrefix(args[0], "-") {
					return "", "", "", "", "", fmt.Errorf("flag %s requires a value", arg)
				}
				value := args[0]
				args = args[1:]
				reordered = append(reordered, value)
			}
		} else {
			positionals = append(positionals, arg)
		}
	}
	if err = flags.Parse(reordered); err != nil {
		return
	}
	if len(positionals) != 1 {
		err = fmt.Errorf("push requires exactly one directory")
		return
	}
	dir = positionals[0]
	return
}

func fail(stderr io.Writer, err error) int {
	_, _ = fmt.Fprintf(stderr, "gimme-cli: %v\n", err)
	return 1
}

func printUsage(w io.Writer) {
	_, _ = fmt.Fprintln(w, "usage:")
	_, _ = fmt.Fprintln(w, "  gimme-cli push <dir> --name <name> [--version <version>] [--url <url>] [--token <token>]")
	_, _ = fmt.Fprintln(w, "  gimme-cli version")
}
