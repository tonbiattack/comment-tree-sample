package testdb

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

func OpenMySQL(t *testing.T) *sql.DB {
	t.Helper()

	host := envOrDefault("MYSQL_HOST", "127.0.0.1")
	port := envOrDefault("MYSQL_HOST_PORT", "33306")
	database := envOrDefault("MYSQL_DATABASE", "comment_tree")
	user := envOrDefault("MYSQL_USER", "comment_user")
	password := envOrDefault("MYSQL_PASSWORD", "comment_pass")

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&multiStatements=true", user, password, host, port, database)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("failed to open mysql: %v", err)
	}

	if err := db.Ping(); err != nil {
		t.Fatalf("failed to ping mysql: %v", err)
	}

	return db
}

func LockDatabase(t *testing.T) {
	t.Helper()

	db := OpenMySQL(t)
	conn, err := db.Conn(t.Context())
	if err != nil {
		db.Close()
		t.Fatalf("failed to get mysql connection for lock: %v", err)
	}

	var acquired int
	if err := conn.QueryRowContext(t.Context(), "SELECT GET_LOCK('comment_tree_test_lock', 30)").Scan(&acquired); err != nil {
		conn.Close()
		db.Close()
		t.Fatalf("failed to acquire mysql lock: %v", err)
	}
	if acquired != 1 {
		conn.Close()
		db.Close()
		t.Fatal("failed to acquire mysql lock within timeout")
	}

	t.Cleanup(func() {
		releaseCtx, cancel := contextWithTimeout()
		defer cancel()
		_, _ = conn.ExecContext(releaseCtx, "DO RELEASE_LOCK('comment_tree_test_lock')")
		_ = conn.Close()
		_ = db.Close()
	})
}

func SQLQueryDir(t *testing.T) string {
	t.Helper()

	root := repoRoot(t)
	return filepath.Join(root, "sql", "queries")
}

func SQLQuerySubDir(t *testing.T, subdir string) string {
	t.Helper()

	return filepath.Join(SQLQueryDir(t), subdir)
}

func ResetSchema(t *testing.T, db *sql.DB) {
	t.Helper()

	root := repoRoot(t)
	schemaPath := filepath.Join(root, "sql", "all.sql")
	content, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatalf("failed to read schema sql: %v", err)
	}

	statements := strings.TrimSpace(string(content))
	if statements == "" {
		t.Fatal("schema sql is empty")
	}

	if _, err := db.Exec("SET FOREIGN_KEY_CHECKS = 0;"); err != nil {
		t.Fatalf("failed to disable foreign key checks: %v", err)
	}
	if _, err := db.Exec("DROP TABLE IF EXISTS comment_closures; DROP TABLE IF EXISTS comments; DROP TABLE IF EXISTS posts;"); err != nil {
		t.Fatalf("failed to drop tables: %v", err)
	}
	if _, err := db.Exec("SET FOREIGN_KEY_CHECKS = 1;"); err != nil {
		t.Fatalf("failed to enable foreign key checks: %v", err)
	}
	if _, err := db.Exec(statements); err != nil {
		t.Fatalf("failed to execute schema sql: %v", err)
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found from current directory")
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

func contextWithTimeout() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 5*time.Second)
}
