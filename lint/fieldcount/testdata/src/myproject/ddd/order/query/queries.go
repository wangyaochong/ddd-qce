package query

type BaseQuery struct{}

func (BaseQuery) isQuery() {}

type ValidQuery struct {
	BaseQuery
	Field1 string
	Field2 int
	Field3 bool
	Field4 float64
	Field5 []string
}

type TooManyFieldsQuery struct { // want "dddfieldcount"
	BaseQuery
	Field1 string
	Field2 int
	Field3 bool
	Field4 float64
	Field5 []string
	Field6 string
}

type FourFieldsQuery struct {
	BaseQuery
	Field1 string
	Field2 int
	Field3 bool
	Field4 float64
}

type ValidResult struct {
	Field1 string
	Field2 int
	Field3 bool
	Field4 float64
	Field5 []string
	Field6 string
	Field7 string
}
