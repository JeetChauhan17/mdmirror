package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/JeetChauhan17/mdmirror/internal/app"
	"github.com/JeetChauhan17/mdmirror/internal/config"
	"github.com/JeetChauhan17/mdmirror/internal/mirror"
	"github.com/JeetChauhan17/mdmirror/internal/watcher"
)

func usage() {
	fmt.Print(`mdmirror - documentation-only project mirror

Usage:
  mdmirror init
  mdmirror add <name> <source>
  mdmirror list
  mdmirror remove <name>
  mdmirror sync  <source> <destination>
  mdmirror watch <source> <destination>
  mdmirror sync-all
  mdmirror start

Commands:
  init      Create the default configuration file
  add       Add a project to the configuration file
  list      List configured 
  remove    Remove a project from the configuration file
  sync      Mirror Markdown files from source to destination
  watch     Watch source and automatically keep the mirror synchronized
  sync-all  Sync all projects from the configuration file
  start     Watch all projects from the configuration file
`)
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "init":
		runInit()

	case "add":
		runAdd()

	case "list":
		runList()

	case "remove":
		runRemove()

	case "sync":
		runLegacySync()

	case "watch":
		runLegacyWatch()

	case "sync-all":
		runSyncAll()

	case "start":
		runStart()

	default:
		fmt.Printf("Unknown command: %s\n\n", os.Args[1])
		usage()
		os.Exit(1)
	}
}

func runInit() {
	if len(os.Args) != 2 {
		fmt.Println("Usage: mdmirror init")
		os.Exit(1)
	}

	path, err := config.Init()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Configuration created: %s\n", path)
	fmt.Println("Edit the configuration to add your projects.")
}

func runAdd() {
	if len(os.Args) != 4 {
		fmt.Println("Usage: mdmirror add <name> <source>")
		os.Exit(1)
	}

	path, err := config.DefaultPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if err := config.Add(path, os.Args[2], os.Args[3]); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Project added: %s\n", os.Args[2])
	fmt.Printf("  Source: %s\n", os.Args[3])
}

func runList() {
	if len(os.Args) != 2 {
		fmt.Println("Usage: mdmirror list")
		os.Exit(1)
	}

	configPath, err := config.DefaultPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	cfg, err := loadConfigFromPath(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Projects (%d)\n\n", len(cfg.Projects))

	for i, project := range cfg.Projects {
		if i > 0 {
			fmt.Println()
		}

		fmt.Printf("%s\n", project.Name)
		fmt.Printf("  Source:      %s\n", project.Source)
		fmt.Printf("  Destination: %s\n", project.Destination)
	}
}

func runRemove() {
	if len(os.Args) != 3 {
		fmt.Println("Usage: mdmirror remove <name>")
		os.Exit(1)
	}

	configPath, err := config.DefaultPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	removed, err := config.Remove(configPath, os.Args[2])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	cfg, err := loadConfigFromPath(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	destination := filepath.Join(cfg.Vault, removed.Name)

	if err := mirror.Remove(destination); err != nil {
		fmt.Fprintf(os.Stderr, "Error: remove project mirror: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Project removed: %s\n", removed.Name)

	fmt.Printf("  Source: %s\n", removed.Source)
}

func runLegacySync() {
	if len(os.Args) != 4 {
		fmt.Println("Usage: mdmirror sync <source> <destination>")
		os.Exit(1)
	}

	source := os.Args[2]
	destination := os.Args[3]

	fmt.Printf("Mirroring Markdown files...\n")
	fmt.Printf("  Source:      %s\n", source)
	fmt.Printf("  Destination: %s\n\n", destination)

	if err := mirror.Mirror(source, destination); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✓ Mirror complete")
}

func runLegacyWatch() {
	if len(os.Args) != 4 {
		fmt.Println("Usage: mdmirror watch <source> <destination>")
		os.Exit(1)
	}

	if err := runWatch(os.Args[2], os.Args[3]); err != nil {
		if errors.Is(err, context.Canceled) {
			fmt.Println("\nWatcher stopped.")
			return
		}

		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runSyncAll() {
	if len(os.Args) != 2 {
		fmt.Println("Usage: mdmirror sync-all")
		os.Exit(1)
	}

	cfg, err := loadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Syncing %d projects...\n\n", len(cfg.Projects))

	application := app.New(cfg)

	if err := application.Sync(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	for _, project := range cfg.Projects {
		fmt.Printf("✓ %s\n", project.Name)
	}

	fmt.Println("\n✓ All projects synchronized")
}

func runStart() {
	if len(os.Args) != 2 && len(os.Args) != 4 {
		fmt.Println("Usage: mdmirror start [--config <path>]")
		os.Exit(1)
	}

	configPath, err := config.DefaultPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if len(os.Args) == 4 {
		if os.Args[2] != "--config" {
			fmt.Println("Usage: mdmirror start [--config <path>]")
			os.Exit(1)
		}

		configPath = os.Args[3]
	}

	cfg, err := loadConfigFromPath(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	fmt.Printf("Watching %d projects...\n", len(cfg.Projects))

	for _, project := range cfg.Projects {
		fmt.Printf("  %s\n", project.Name)
	}

	fmt.Println("\nConfiguration reload: every 1 second.")
	fmt.Println("Press Ctrl+C to stop.")

	application := app.New(cfg)

	if err := application.RunReloadLoop(ctx, configPath, time.Second); err != nil {
		if errors.Is(err, context.Canceled) {
			fmt.Println("\nWatcher stopped.")
			return
		}

		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runWatch(source, destination string) error {
	w, err := watcher.New(source, destination)
	if err != nil {
		return err
	}
	defer w.Close()

	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	fmt.Printf("Watching %s\n", source)
	fmt.Printf("Mirroring to %s\n", destination)
	fmt.Println("Press Ctrl+C to stop.")

	return w.Start(ctx)
}
