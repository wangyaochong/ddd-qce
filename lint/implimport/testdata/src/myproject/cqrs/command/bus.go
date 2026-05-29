package command

type CommandBus interface {
	RegisteredTypes() []string
}
