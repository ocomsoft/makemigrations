package struct2schema

import (
	"testing"
)

// TestNewTagParser verifies that a TagParser is created with the correct priority order.
func TestNewTagParser(t *testing.T) {
	t.Parallel()

	tp := NewTagParser()
	if tp == nil {
		t.Fatal("NewTagParser returned nil")
	}
	expected := []string{"db", "sql", "gorm", "bun"}
	if len(tp.tagPriority) != len(expected) {
		t.Fatalf("tagPriority length = %d, want %d", len(tp.tagPriority), len(expected))
	}
	for i, v := range expected {
		if tp.tagPriority[i] != v {
			t.Errorf("tagPriority[%d] = %q, want %q", i, tp.tagPriority[i], v)
		}
	}
}

// TestParseTagsEmpty verifies that empty tags return zero-value TagInfo.
func TestParseTagsEmpty(t *testing.T) {
	t.Parallel()

	tp := NewTagParser()
	info := tp.ParseTags("")
	if info.ColumnName != "" || info.Type != "" || info.Ignore {
		t.Error("empty tag should produce zero-value TagInfo")
	}
}

// TestParseDBTag verifies db tag parsing.
func TestParseDBTag(t *testing.T) {
	t.Parallel()

	tp := NewTagParser()

	tests := []struct {
		name       string
		tag        string
		wantCol    string
		wantIgnore bool
	}{
		{name: "simple column name", tag: `db:"user_name"`, wantCol: "user_name"},
		{name: "ignore dash", tag: `db:"-"`, wantIgnore: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			info := tp.ParseTags(tt.tag)
			if info.ColumnName != tt.wantCol {
				t.Errorf("ColumnName = %q, want %q", info.ColumnName, tt.wantCol)
			}
			if info.Ignore != tt.wantIgnore {
				t.Errorf("Ignore = %v, want %v", info.Ignore, tt.wantIgnore)
			}
		})
	}
}

// TestParseSQLTag verifies sql tag parsing with options.
func TestParseSQLTag(t *testing.T) {
	t.Parallel()

	tp := NewTagParser()

	tests := []struct {
		name       string
		tag        string
		wantCol    string
		wantPK     bool
		wantIgnore bool
	}{
		{name: "column with pk", tag: `sql:"my_col,pk"`, wantCol: "my_col", wantPK: true},
		{name: "column only", tag: `sql:"email"`, wantCol: "email"},
		{name: "ignore", tag: `sql:"-"`, wantIgnore: true},
		{name: "column with null", tag: `sql:"name,null"`, wantCol: "name"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			info := tp.ParseTags(tt.tag)
			if info.ColumnName != tt.wantCol {
				t.Errorf("ColumnName = %q, want %q", info.ColumnName, tt.wantCol)
			}
			if info.PrimaryKey != tt.wantPK {
				t.Errorf("PrimaryKey = %v, want %v", info.PrimaryKey, tt.wantPK)
			}
			if info.Ignore != tt.wantIgnore {
				t.Errorf("Ignore = %v, want %v", info.Ignore, tt.wantIgnore)
			}
		})
	}
}

// TestParseGORMTag verifies GORM tag parsing with semicolon-separated options.
func TestParseGORMTag(t *testing.T) {
	t.Parallel()

	tp := NewTagParser()

	tests := []struct {
		name       string
		tag        string
		wantCol    string
		wantType   string
		wantLen    int
		wantPK     bool
		wantIdx    bool
		wantUnique bool
		wantIgnore bool
	}{
		{
			name:    "column and type",
			tag:     `gorm:"column:user_name;type:varchar"`,
			wantCol: "user_name", wantType: "varchar",
		},
		{
			name:    "size option",
			tag:     `gorm:"column:title;size:100"`,
			wantCol: "title", wantLen: 100,
		},
		{
			name:   "primaryKey",
			tag:    `gorm:"primaryKey"`,
			wantPK: true,
		},
		{
			name:   "primary_key underscore",
			tag:    `gorm:"primary_key"`,
			wantPK: true,
		},
		{
			name:    "index",
			tag:     `gorm:"index"`,
			wantIdx: true,
		},
		{
			name:       "unique",
			tag:        `gorm:"unique"`,
			wantUnique: true,
		},
		{
			name:       "uniqueIndex",
			tag:        `gorm:"uniqueIndex"`,
			wantIdx:    true,
			wantUnique: true,
		},
		{
			name:       "ignore",
			tag:        `gorm:"-"`,
			wantIgnore: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			info := tp.ParseTags(tt.tag)
			if info.ColumnName != tt.wantCol {
				t.Errorf("ColumnName = %q, want %q", info.ColumnName, tt.wantCol)
			}
			if info.Type != tt.wantType {
				t.Errorf("Type = %q, want %q", info.Type, tt.wantType)
			}
			if info.Length != tt.wantLen {
				t.Errorf("Length = %d, want %d", info.Length, tt.wantLen)
			}
			if info.PrimaryKey != tt.wantPK {
				t.Errorf("PrimaryKey = %v, want %v", info.PrimaryKey, tt.wantPK)
			}
			if info.Index != tt.wantIdx {
				t.Errorf("Index = %v, want %v", info.Index, tt.wantIdx)
			}
			if info.Unique != tt.wantUnique {
				t.Errorf("Unique = %v, want %v", info.Unique, tt.wantUnique)
			}
			if info.Ignore != tt.wantIgnore {
				t.Errorf("Ignore = %v, want %v", info.Ignore, tt.wantIgnore)
			}
		})
	}
}

// TestParseGORMPrecisionScale verifies precision and scale parsing in GORM tags.
func TestParseGORMPrecisionScale(t *testing.T) {
	t.Parallel()

	tp := NewTagParser()
	info := tp.ParseTags(`gorm:"precision:10;scale:2"`)
	if info.Precision != 10 {
		t.Errorf("Precision = %d, want 10", info.Precision)
	}
	if info.Scale != 2 {
		t.Errorf("Scale = %d, want 2", info.Scale)
	}
}

// TestParseGORMDefault verifies default value parsing.
func TestParseGORMDefault(t *testing.T) {
	t.Parallel()

	tp := NewTagParser()
	info := tp.ParseTags(`gorm:"default:now()"`)
	if info.Default != "now()" {
		t.Errorf("Default = %q, want %q", info.Default, "now()")
	}
}

// TestParseGORMNullability verifies null/not-null parsing in GORM tags.
func TestParseGORMNullability(t *testing.T) {
	t.Parallel()

	tp := NewTagParser()

	t.Run("not null", func(t *testing.T) {
		t.Parallel()
		info := tp.ParseTags(`gorm:"not null"`)
		if info.Nullable == nil || *info.Nullable != false {
			t.Error("'not null' should set Nullable to false")
		}
	})

	t.Run("null", func(t *testing.T) {
		t.Parallel()
		info := tp.ParseTags(`gorm:"null"`)
		if info.Nullable == nil || *info.Nullable != true {
			t.Error("'null' should set Nullable to true")
		}
	})
}

// TestParseGORMAutoTimestamps verifies auto-create/update time tags.
func TestParseGORMAutoTimestamps(t *testing.T) {
	t.Parallel()

	tp := NewTagParser()

	tests := []struct {
		tag            string
		wantAutoCreate bool
		wantAutoUpdate bool
	}{
		{tag: `gorm:"autoCreateTime"`, wantAutoCreate: true},
		{tag: `gorm:"autoUpdateTime"`, wantAutoUpdate: true},
		{tag: `gorm:"auto_create"`, wantAutoCreate: true},
		{tag: `gorm:"auto_update"`, wantAutoUpdate: true},
	}

	for _, tt := range tests {
		t.Run(tt.tag, func(t *testing.T) {
			t.Parallel()
			info := tp.ParseTags(tt.tag)
			if info.AutoCreate != tt.wantAutoCreate {
				t.Errorf("AutoCreate = %v, want %v", info.AutoCreate, tt.wantAutoCreate)
			}
			if info.AutoUpdate != tt.wantAutoUpdate {
				t.Errorf("AutoUpdate = %v, want %v", info.AutoUpdate, tt.wantAutoUpdate)
			}
		})
	}
}

// TestParseGORMForeignKey verifies foreign key tag parsing.
func TestParseGORMForeignKey(t *testing.T) {
	t.Parallel()

	tp := NewTagParser()
	info := tp.ParseTags(`gorm:"foreignKey:UserID;references:ID"`)

	if info.ForeignKey == nil {
		t.Fatal("ForeignKey should not be nil")
	}
	if info.ForeignKey.Table != "UserID" {
		t.Errorf("ForeignKey.Table = %q, want %q", info.ForeignKey.Table, "UserID")
	}
	if info.ForeignKey.Column != "ID" {
		t.Errorf("ForeignKey.Column = %q, want %q", info.ForeignKey.Column, "ID")
	}
}

// TestParseGORMConstraint verifies constraint parsing (OnDelete, OnUpdate).
func TestParseGORMConstraint(t *testing.T) {
	t.Parallel()

	tp := NewTagParser()
	info := tp.ParseTags(`gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE,OnUpdate:RESTRICT"`)

	if info.ForeignKey == nil {
		t.Fatal("ForeignKey should not be nil")
	}
	if info.ForeignKey.OnDelete != "CASCADE" {
		t.Errorf("OnDelete = %q, want %q", info.ForeignKey.OnDelete, "CASCADE")
	}
	if info.ForeignKey.OnUpdate != "RESTRICT" {
		t.Errorf("OnUpdate = %q, want %q", info.ForeignKey.OnUpdate, "RESTRICT")
	}
}

// TestParseGORMManyToMany verifies many-to-many tag parsing.
func TestParseGORMManyToMany(t *testing.T) {
	t.Parallel()

	tp := NewTagParser()
	info := tp.ParseTags(`gorm:"many2many:user_roles;joinForeignKey:UserID;joinReferences:RoleID"`)

	if info.ManyToMany == nil {
		t.Fatal("ManyToMany should not be nil")
	}
	if info.ManyToMany.JoinTable != "user_roles" {
		t.Errorf("JoinTable = %q, want %q", info.ManyToMany.JoinTable, "user_roles")
	}
	if info.ManyToMany.ForeignKey != "UserID" {
		t.Errorf("ForeignKey = %q, want %q", info.ManyToMany.ForeignKey, "UserID")
	}
	if info.ManyToMany.References != "RoleID" {
		t.Errorf("References = %q, want %q", info.ManyToMany.References, "RoleID")
	}
}

// TestParseBunTag verifies bun tag parsing.
func TestParseBunTag(t *testing.T) {
	t.Parallel()

	tp := NewTagParser()

	tests := []struct {
		name    string
		tag     string
		wantCol string
		wantPK  bool
	}{
		{name: "column name", tag: `bun:"user_name"`, wantCol: "user_name"},
		{name: "column with pk", tag: `bun:"id,pk"`, wantCol: "id", wantPK: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			info := tp.ParseTags(tt.tag)
			if info.ColumnName != tt.wantCol {
				t.Errorf("ColumnName = %q, want %q", info.ColumnName, tt.wantCol)
			}
			if info.PrimaryKey != tt.wantPK {
				t.Errorf("PrimaryKey = %v, want %v", info.PrimaryKey, tt.wantPK)
			}
		})
	}
}

// TestTagPriority verifies that earlier tags take priority for column naming.
// The priority order is: db, sql, gorm, bun.
func TestTagPriority(t *testing.T) {
	t.Parallel()

	tp := NewTagParser()

	// db takes priority over gorm for column name
	info := tp.ParseTags(`db:"db_col" gorm:"column:gorm_col"`)
	if info.ColumnName != "db_col" {
		t.Errorf("db should take priority: got %q, want %q", info.ColumnName, "db_col")
	}

	// sql takes priority over gorm for column name
	info2 := tp.ParseTags(`sql:"sql_col" gorm:"column:gorm_col"`)
	if info2.ColumnName != "sql_col" {
		t.Errorf("sql should take priority over gorm: got %q, want %q", info2.ColumnName, "sql_col")
	}
}

// TestParseTagsWithBackticks verifies that backticks are stripped properly.
func TestParseTagsWithBackticks(t *testing.T) {
	t.Parallel()

	tp := NewTagParser()
	info := tp.ParseTags("`db:\"my_col\"`")
	if info.ColumnName != "my_col" {
		t.Errorf("ColumnName = %q, want %q", info.ColumnName, "my_col")
	}
}

// TestGetTableName verifies table name extraction from struct-level tags.
func TestGetTableName(t *testing.T) {
	t.Parallel()

	tp := NewTagParser()

	tests := []struct {
		name string
		tags map[string]string
		want string
	}{
		{
			name: "db tableName",
			tags: map[string]string{"db": "tableName:users"},
			want: "users",
		},
		{
			name: "bun tableName",
			tags: map[string]string{"bun": "tableName:accounts"},
			want: "accounts",
		},
		{
			name: "no tableName",
			tags: map[string]string{"db": "some_value"},
			want: "",
		},
		{
			name: "empty tags",
			tags: map[string]string{},
			want: "",
		},
		{
			name: "priority db over bun",
			tags: map[string]string{
				"db":  "tableName:db_table",
				"bun": "tableName:bun_table",
			},
			want: "db_table",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := tp.GetTableName(tt.tags)
			if got != tt.want {
				t.Errorf("GetTableName() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestParseSQLTagOptions verifies generic tag options in sql tags.
func TestParseSQLTagOptions(t *testing.T) {
	t.Parallel()

	tp := NewTagParser()

	info := tp.ParseTags(`sql:"amount,unique,index"`)
	if info.ColumnName != "amount" {
		t.Errorf("ColumnName = %q, want %q", info.ColumnName, "amount")
	}
	if !info.Unique {
		t.Error("Unique should be true")
	}
	if !info.Index {
		t.Error("Index should be true")
	}
}
