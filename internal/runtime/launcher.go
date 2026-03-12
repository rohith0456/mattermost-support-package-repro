// Package runtime wraps Docker Compose operations for repro project management.
package runtime

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// Launcher manages the Docker Compose lifecycle for a repro project.
type Launcher struct {
	projectDir string
	composeCmd string
}

// NewLauncher creates a Launcher for the given project directory.
func NewLauncher(projectDir string) (*Launcher, error) {
	absDir, err := filepath.Abs(projectDir)
	if err != nil {
		return nil, fmt.Errorf("resolving project dir: %w", err)
	}

	if _, err := os.Stat(filepath.Join(absDir, "docker-compose.yml")); os.IsNotExist(err) {
		return nil, fmt.Errorf("docker-compose.yml not found in %s", absDir)
	}

	composeCmd, err := detectComposeCommand()
	if err != nil {
		return nil, err
	}

	return &Launcher{
		projectDir: absDir,
		composeCmd: composeCmd,
	}, nil
}

// Up starts all services.
func (l *Launcher) Up() error {
	return l.run("up", "-d")
}

// Down stops all services.
func (l *Launcher) Down() error {
	return l.run("down")
}

// Reset stops all services and removes volumes.
func (l *Launcher) Reset() error {
	return l.run("down", "-v")
}

// Status shows service status.
func (l *Launcher) Status() error {
	return l.run("ps")
}

// Logs follows service logs.
func (l *Launcher) Logs(follow bool, service string) error {
	args := []string{"logs"}
	if follow {
		args = append(args, "-f")
	}
	if service != "" {
		args = append(args, service)
	}
	return l.run(args...)
}

// run executes a docker compose subcommand in the project directory.
func (l *Launcher) run(args ...string) error {
	var cmd *exec.Cmd

	if l.composeCmd == "docker-compose" {
		cmd = exec.Command("docker-compose", args...)
	} else {
		// "docker compose" (v2 plugin style)
		fullArgs := append([]string{"compose"}, args...)
		cmd = exec.Command("docker", fullArgs...)
	}

	// Set working directory and env file
	cmd.Dir = l.projectDir
	cmd.Env = append(os.Environ(),
		"COMPOSE_FILE=docker-compose.yml",
	)

	// Check for .env file
	envFile := filepath.Join(l.projectDir, ".env")
	if _, err := os.Stat(envFile); err == nil {
		cmd.Env = append(cmd.Env, "COMPOSE_ENV_FILES=.env")
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	return cmd.Run()
}

// detectComposeCommand determines whether to use "docker compose" or "docker-compose".
func detectComposeCommand() (string, error) {
	// Prefer "docker compose" (v2)
	if err := exec.Command("docker", "compose", "version").Run(); err == nil {
		return "docker compose", nil
	}

	// Fall back to standalone docker-compose
	if _, err := exec.LookPath("docker-compose"); err == nil {
		return "docker-compose", nil
	}

	return "", fmt.Errorf("neither 'docker compose' nor 'docker-compose' found; please install Docker Desktop")
}

// CheckDocker verifies Docker is available and running.
func CheckDocker() error {
	cmd := exec.Command("docker", "info")
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("Docker is not available or not running: %w\n"+
			"Please install Docker Desktop: https://www.docker.com/products/docker-desktop/", err)
	}
	return nil
}

// CheckPorts verifies that required ports are available.
func CheckPorts(ports []int) []int {
	var occupied []int
	for _, port := range ports {
		if !isPortAvailable(port) {
			occupied = append(occupied, port)
		}
	}
	return occupied
}

func isPortAvailable(port int) bool {
	addr := fmt.Sprintf("localhost:%d", port)
	conn, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		// Connection refused or timeout — port is free
		return true
	}
	conn.Close()
	// Successfully connected — port is already in use
	return false
}
