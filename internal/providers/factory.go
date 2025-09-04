/*
MIT License

# Copyright (c) 2025 OcomSoft

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
*/
package providers

import (
	"fmt"

	"github.com/ocomsoft/makemigrations/internal/providers/auroradsql"
	"github.com/ocomsoft/makemigrations/internal/providers/clickhouse"
	"github.com/ocomsoft/makemigrations/internal/providers/mysql"
	"github.com/ocomsoft/makemigrations/internal/providers/postgresql"
	"github.com/ocomsoft/makemigrations/internal/providers/redshift"
	"github.com/ocomsoft/makemigrations/internal/providers/sqlite"
	"github.com/ocomsoft/makemigrations/internal/providers/sqlserver"
	"github.com/ocomsoft/makemigrations/internal/providers/starrocks"
	"github.com/ocomsoft/makemigrations/internal/providers/tidb"
	"github.com/ocomsoft/makemigrations/internal/providers/turso"
	"github.com/ocomsoft/makemigrations/internal/providers/vertica"
	"github.com/ocomsoft/makemigrations/internal/providers/ydb"
	"github.com/ocomsoft/makemigrations/internal/types"
)

// NewProvider creates a new database provider based on the database type
func NewProvider(dbType types.DatabaseType) (Provider, error) {
	switch dbType {
	case types.DatabasePostgreSQL:
		return postgresql.New(), nil
	case types.DatabaseMySQL:
		return mysql.New(), nil
	case types.DatabaseSQLite:
		return sqlite.New(), nil
	case types.DatabaseSQLServer:
		return sqlserver.New(), nil
	case types.DatabaseRedshift:
		return redshift.New(), nil
	case types.DatabaseClickHouse:
		return clickhouse.New(), nil
	case types.DatabaseTiDB:
		return tidb.New(), nil
	case types.DatabaseVertica:
		return vertica.New(), nil
	case types.DatabaseYDB:
		return ydb.New(), nil
	case types.DatabaseTurso:
		return turso.New(), nil
	case types.DatabaseStarRocks:
		return starrocks.New(), nil
	case types.DatabaseAuroraDSQL:
		return auroradsql.New(), nil
	default:
		return nil, fmt.Errorf("unsupported database type: %s", dbType)
	}
}
