package observability

import (
	"reflect"

	"github.com/ddd-qce/core/cqrs/command"
	"github.com/ddd-qce/core/cqrs/event"
	"github.com/ddd-qce/core/cqrs/query"
	"github.com/ddd-qce/core/domain/domainevent"
)

type BusTypeSampleProvider interface {
	GetCommandSample(typeName string) (any, any)
	GetQuerySample(typeName string) (any, any)
	GetEventSample(typeName string) domainevent.Event
}

func CollectFromBuses(cmdBus command.CommandBus, queryBus query.QueryBus, evtBus event.EventBus, registry *TypePrototypeRegistry, provider BusTypeSampleProvider) {
	CollectFromCommandBus(cmdBus, registry, provider)
	CollectFromQueryBus(queryBus, registry, provider)
	CollectFromEventBus(evtBus, registry, provider)
}

func CollectFromCommandBus(bus command.CommandBus, registry *TypePrototypeRegistry, provider BusTypeSampleProvider) {
	if bus == nil || registry == nil {
		return
	}

	typeNames := bus.RegisteredTypes()
	for _, name := range typeNames {
		if provider != nil {
			sample, resultSample := provider.GetCommandSample(name)
			if sample != nil {
				registry.RegisterFromSample("command", name, sample, resultSample)
				continue
			}
		}
		registry.Register("command", name, nil, "", nil)
	}
}

func CollectFromQueryBus(bus query.QueryBus, registry *TypePrototypeRegistry, provider BusTypeSampleProvider) {
	if bus == nil || registry == nil {
		return
	}

	typeNames := bus.RegisteredTypes()
	for _, name := range typeNames {
		if provider != nil {
			sample, resultSample := provider.GetQuerySample(name)
			if sample != nil {
				registry.RegisterFromSample("query", name, sample, resultSample)
				continue
			}
		}
		registry.Register("query", name, nil, "", nil)
	}
}

func CollectFromEventBus(bus event.EventBus, registry *TypePrototypeRegistry, provider BusTypeSampleProvider) {
	if bus == nil || registry == nil {
		return
	}

	typeNames := bus.SubscribedTypes()
	for _, name := range typeNames {
		if provider != nil {
			sample := provider.GetEventSample(name)
			if sample != nil {
				registry.RegisterFromSample("event", name, sample, nil)
				continue
			}
		}
		registry.Register("event", name, nil, "", nil)
	}
}

type ReflectionSampleProvider struct {
	commandSamples map[string]samplePair
	querySamples   map[string]samplePair
	eventSamples   map[string]any
}

type samplePair struct {
	sample     any
	resultSample any
}

func NewReflectionSampleProvider() *ReflectionSampleProvider {
	return &ReflectionSampleProvider{
		commandSamples: make(map[string]samplePair),
		querySamples:   make(map[string]samplePair),
		eventSamples:   make(map[string]any),
	}
}

func (p *ReflectionSampleProvider) RegisterCommand(name string, sample, resultSample any) {
	p.commandSamples[name] = samplePair{sample: sample, resultSample: resultSample}
}

func (p *ReflectionSampleProvider) RegisterQuery(name string, sample, resultSample any) {
	p.querySamples[name] = samplePair{sample: sample, resultSample: resultSample}
}

func (p *ReflectionSampleProvider) RegisterEvent(name string, sample domainevent.Event) {
	p.eventSamples[name] = sample
}

func (p *ReflectionSampleProvider) GetCommandSample(typeName string) (any, any) {
	if pair, ok := p.commandSamples[typeName]; ok {
		return pair.sample, pair.resultSample
	}
	return nil, nil
}

func (p *ReflectionSampleProvider) GetQuerySample(typeName string) (any, any) {
	if pair, ok := p.querySamples[typeName]; ok {
		return pair.sample, pair.resultSample
	}
	return nil, nil
}

func (p *ReflectionSampleProvider) GetEventSample(typeName string) domainevent.Event {
	if sample, ok := p.eventSamples[typeName]; ok {
		if evt, ok := sample.(domainevent.Event); ok {
			return evt
		}
	}
	return nil
}

func RegisterTypesFromReflect(
	registry *TypePrototypeRegistry,
	commandTypes map[string]reflect.Type,
	commandResultTypes map[string]reflect.Type,
	queryTypes map[string]reflect.Type,
	queryResultTypes map[string]reflect.Type,
	eventTypes map[string]reflect.Type,
) {
	for name, t := range commandTypes {
		resultType := commandResultTypes[name]
		registry.RegisterFromType("command", name, t, resultType)
	}
	for name, t := range queryTypes {
		resultType := queryResultTypes[name]
		registry.RegisterFromType("query", name, t, resultType)
	}
	for name, t := range eventTypes {
		registry.RegisterFromType("event", name, t, nil)
	}
}