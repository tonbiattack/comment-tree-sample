package mysqlconn

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Config struct {
	Host     string
	Port     string
	Database string
	User     string
	Password string
}

func Resolve(repoRoot string) Config {
	host := envOrDefault("MYSQL_HOST", "127.0.0.1")
	port := envOrDefault("MYSQL_HOST_PORT", "")
	database := envOrDefault("MYSQL_DATABASE", "comment_tree")
	user := envOrDefault("MYSQL_USER", "comment_user")
	password := envOrDefault("MYSQL_PASSWORD", "comment_pass")

	if port == "" {
		if discovered, err := discoverComposePort(repoRoot); err == nil && discovered != "" {
			port = discovered
		}
	}
	if port == "" {
		port = "33308"
	}

	return Config{
		Host:     host,
		Port:     port,
		Database: database,
		User:     user,
		Password: password,
	}
}

func (c Config) DSN(multiStatements bool) string {
	options := "parseTime=true"
	if multiStatements {
		options += "&multiStatements=true"
	}
	return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?%s", c.User, c.Password, c.Host, c.Port, c.Database, options)
}

func discoverComposePort(repoRoot string) (string, error) {
	cmd := exec.Command("docker", "compose", "port", "mysql", "3306")
	cmd.Dir = repoRoot
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("docker compose port failed: %w: %s", err, strings.TrimSpace(stderr.String()))
	}

	output := strings.TrimSpace(stdout.String())
	if output == "" {
		return "", nil
	}

	lastColon := strings.LastIndex(output, ":")
	if lastColon == -1 || lastColon == len(output)-1 {
		return "", fmt.Errorf("unexpected docker compose port output: %s", output)
	}
	return output[lastColon+1:], nil
}

func FindRepoRoot(start string) (string, error) {
	dir := start
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found from %s", start)
		}
		dir = parent
	}
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
