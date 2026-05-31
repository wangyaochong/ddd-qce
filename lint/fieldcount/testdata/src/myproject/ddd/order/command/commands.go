package command

type BaseCommand struct{}

func (BaseCommand) isCommand() {}

type ValidCommand struct {
	BaseCommand
	Field1 string
	Field2 int
	Field3 bool
	Field4 float64
	Field5 []string
}

type TooManyFieldsCommand struct { // want "dddfieldcount"
	BaseCommand
	Field1 string
	Field2 int
	Field3 bool
	Field4 float64
	Field5 []string
	Field6 string
}

type EmbeddedCountsAsOneCommand struct {
	BaseCommand
	Details struct {
		SubField1 string
		SubField2 string
		SubField3 string
	}
	Field2 string
	Field3 string
}

type FiveFieldsCommand struct {
	BaseCommand
	Field1 string
	Field2 int
	Field3 bool
	Field4 float64
	Field5 []string
}

type ValidResult struct {
	Field1 string
	Field2 int
	Field3 bool
	Field4 float64
	Field5 []string
	Field6 string
}
