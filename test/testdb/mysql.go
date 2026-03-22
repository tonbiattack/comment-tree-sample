// Package testdb はテスト用の MySQL 接続ヘルパーを提供するパッケージです。
//
// このパッケージは integration test 専用であり、実際の MySQL に接続します。
// テスト戦略:
//   - モックを使わず実 DB を使用することで、SQL の動作を実際の環境で確認する
//   - 各テスト前に ResetSchema でスキーマを再投入し、テスト間の独立性を確保する
//   - LockDatabase で MySQL の GET_LOCK を使い、並行テスト実行時のスキーマ競合を防ぐ
package testdb

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql" // MySQL ドライバーを database/sql に登録する

	"private-comment-tree-sample/internal/mysqlconn"
)

// OpenMySQL はテスト用の MySQL 接続を開いて返します。
//
// 接続情報は mysqlconn.Resolve でリポジトリルートを基準に解決します。
// 接続に失敗した場合はテストを即時 Fatal で終了します。
func OpenMySQL(t *testing.T) *sql.DB {
	t.Helper()

	root := repoRoot(t)
	cfg := mysqlconn.Resolve(root)
	// multiStatements=true にすることで all.sql の複数ステートメントを一括実行できる
	dsn := cfg.DSN(true)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("failed to open mysql: %v", err)
	}

	// Open は接続を遅延するため、Ping で実際の接続確認を行う
	if err := db.Ping(); err != nil {
		t.Fatalf("failed to ping mysql: %v", err)
	}

	return db
}

// LockDatabase は MySQL の GET_LOCK でテスト用の名前付きロックを取得します。
//
// 複数のテストパッケージが並行して実行されると、同じ DB を同時にリセットして
// 互いのデータを破壊し合う可能性があります。
// GET_LOCK('comment_tree_test_lock', 30) を使うことで、
// スキーマ再投入とテスト実行をグローバルにシリアライズします。
//
// ロックは t.Cleanup に登録されており、テスト終了時に自動で解放されます。
// ロック取得に 30 秒以内に失敗した場合はテストを Fatal で終了します。
func LockDatabase(t *testing.T) {
	t.Helper()

	db := OpenMySQL(t)
	// ロックは接続に紐づくため、専用の conn を確保して最後まで保持する
	conn, err := db.Conn(t.Context())
	if err != nil {
		db.Close()
		t.Fatalf("failed to get mysql connection for lock: %v", err)
	}

	// GET_LOCK は取得成功=1、タイムアウト=0、エラー=NULL を返す
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

	// テスト終了時にロックを解放し、接続を閉じる
	t.Cleanup(func() {
		releaseCtx, cancel := contextWithTimeout()
		defer cancel()
		_, _ = conn.ExecContext(releaseCtx, "DO RELEASE_LOCK('comment_tree_test_lock')")
		_ = conn.Close()
		_ = db.Close()
	})
}

// SQLQueryDir は標準の SQL クエリディレクトリ（sql/queries）の絶対パスを返します。
//
// 隣接リスト方式（commenttree）のリポジトリに渡すパスとして使用します。
func SQLQueryDir(t *testing.T) string {
	t.Helper()

	root := repoRoot(t)
	return filepath.Join(root, "sql", "queries")
}

// SQLQuerySubDir は SQL クエリディレクトリのサブディレクトリの絶対パスを返します。
//
// 例: SQLQuerySubDir(t, "closure") → "リポジトリルート/sql/queries/closure"
// 閉包テーブル・Materialized Path・ビジネスロジックのリポジトリに渡すパスとして使用します。
func SQLQuerySubDir(t *testing.T, subdir string) string {
	t.Helper()

	return filepath.Join(SQLQueryDir(t), subdir)
}

// ResetSchema はテスト用 DB のスキーマをリセットし、初期データを再投入します。
//
// 手順:
//  1. 外部キー制約を一時的に無効化する（DROP TABLE の順序に依存しないため）
//  2. comment_closures / comments / posts テーブルを DROP する
//  3. 外部キー制約を再有効化する
//  4. sql/all.sql を実行してスキーマ作成と初期データ投入を行う
//
// sql/all.sql にはスキーマ定義（CREATE TABLE）と初期データ（INSERT）が
// 一括で記述されています。
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

	// 外部キー制約を無効化してから DROP する（テーブルの依存順序を気にしなくて済む）
	if _, err := db.Exec("SET FOREIGN_KEY_CHECKS = 0;"); err != nil {
		t.Fatalf("failed to disable foreign key checks: %v", err)
	}
	if _, err := db.Exec("DROP TABLE IF EXISTS comment_closures; DROP TABLE IF EXISTS comments; DROP TABLE IF EXISTS posts;"); err != nil {
		t.Fatalf("failed to drop tables: %v", err)
	}
	if _, err := db.Exec("SET FOREIGN_KEY_CHECKS = 1;"); err != nil {
		t.Fatalf("failed to enable foreign key checks: %v", err)
	}
	// all.sql には複数のステートメントが含まれるため multiStatements=true が必要
	if _, err := db.Exec(statements); err != nil {
		t.Fatalf("failed to execute schema sql: %v", err)
	}
}

// repoRoot はカレントディレクトリから上方向に go.mod を探してリポジトリルートを返します。
//
// テストは各パッケージのディレクトリで実行されるため、
// go.mod を基準にプロジェクト全体のリソースへのパスを解決します。
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
		// ファイルシステムのルートまで到達した場合はエラー
		if parent == dir {
			t.Fatal("go.mod not found from current directory")
		}
		dir = parent
	}
}

// envOrDefault は環境変数 key の値を返します。
// 環境変数が未設定または空の場合は fallback を返します。
func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

// contextWithTimeout は 5 秒のタイムアウト付きコンテキストを返します。
//
// ロック解放など、テスト終了時のクリーンアップ処理に使用します。
func contextWithTimeout() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 5*time.Second)
}
