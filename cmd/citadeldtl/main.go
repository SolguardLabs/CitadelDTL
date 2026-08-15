package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"citadeldtl/src/api"
	"citadeldtl/src/report"
	"citadeldtl/src/scenario"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "run":
		if len(os.Args) != 3 {
			fmt.Fprintln(os.Stderr, "usage: citadeldtl run <fixture.json>")
			os.Exit(2)
		}
		result, err := scenario.RunFile(os.Args[2])
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
		if err := report.WriteJSON(os.Stdout, result); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
	case "list":
		for _, name := range scenario.BuiltinScenarios() {
			fmt.Println(name)
		}
	case "serve":
		if err := serve(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
	default:
		usage()
		os.Exit(2)
	}
}

func serve(arguments []string) error {
	flags := flag.NewFlagSet("serve", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	address := flags.String("address", "127.0.0.1:8080", "HTTP listen address")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("serve does not accept positional arguments")
	}
	service, err := api.NewService()
	if err != nil {
		return fmt.Errorf("initialize service: %w", err)
	}
	server := &http.Server{
		Addr:              *address,
		Handler:           api.NewHTTPServer(service),
		ReadHeaderTimeout: 3 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    32 << 10,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	fmt.Fprintf(os.Stderr, "CitadelDTL listening on %s\n", server.Addr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve CitadelDTL: %w", err)
	}
	return nil
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage:")
	fmt.Fprintln(os.Stderr, "  citadeldtl run <fixture.json>")
	fmt.Fprintln(os.Stderr, "  citadeldtl list")
	fmt.Fprintln(os.Stderr, "  citadeldtl serve [--address 127.0.0.1:8080]")
}
