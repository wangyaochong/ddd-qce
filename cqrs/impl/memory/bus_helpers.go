package memory

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"sync/atomic"
)

var ErrBusClosed = errors.New("bus is closed")

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

func typeNamesFromMap(m map[reflect.Type]any) []string {
	names := make([]string, 0, len(m))
	for t := range m {
		name := t.Name()
		if t.Kind() == reflect.Ptr {
			name = t.Elem().Name()
		}
		names = append(names, name)
	}
	return names
}

func shutdownBus(closed *atomic.Bool, inFlight *sync.WaitGroup, ctx context.Context) error {
	closed.Store(true)
	done := make(chan struct{})
	go func() {
		inFlight.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
