package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Shreehari-Acharya/Bannin/daemon/internal/dispatcher"
	"github.com/Shreehari-Acharya/Bannin/daemon/internal/installers"
	"github.com/Shreehari-Acharya/Bannin/daemon/internal/receiver"
	"github.com/Shreehari-Acharya/Bannin/daemon/internal/ui"
	"github.com/Shreehari-Acharya/Bannin/daemon/pkg/models"
	"github.com/Shreehari-Acharya/Bannin/daemon/watchers"
	tea "github.com/charmbracelet/bubbletea"
)

const backendURL = "http://localhost:4000"

func main() {
	fmt.Printf("backend URL configured as %s\n", backendURL)

	if len(os.Args) > 1 && os.Args[1] == "init" {
		if err := startAgentAPIServer(); err != nil {
			fmt.Printf("fatal error starting Agent API server: %v\n", err)
			os.Exit(1)
		}
		runInstallerUI(backendURL)
		return
	}

	eventQueue := make(chan models.SecEvent, 100)
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		fmt.Println(watchers.DescribeFalcoListener("8081"))
		if err := watchers.StartFalcoHTTP("8081", eventQueue); err != nil {
			fmt.Printf("fatal error starting Falco watcher: %v\n", err)
			os.Exit(1)
		}
	}()

	go func() {
		for alert := range eventQueue {
			if err := dispatcher.SendAlerts(alert, backendURL); err != nil {
				fmt.Printf("error sending alert: %v\n", err)
				continue
			}

			fmt.Println("alert sent successfully")
		}
	}()

	if err := startAgentAPIServer(); err != nil {
		fmt.Printf("fatal error starting Agent API server: %v\n", err)
		os.Exit(1)
	}

	<-signalChan
	fmt.Println("daemon shutting down...")
}

func startAgentAPIServer() error {
	handler := receiver.NewHandler()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /tools/read", handler.HandleToolsRead)
	mux.HandleFunc("POST /tools/write", handler.HandleToolsWrite)
	mux.HandleFunc("POST /tools/edit", handler.HandleToolsEdit)
	mux.HandleFunc("GET /tools/validate", handler.HandleToolsValidate)
	mux.HandleFunc("POST /tools/validate", handler.HandleToolsValidate)
	mux.HandleFunc("GET /tools/restart", handler.HandleToolsRestart)
	mux.HandleFunc("GET /tools/direnum", handler.HandleDirEnum)

	listener, err := net.Listen("tcp", ":8080")
	if err != nil {
		return err
	}

	server := &http.Server{Handler: mux}

	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			fmt.Printf("Agent API server error: %v\n", err)
		}
	}()

	go func() {
		<-time.After(24 * time.Hour)
		_ = server.Shutdown(context.Background())
	}()

	fmt.Println("Agent API listening on :8080")
	return nil
}

func runInstallerUI(backendURL string) {
	tools := []installers.SecurityTools{
		installers.NewFalcoTool(),
	}

	program := tea.NewProgram(ui.InitialModel(tools, backendURL), tea.WithAltScreen())
	if _, err := program.Run(); err != nil {
		fmt.Printf("fatal error in UI: %v\n", err)
		os.Exit(1)
	}
}
