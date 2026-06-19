package db

import (
	"reflect"
	"strings"
	"testing"

	"github.com/v4run/hangar/internal/config"
)

// argsTail returns args without the leading argv[0] (the absolute binary
// path resolved by exec.LookPath, which varies per machine).
func argsTail(args []string) []string {
	if len(args) == 0 {
		return nil
	}
	return args[1:]
}

func TestBuildPostgresPSQL(t *testing.T) {
	d := &config.Database{
		Name: "p", Engine: config.EnginePostgres,
		Host: "db.example", Port: 5432, User: "u", DBName: "app",
		Client: config.ClientPSQL,
	}
	cmd, err := Build(d, "", 0, "secret")
	if err != nil {
		t.Skipf("psql not installed: %v", err)
	}
	if !strings.HasSuffix(cmd.Path, "psql") {
		t.Fatalf("path: %s", cmd.Path)
	}
	want := []string{"-h", "db.example", "-p", "5432", "-U", "u", "-d", "app"}
	if !reflect.DeepEqual(argsTail(cmd.Args), want) {
		t.Fatalf("args: %v want %v", argsTail(cmd.Args), want)
	}
	if len(cmd.Env) != 1 || cmd.Env[0] != "PGPASSWORD=secret" {
		t.Fatalf("env: %v", cmd.Env)
	}
}

func TestBuildPostgresPSQLOverrideHostPort(t *testing.T) {
	d := &config.Database{Name: "p", Engine: config.EnginePostgres, Host: "db.example", Port: 5432, User: "u", Client: config.ClientPSQL}
	cmd, err := Build(d, "127.0.0.1", 6543, "")
	if err != nil {
		t.Skipf("psql not installed: %v", err)
	}
	tail := argsTail(cmd.Args)
	if tail[1] != "127.0.0.1" || tail[3] != "6543" {
		t.Fatalf("override not applied: %v", tail)
	}
	if len(cmd.Env) != 0 {
		t.Fatalf("expected no env when password empty: %v", cmd.Env)
	}
}

func TestBuildMySQL(t *testing.T) {
	d := &config.Database{Name: "m", Engine: config.EngineMySQL, Host: "h", Port: 3306, User: "u", DBName: "db"}
	cmd, err := Build(d, "", 0, "pw")
	if err != nil {
		t.Skipf("mysql not installed: %v", err)
	}
	want := []string{"-h", "h", "-P", "3306", "-u", "u", "db"}
	if !reflect.DeepEqual(argsTail(cmd.Args), want) {
		t.Fatalf("args: %v want %v", argsTail(cmd.Args), want)
	}
	if len(cmd.Env) != 1 || cmd.Env[0] != "MYSQL_PWD=pw" {
		t.Fatalf("env: %v", cmd.Env)
	}
}

func TestBuildRedis(t *testing.T) {
	d := &config.Database{Name: "r", Engine: config.EngineRedis, Host: "h", Port: 6379}
	cmd, err := Build(d, "", 0, "pw")
	if err != nil {
		t.Skipf("redis-cli not installed: %v", err)
	}
	want := []string{"-h", "h", "-p", "6379"}
	if !reflect.DeepEqual(argsTail(cmd.Args), want) {
		t.Fatalf("args: %v want %v", argsTail(cmd.Args), want)
	}
	if len(cmd.Env) != 1 || cmd.Env[0] != "REDISCLI_AUTH=pw" {
		t.Fatalf("env: %v", cmd.Env)
	}
}

func TestBuildSQLite(t *testing.T) {
	d := &config.Database{Name: "s", Engine: config.EngineSQLite, Host: "/tmp/x.db"}
	cmd, err := Build(d, "", 0, "")
	if err != nil {
		t.Skipf("sqlite3 not installed: %v", err)
	}
	tail := argsTail(cmd.Args)
	if len(tail) != 1 || tail[0] != "/tmp/x.db" {
		t.Fatalf("args: %v", tail)
	}
}

func TestBuildPostgresPGCLI(t *testing.T) {
	d := &config.Database{
		Name: "p", Engine: config.EnginePostgres,
		Host: "db", Port: 5432, User: "u", DBName: "app",
		Client: config.ClientPGCLI,
	}
	cmd, err := Build(d, "", 0, "se cret/with@special#")
	if err != nil {
		t.Skipf("pgcli not installed: %v", err)
	}
	if len(cmd.Args) != 2 {
		t.Fatalf("expected pgcli + URL: %v", cmd.Args)
	}
	if !strings.HasPrefix(cmd.Args[1], "postgresql://u:") {
		t.Fatalf("URL shape unexpected: %s", cmd.Args[1])
	}
	if !strings.Contains(cmd.Args[1], "@db:5432/app") {
		t.Fatalf("host/db missing: %s", cmd.Args[1])
	}
}

func TestBuildRejectsUnknownEngine(t *testing.T) {
	d := &config.Database{Name: "x", Engine: "bogus"}
	if _, err := Build(d, "", 0, ""); err == nil {
		t.Fatal("expected error for unknown engine")
	}
}

func TestRequiresTunnelTarget(t *testing.T) {
	if RequiresTunnelTarget(config.EngineSQLite) {
		t.Fatal("sqlite should not require tunnel target")
	}
	for _, e := range []config.DBEngine{config.EnginePostgres, config.EngineMySQL, config.EngineRedis} {
		if !RequiresTunnelTarget(e) {
			t.Fatalf("%s should require tunnel target", e)
		}
	}
}
