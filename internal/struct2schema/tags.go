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
package struct2schema

import (
	"reflect"
	"strconv"
	"strings"
)

// TagInfo contains parsed information from struct field tags
type TagInfo struct {
	ColumnName string
	Type       string
	Length     int
	Precision  int
	Scale      int
	PrimaryKey bool
	Nullable   *bool
	Default    string
	AutoCreate bool
	AutoUpdate bool
	ForeignKey *ForeignKeyInfo
	ManyToMany *ManyToManyInfo
	Index      bool
	Unique     bool
	Ignore     bool
}

// ForeignKeyInfo contains foreign key relationship information
type ForeignKeyInfo struct {
	Table    string
	Column   string
	OnDelete string
	OnUpdate string
}

// ManyToManyInfo contains many-to-many relationship information
type ManyToManyInfo struct {
	Table      string
	JoinTable  string
	ForeignKey string
	References string
}

// TagParser handles parsing of struct field tags
type TagParser struct {
	tagPriority []string
}

// NewTagParser creates a new tag parser with priority ordering
func NewTagParser() *TagParser {
	return &TagParser{
		tagPriority: []string{"db", "sql", "gorm", "bun"}, // Priority order
	}
}

// ParseTags extracts tag information from a struct field tag string
func (tp *TagParser) ParseTags(tagString string) TagInfo {
	tagInfo := TagInfo{}

	if tagString == "" {
		return tagInfo
	}

	// Remove backticks if present
	tagString = strings.Trim(tagString, "`")

	// Parse struct tags using reflect.StructTag
	tag := reflect.StructTag(tagString)

	// Process tags in priority order
	for _, tagKey := range tp.tagPriority {
		if tagValue := tag.Get(tagKey); tagValue != "" {
			tp.parseTagValue(tagKey, tagValue, &tagInfo)
		}
	}

	return tagInfo
}

// parseTagValue parses a specific tag value based on the tag type
func (tp *TagParser) parseTagValue(tagKey, tagValue string, info *TagInfo) {
	switch tagKey {
	case "db":
		tp.parseDBTag(tagValue, info)
	case "sql":
		tp.parseSQLTag(tagValue, info)
	case "gorm":
		tp.parseGORMTag(tagValue, info)
	case "bun":
		tp.parseBunTag(tagValue, info)
	}
}

// parseDBTag parses db struct tags (simple format: db:"column_name" or db:"-")
func (tp *TagParser) parseDBTag(value string, info *TagInfo) {
	if value == "-" {
		info.Ignore = true
		return
	}

	// Simple column name
	if info.ColumnName == "" {
		info.ColumnName = value
	}
}

// parseSQLTag parses sql struct tags
func (tp *TagParser) parseSQLTag(value string, info *TagInfo) {
	if value == "-" {
		info.Ignore = true
		return
	}

	// Parse comma-separated options
	parts := strings.Split(value, ",")
	for i, part := range parts {
		part = strings.TrimSpace(part)
		if i == 0 && part != "" && info.ColumnName == "" {
			info.ColumnName = part
		} else {
			tp.parseTagOption(part, info)
		}
	}
}

// parseGORMTag parses GORM struct tags (complex format with semicolons)
func (tp *TagParser) parseGORMTag(value string, info *TagInfo) {
	if value == "-" {
		info.Ignore = true
		return
	}

	// Parse semicolon-separated options
	parts := strings.Split(value, ";")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// Handle key:value pairs
		if strings.Contains(part, ":") {
			kv := strings.SplitN(part, ":", 2)
			key := strings.TrimSpace(kv[0])
			val := strings.TrimSpace(kv[1])
			tp.parseGORMOption(key, val, info)
		} else {
			// Handle standalone options
			tp.parseGORMOption(part, "", info)
		}
	}
}

// parseBunTag parses Bun ORM struct tags
func (tp *TagParser) parseBunTag(value string, info *TagInfo) {
	if value == "-" {
		info.Ignore = true
		return
	}

	// Parse comma-separated options (similar to sql tag)
	parts := strings.Split(value, ",")
	for i, part := range parts {
		part = strings.TrimSpace(part)
		if i == 0 && part != "" && info.ColumnName == "" {
			info.ColumnName = part
		} else {
			tp.parseTagOption(part, info)
		}
	}
}

// parseGORMOption parses individual GORM tag options
func (tp *TagParser) parseGORMOption(key, value string, info *TagInfo) {
	switch strings.ToLower(key) {
	case "column":
		if info.ColumnName == "" {
			info.ColumnName = value
		}
	case "type":
		if info.Type == "" {
			info.Type = value
		}
	case "size":
		if info.Length == 0 {
			if size, err := strconv.Atoi(value); err == nil {
				info.Length = size
			}
		}
	case "precision":
		if info.Precision == 0 {
			if precision, err := strconv.Atoi(value); err == nil {
				info.Precision = precision
			}
		}
	case "scale":
		if info.Scale == 0 {
			if scale, err := strconv.Atoi(value); err == nil {
				info.Scale = scale
			}
		}
	case "primarykey", "primary_key":
		info.PrimaryKey = true
	case "not null", "notnull":
		nullable := false
		info.Nullable = &nullable
	case "null":
		nullable := true
		info.Nullable = &nullable
	case "default":
		if info.Default == "" {
			info.Default = value
		}
	case "autocreate", "auto_create", "autocreatetime":
		info.AutoCreate = true
	case "autoupdate", "auto_update", "autoupdatetime":
		info.AutoUpdate = true
	case "foreignkey", "foreign_key":
		if info.ForeignKey == nil {
			info.ForeignKey = &ForeignKeyInfo{
				Table: value,
			}
		}
	case "references":
		if info.ForeignKey != nil {
			info.ForeignKey.Column = value
		}
	case "constraint":
		// Parse constraint options like "OnDelete:CASCADE"
		tp.parseConstraint(value, info)
	case "many2many":
		if info.ManyToMany == nil {
			info.ManyToMany = &ManyToManyInfo{
				JoinTable: value,
			}
		}
	case "joinforeign_key", "joinforeignkey":
		if info.ManyToMany != nil {
			info.ManyToMany.ForeignKey = value
		}
	case "joinreferences":
		if info.ManyToMany != nil {
			info.ManyToMany.References = value
		}
	case "index":
		info.Index = true
	case "unique":
		info.Unique = true
	case "uniqueindex", "unique_index":
		info.Index = true
		info.Unique = true
	}
}

// parseTagOption parses generic tag options
func (tp *TagParser) parseTagOption(option string, info *TagInfo) {
	switch strings.ToLower(option) {
	case "primary_key", "pk":
		info.PrimaryKey = true
	case "not null", "notnull":
		nullable := false
		info.Nullable = &nullable
	case "null":
		nullable := true
		info.Nullable = &nullable
	case "auto_increment", "autoincrement":
		info.AutoCreate = true
	case "index":
		info.Index = true
	case "unique":
		info.Unique = true
	}
}

// parseConstraint parses constraint specifications
func (tp *TagParser) parseConstraint(constraint string, info *TagInfo) {
	// Parse constraint like "OnDelete:CASCADE,OnUpdate:RESTRICT"
	parts := strings.Split(constraint, ",")
	for _, part := range parts {
		if strings.Contains(part, ":") {
			kv := strings.SplitN(part, ":", 2)
			key := strings.TrimSpace(kv[0])
			val := strings.TrimSpace(kv[1])

			switch strings.ToLower(key) {
			case "ondelete":
				if info.ForeignKey != nil {
					info.ForeignKey.OnDelete = strings.ToUpper(val)
				}
			case "onupdate":
				if info.ForeignKey != nil {
					info.ForeignKey.OnUpdate = strings.ToUpper(val)
				}
			}
		}
	}
}

// GetTableName extracts table name from struct tags
func (tp *TagParser) GetTableName(tags map[string]string) string {
	// Check for table name in priority order
	for _, tagKey := range tp.tagPriority {
		if tagValue, exists := tags[tagKey]; exists {
			if strings.HasPrefix(tagValue, "tableName:") {
				return strings.TrimPrefix(tagValue, "tableName:")
			}
		}
	}
	return ""
}
