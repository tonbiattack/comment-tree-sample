// Package mysqlconn は MySQL への接続設定を管理するパッケージです。
//
// 接続情報は環境変数から取得します。環境変数が未設定の場合は Docker Compose の
// ポート自動検出を試みます。それも失敗した場合はデフォルト値を使用します。
//
// 環境変数一覧:
//   - MYSQL_HOST: ホスト名（デフォルト: 127.0.0.1）
//   - MYSQL_HOST_PORT: ホスト側ポート番号（未設定時は docker compose port で自動検出）
//   - MYSQL_DATABASE: データベース名（デフォルト: comment_tree）
//   - MYSQL_USER: ユーザー名（デフォルト: comment_user）
//   - MYSQL_PASSWORD: パスワード（デフォルト: comment_pass）
package mysqlconn

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Config は MySQL への接続情報をまとめた構造体です。
type Config struct {
	Host     string
	Port     string
	Database string
	User     string
	Password string
}

// Resolve は環境変数・Docker Compose ポート検出の優先順位で接続設定を解決します。
//
// repoRoot には go.mod が存在するリポジトリルートディレクトリを渡します。
// Docker Compose コマンドをそのディレクトリで実行するために使用します。
//
// ポートの解決順:
//  1. MYSQL_HOST_PORT 環境変数が設定されている場合はその値を使う
//  2. 未設定の場合は docker compose port mysql 3306 でコンテナのポートを自動検出する
//  3. 自動検出も失敗した場合は 33308 をフォールバック値として使う
func Resolve(repoRoot string) Config {
	host := envOrDefault("MYSQL_HOST", "127.0.0.1")
	port := envOrDefault("MYSQL_HOST_PORT", "")
	database := envOrDefault("MYSQL_DATABASE", "comment_tree")
	user := envOrDefault("MYSQL_USER", "comment_user")
	password := envOrDefault("MYSQL_PASSWORD", "comment_pass")

	// ポートが環境変数で指定されていない場合、Docker Compose から自動検出を試みる
	if port == "" {
		if discovered, err := discoverComposePort(repoRoot); err == nil && discovered != "" {
			port = discovered
		}
	}
	// 自動検出も失敗した場合のフォールバックポート
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

// DSN は go-sql-driver/mysql が受け入れる形式の接続文字列（Data Source Name）を返します。
//
// 形式: "user:password@tcp(host:port)/database?options"
//
// multiStatements を true にすると複数のSQL文をセミコロン区切りで一度に実行できます。
// テストのスキーマ再投入（all.sql の一括実行）で使用します。
// 通常のアプリケーション実行では false にして誤った一括実行を防ぎます。
func (c Config) DSN(multiStatements bool) string {
	options := "parseTime=true" // DATETIME カラムを time.Time に自動変換する
	if multiStatements {
		options += "&multiStatements=true"
	}
	return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?%s", c.User, c.Password, c.Host, c.Port, c.Database, options)
}

// discoverComposePort は docker compose コマンドを実行してコンテナのホスト側ポートを取得します。
//
// docker compose port mysql 3306 の出力形式は "0.0.0.0:XXXXX" なので、
// 最後のコロン以降の文字列を抽出してポート番号を取得します。
func discoverComposePort(repoRoot string) (string, error) {
	cmd := exec.Command("docker", "compose", "port", "mysql", "3306")
	// docker-compose.yml が存在するディレクトリで実行する
	cmd.Dir = repoRoot
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("docker compose port failed: %w: %s", err, strings.TrimSpace(stderr.String()))
	}

	// 出力例: "0.0.0.0:49153" または "127.0.0.1:49153"
	output := strings.TrimSpace(stdout.String())
	if output == "" {
		return "", nil
	}

	// 最後のコロンの位置を特定してポート番号を切り出す
	lastColon := strings.LastIndex(output, ":")
	if lastColon == -1 || lastColon == len(output)-1 {
		return "", fmt.Errorf("unexpected docker compose port output: %s", output)
	}
	return output[lastColon+1:], nil
}

// FindRepoRoot はカレントディレクトリから上方向に go.mod を探してリポジトリルートを返します。
//
// テストファイルは深い階層に配置されることがあるため、
// go.mod の位置を基準にしてファイルパスを解決するために使用します。
// go.mod が見つからない場合はエラーを返します。
func FindRepoRoot(start string) (string, error) {
	dir := start
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		// ファイルシステムのルートまで到達した場合はエラー
		if parent == dir {
			return "", fmt.Errorf("go.mod not found from %s", start)
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
