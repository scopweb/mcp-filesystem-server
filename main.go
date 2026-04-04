package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/mark3labs/mcp-go/server"
	"github.com/scopweb/mcp-filesystem-go-light/filesystemserver"
)

func main() {
	devMode := flag.Bool("dev", false, "Enable development-only features")
	logDir := flag.String("log-dir", "", "Directory for development audit logs (requires --dev)")
	flag.Parse()

	if *logDir != "" && !*devMode {
		log.Fatalf("--log-dir requires --dev")
	}
	if *devMode && *logDir == "" {
		log.Fatalf("--dev requires --log-dir")
	}

	// Parse command line arguments.
	// Directories can also be provided dynamically via MCP Roots (roots/list).
	allowedDirs := flag.Args()
	if len(allowedDirs) == 0 {
		fmt.Fprintf(os.Stderr,
			"Info: no allowed directories specified; expecting MCP client to provide roots via roots/list.\n")
	}

	// Create and start the server
	options := make([]filesystemserver.ServerOption, 0, 1)
	if *devMode {
		options = append(options, filesystemserver.WithDevelopmentAuditLogging(*logDir))
	}

	fss, closer, err := filesystemserver.NewFilesystemServerWithOptions(allowedDirs, options...)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}
	defer closer.Close()

	// Serve requests
	if err := server.ServeStdio(fss); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
