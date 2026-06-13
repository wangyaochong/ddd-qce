package relationship

import (
	"context"
	"reflect"

	"github.com/ddd-qce/core/cqrs/command"
	cqrsevent "github.com/ddd-qce/core/cqrs/event"
	"github.com/ddd-qce/core/cqrs/query"
	domainevent "github.com/ddd-qce/core/domain/event"
)

type NotifyingCommandBus struct {
	inner command.CommandBus
	reg   *RelationshipRegistry
}

func WrapCommandBus(inner command.CommandBus, reg *RelationshipRegistry) *NotifyingCommandBus {
	return &NotifyingCommandBus{inner: inner, reg: reg}
}

func (b *NotifyingCommandBus) RegisterHandler(handler any) error {
	handlerType := reflect.TypeOf(handler)
	payloadType, ok := extractHandlerPayloadType(handlerType)
	if !ok {
		return b.inner.RegisterHandler(handler)
	}

	commandName := typeName(payloadType)
	handlerName := handlerTypeName(handlerType)
	domain := InferDomainFromPkgPath(typePkgPath(payloadType))

	b.reg.RecordCommandHandler(commandName, handlerName)
	b.reg.RecordTypeDomain(commandName, domain)
	b.reg.RecordTypeDomain(handlerName, domain)

	return b.inner.RegisterHandler(handler)
}

func (b *NotifyingCommandBus) Execute(ctx context.Context, cmd any) (any, error) {
	return b.inner.Execute(ctx, cmd)
}

func (b *NotifyingCommandBus) RegisteredTypes() []string {
	return b.inner.RegisteredTypes()
}

func (b *NotifyingCommandBus) Shutdown(ctx context.Context) error {
	return b.inner.Shutdown(ctx)
}

type NotifyingQueryBus struct {
	inner query.QueryBus
	reg   *RelationshipRegistry
}

func WrapQueryBus(inner query.QueryBus, reg *RelationshipRegistry) *NotifyingQueryBus {
	return &NotifyingQueryBus{inner: inner, reg: reg}
}

func (b *NotifyingQueryBus) RegisterHandler(handler any) error {
	handlerType := reflect.TypeOf(handler)
	payloadType, ok := extractHandlerPayloadType(handlerType)
	if !ok {
		return b.inner.RegisterHandler(handler)
	}

	queryName := typeName(payloadType)
	handlerName := handlerTypeName(handlerType)
	domain := InferDomainFromPkgPath(typePkgPath(payloadType))

	b.reg.RecordQueryHandler(queryName, handlerName)
	b.reg.RecordTypeDomain(queryName, domain)
	b.reg.RecordTypeDomain(handlerName, domain)

	return b.inner.RegisterHandler(handler)
}

func (b *NotifyingQueryBus) Execute(ctx context.Context, q any) (any, error) {
	return b.inner.Execute(ctx, q)
}

func (b *NotifyingQueryBus) RegisteredTypes() []string {
	return b.inner.RegisteredTypes()
}

func (b *NotifyingQueryBus) Shutdown(ctx context.Context) error {
	return b.inner.Shutdown(ctx)
}

type NotifyingEventBus struct {
	inner cqrsevent.EventBus
	reg   *RelationshipRegistry
}

func WrapEventBus(inner cqrsevent.EventBus, reg *RelationshipRegistry) *NotifyingEventBus {
	return &NotifyingEventBus{inner: inner, reg: reg}
}

func (b *NotifyingEventBus) SubscribeHandler(handler any) error {
	handlerType := reflect.TypeOf(handler)
	payloadType, ok := extractHandlerPayloadType(handlerType)
	if !ok {
		return b.inner.SubscribeHandler(handler)
	}

	eventName := typeName(payloadType)
	handlerName := handlerTypeName(handlerType)
	domain := InferDomainFromPkgPath(typePkgPath(payloadType))

	b.reg.RecordEventHandler(eventName, handlerName)
	b.reg.RecordTypeDomain(eventName, domain)
	b.reg.RecordTypeDomain(handlerName, domain)

	return b.inner.SubscribeHandler(handler)
}

func (b *NotifyingEventBus) Publish(ctx context.Context, evt domainevent.Event) error {
	return b.inner.Publish(ctx, evt)
}

func (b *NotifyingEventBus) SubscribedTypes() []string {
	return b.inner.SubscribedTypes()
}

func (b *NotifyingEventBus) Shutdown(ctx context.Context) error {
	return b.inner.Shutdown(ctx)
}

func extractHandlerPayloadType(handlerType reflect.Type) (reflect.Type, bool) {
	if handlerType.Kind() != reflect.Ptr {
		for i := 0; i < handlerType.NumMethod(); i++ {
			method := handlerType.Method(i)
			if method.Name != "Handle" {
				continue
			}
			return extractPayloadFromHandleMethod(method.Type), true
		}
		return nil, false
	}

	handleMethod, ok := handlerType.MethodByName("Handle")
	if !ok {
		return nil, false
	}
	pt := extractPayloadFromHandleMethod(handleMethod.Type)
	if pt == nil {
		return nil, false
	}
	return pt, true
}

func extractPayloadFromHandleMethod(methodType reflect.Type) reflect.Type {
	if methodType.NumIn() != 3 {
		return nil
	}
	return methodType.In(2)
}

func handlerTypeName(t reflect.Type) string {
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t.Name()
}

func typeName(t reflect.Type) string {
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t.Name()
}

func typePkgPath(t reflect.Type) string {
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t.PkgPath()
}