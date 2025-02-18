package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/saika-m/goload/pkg/config"
	"github.com/saika-m/goload/pkg/master"
	"github.com/saika-m/goload/pkg/worker"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile string
	mode    string
)

// rootCmd represents the base command
var rootCmd = &cobra.Command{
	Use:   "goload",
	Short: "A distributed load testing framework written in Go",
	Long: `Goload is a high-performance distributed load testing framework 
that allows you to simulate thousands of users accessing your system 
from multiple geographic locations simultaneously.

Features:
  - Distributed testing across multiple nodes
  - Real-time metrics and visualization
  - Support for multiple protocols (HTTP, gRPC, WebSocket)
  - Scalable from local tests to cloud deployment`,
}

// Execute adds all child commands to the root command and sets flags appropriately
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.goload.yaml)")
	rootCmd.PersistentFlags().StringVar(&mode, "mode", "master", "operation mode (master|worker)")

	// Add sub-commands
	rootCmd.AddCommand(startCmd())
	rootCmd.AddCommand(stopCmd())
	rootCmd.AddCommand(statusCmd())
	rootCmd.AddCommand(workerCmd())
}

func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		// Search config in home directory with name ".goload" (without extension)
		viper.AddConfigPath(home)
		viper.AddConfigPath(".")
		viper.SetConfigName(".goload")
		viper.SetConfigType("yaml")
	}

	// Read environment variables
	viper.AutomaticEnv()

	// If a config file is found, read it in
	if err := viper.ReadInConfig(); err == nil {
		fmt.Printf("Using config file: %s\n", viper.ConfigFileUsed())
	}
}

// startCmd creates the start command
func startCmd() *cobra.Command {
	var (
		vUsers   int
		duration time.Duration
		target   string
		protocol string
	)

	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start a load test",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			// Create configuration
			cfg := &config.TestConfig{
				VirtualUsers: vUsers,
				Duration:     duration,
				Target:       target,
				Protocol:     protocol,
			}

			// Create master node
			m, err := master.NewMaster(cfg)
			if err != nil {
				return fmt.Errorf("failed to create master: %w", err)
			}

			// Handle graceful shutdown
			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

			go func() {
				<-sigChan
				log.Println("Received shutdown signal. Stopping test...")
				cancel()
			}()

			// Start the test
			if err := m.Start(ctx); err != nil {
				return fmt.Errorf("test failed: %w", err)
			}

			return nil
		},
	}

	// Add start-specific flags
	cmd.Flags().IntVar(&vUsers, "vusers", 10, "number of virtual users")
	cmd.Flags().DurationVar(&duration, "duration", 1*time.Minute, "test duration (e.g., 1h, 30m)")
	cmd.Flags().StringVar(&target, "target", "", "target URL or endpoint")
	cmd.Flags().StringVar(&protocol, "protocol", "http", "protocol to use (http|grpc|ws)")

	cmd.MarkFlagRequired("target")

	return cmd
}

// stopCmd creates the stop command
func stopCmd() *cobra.Command {
	var testID string

	cmd := &cobra.Command{
		Use:   "stop",
		Short: "Stop a running load test",
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := master.NewMaster(nil)
			if err != nil {
				return fmt.Errorf("failed to create master: %w", err)
			}

			if err := m.Stop(testID); err != nil {
				return fmt.Errorf("failed to stop test: %w", err)
			}

			fmt.Printf("Successfully stopped test: %s\n", testID)
			return nil
		},
	}

	cmd.Flags().StringVar(&testID, "test-id", "", "ID of the test to stop")
	cmd.MarkFlagRequired("test-id")

	return cmd
}

// statusCmd creates the status command
func statusCmd() *cobra.Command {
	var testID string

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Get the status of a running test",
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := master.NewMaster(nil)
			if err != nil {
				return fmt.Errorf("failed to create master: %w", err)
			}

			status, err := m.GetStatus(testID)
			if err != nil {
				return fmt.Errorf("failed to get test status: %w", err)
			}

			// Print status in a formatted way
			fmt.Printf("Test ID: %s\n", status.TestID)
			fmt.Printf("Status: %s\n", status.State)
			fmt.Printf("Active Users: %d\n", status.ActiveUsers)
			fmt.Printf("Total Requests: %d\n", status.TotalRequests)
			fmt.Printf("Error Rate: %.2f%%\n", status.ErrorRate)
			fmt.Printf("Avg Response Time: %s\n", status.AvgResponseTime)

			return nil
		},
	}

	cmd.Flags().StringVar(&testID, "test-id", "", "ID of the test to check")
	cmd.MarkFlagRequired("test-id")

	return cmd
}

// workerCmd creates the worker command
func workerCmd() *cobra.Command {
	var (
		masterAddr string
		capacity   int
		workDir    string
	)

	cmd := &cobra.Command{
		Use:   "worker",
		Short: "Start a worker node",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			// Ensure work directory exists
			if err := os.MkdirAll(workDir, 0755); err != nil {
				return fmt.Errorf("failed to create work directory: %w", err)
			}

			// Create worker configuration
			cfg := &config.WorkerConfig{
				MasterAddress: masterAddr,
				Capacity:      capacity,
				WorkDir:       workDir,
			}

			// Create and start worker
			w, err := worker.NewWorker(cfg)
			if err != nil {
				return fmt.Errorf("failed to create worker: %w", err)
			}

			// Handle graceful shutdown
			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

			go func() {
				<-sigChan
				log.Println("Received shutdown signal. Stopping worker...")
				cancel()
			}()

			// Start the worker
			if err := w.Start(ctx); err != nil {
				return fmt.Errorf("worker failed: %w", err)
			}

			return nil
		},
	}

	// Add worker-specific flags
	cmd.Flags().StringVar(&masterAddr, "master", "", "master node address")
	cmd.Flags().IntVar(&capacity, "capacity", 100, "maximum number of virtual users")
	cmd.Flags().StringVar(&workDir, "work-dir", filepath.Join(os.TempDir(), "goload-worker"), "worker working directory")

	cmd.MarkFlagRequired("master")

	return cmd
}
