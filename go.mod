module github.com/ayMissouri/noted

go 1.26.0

toolchain go1.26.5

require modernc.org/sqlite v1.49.1

require (
	github.com/dustin/go-humanize v1.0.1
	github.com/google/uuid v1.6.0
	github.com/mattn/go-isatty v0.0.20
	github.com/ncruces/go-strftime v1.0.0
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec
	golang.org/x/sys v0.43.0
	modernc.org/libc v1.72.0
	modernc.org/mathutil v1.7.1
	modernc.org/memory v1.11.0
)

require (
	cel.dev/expr v0.25.1
	filippo.io/edwards25519 v1.1.1
	github.com/antlr4-go/antlr/v4 v4.13.1
	github.com/coreos/go-semver v0.3.1
	github.com/cubicdaiya/gonp v1.0.4
	github.com/davecgh/go-spew v1.1.1
	github.com/fatih/structtag v1.2.0
	github.com/go-sql-driver/mysql v1.9.3
	github.com/google/cel-go v0.28.0
	github.com/inconshreveable/mousetrap v1.1.0
	github.com/jackc/pgpassfile v1.0.0
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761
	github.com/jackc/pgx/v5 v5.9.2
	github.com/jackc/puddle/v2 v2.2.2
	github.com/jinzhu/inflection v1.0.0
	github.com/ncruces/go-sqlite3 v0.32.0
	github.com/ncruces/julianday v1.0.0
	github.com/pganalyze/pg_query_go/v6 v6.2.2
	github.com/pingcap/errors v0.11.5-0.20250523034308-74f78ae071ee
	github.com/pingcap/failpoint v0.0.0-20240528011301-b51a646c7c86
	github.com/pingcap/log v1.1.0
	github.com/pingcap/tidb/pkg/parser v0.0.0-20260418072757-ce92298d1124
	github.com/riza-io/grpc-go v0.2.0
	github.com/spf13/cobra v1.10.2
	github.com/spf13/pflag v1.0.10
	github.com/sqlc-dev/doubleclick v1.0.0
	github.com/sqlc-dev/sqlc v1.31.1
	github.com/tetratelabs/wazero v1.11.0
	github.com/wasilibs/go-pgquery v0.0.0-20250409022910-10ac41983c07
	github.com/wasilibs/wazero-helpers v0.0.0-20240620070341-3dff1577cd52
	go.uber.org/atomic v1.11.0
	go.uber.org/multierr v1.11.0
	go.uber.org/zap v1.27.0
	golang.org/x/exp v0.0.0-20250620022241-b7579e27df2b
	golang.org/x/net v0.49.0
	golang.org/x/sync v0.20.0
	golang.org/x/text v0.36.0
	google.golang.org/genproto/googleapis/api v0.0.0-20260120221211-b8f7ae30c516
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260120221211-b8f7ae30c516
	google.golang.org/grpc v1.80.0
	google.golang.org/protobuf v1.36.11
	gopkg.in/natefinch/lumberjack.v2 v2.2.1
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/labstack/echo/v5 v5.0.0
	golang.org/x/time v0.14.0
)

require github.com/yuin/goldmark v1.8.2

tool github.com/sqlc-dev/sqlc/cmd/sqlc
