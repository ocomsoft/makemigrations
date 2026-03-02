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

// NewProvider creates a new database provider based on the database type.
// If typeMappings is non-nil, it is applied to the provider via SetTypeMappings.
func NewProvider(dbType types.DatabaseType, typeMappings map[string]string) (Provider, error) {
	var p Provider
	switch dbType {
	case types.DatabasePostgreSQL:
		p = postgresql.New()
	case types.DatabaseMySQL:
		p = mysql.New()
	case types.DatabaseSQLite:
		p = sqlite.New()
	case types.DatabaseSQLServer:
		p = sqlserver.New()
	case types.DatabaseRedshift:
		p = redshift.New()
	case types.DatabaseClickHouse:
		p = clickhouse.New()
	case types.DatabaseTiDB:
		p = tidb.New()
	case types.DatabaseVertica:
		p = vertica.New()
	case types.DatabaseYDB:
		p = ydb.New()
	case types.DatabaseTurso:
		p = turso.New()
	case types.DatabaseStarRocks:
		p = starrocks.New()
	case types.DatabaseAuroraDSQL:
		p = auroradsql.New()
	default:
		return nil, fmt.Errorf("unsupported database type: %s", dbType)
	}
	if typeMappings != nil {
		p.SetTypeMappings(typeMappings)
	}
	return p, nil
}
