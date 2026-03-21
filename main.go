package main

import (
	"fmt"
	"log"
	"os"

	"github.com/scopweb/mcp-filesystem-server/filesystemserver"
	"github.com/mark3labs/mcp-go/server"
)

func main() {
	// Parse command line arguments.
	// Directories can also be provided dynamically via MCP Roots (roots/list).
	allowedDirs := os.Args[1:]
	if len(allowedDirs) == 0 {
		fmt.Fprintf(os.Stderr,
			"Info: no allowed directories specified; expecting MCP client to provide roots via roots/list.\n")
	}

	// Create and start the server
	fss, err := filesystemserver.NewFilesystemServer(allowedDirs)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	// Serve requests
	if err := server.ServeStdio(fss); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}