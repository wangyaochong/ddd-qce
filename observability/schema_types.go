package observability

import "time"

type TableInfo struct {
	Name        string
	Description string
	RowCount    int64
	LastUpdated *time.Time
	DiskSize    int64
	AvgRowSize  int64
}

type ColumnInfo struct {
	Name         string
	Type         string
	Nullable     bool
	DefaultValue string
	IsPrimaryKey bool
	Description  string
}

type IndexInfo struct {
	Name    string
	Columns []string
	Unique  bool
}

type TableDetail struct {
	TableInfo
	Columns []ColumnInfo
	Indexes []IndexInfo
}

type TableRelation struct {
	FromTable  string
	FromColumn string
	ToTable    string
	ToColumn   string
}
