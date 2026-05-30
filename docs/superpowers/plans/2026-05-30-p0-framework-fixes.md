# P0 Framework Fixes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix two P0 issues: (1) EventStore shallowCopy safety + Append key consistency, (2) ValueObject validator registry for JSON deserialization immutability.

**Architecture:** Task 1 removes the unsafe `shallowCopy` code path from memory EventStore, requires `WithFactory` for interface types (matching pg impl), and fixes the Append map key inconsistency. Task 2 adds a global validator registry to ValueObject so that validators survive JSON round-trips.

**Tech Stack:** Go 1.26, no new dependencies.

---

### Task 1: EventStore shallowCopy fix + Append key consistency

**Files:**
- Modify: `cqrs/impl/memory/event_store.go`
- Modify: `cqrs/impl/memory/event_store_test.go`

- [ ] **Step 1: Remove `shallowCopy` field and all branches in `event_store.go`**

In `EventSourceStore` struct, delete the `shallowCopy bool` field.

In `NewEventSourceStore`, when `t == nil` (interface type), instead of setting `s.shallowCopy = true`, require `WithFactory`:
```go
if t == nil {
    for _, opt := range opts {
        opt(s)
    }
    if s.newFunc == nil {
        return nil, fmt.Errorf("EventSourceStore[T]: WithFactory is required when T is an interface type")
    }
    return s, nil
}
```

In `Append`, remove the `shallowCopy` branch — always deep copy:
```go
for _, evt := range events {
    if aggregateID != evt.AggregateID() {
        return fmt.Errorf("EventSourceStore.Append: aggregateID parameter (%s) does not match evt.AggregateID() (%s)", aggregateID, evt.AggregateID())
    }
    copied, err := s.copyEvent(evt)
    if err != nil {
        return fmt.Errorf("copy event: %w", err)
    }
    s.events[aggregateID] = append(s.events[aggregateID], copied)
    s.nextPosition++
    s.globalEvents = append(s.globalEvents, globalEntry[T]{
        position: s.nextPosition,
        event:    copied,
    })
}
```

In `Load`, remove the `shallowCopy` branch — always deep copy:
```go
slice := events[afterVersion:]
result := make([]T, len(slice))
for i, e := range slice {
    copied, err := s.copyEvent(e)
    if err != nil {
        return nil, fmt.Errorf("copy event at index %d: %w", i, err)
    }
    result[i] = copied
}
return result, nil
```

In `LoadAll`, remove the `shallowCopy` branch — always deep copy:
```go
for i := startIdx; i < endIdx; i++ {
    entry := s.globalEvents[i]
    copied, err := s.copyEvent(entry.event)
    if err != nil {
        return nil, fmt.Errorf("copy event at position %d: %w", entry.position, err)
    }
    result[i-startIdx] = event.GlobalEvent[T]{
        Position: entry.position,
        Event:    copied,
    }
}
```

- [ ] **Step 2: Add tests for the new behaviors in `event_store_test.go`**

Add test for interface type requiring `WithFactory`:
```go
func TestEventStore_InterfaceTypeRequiresFactory(t *testing.T) {
    _, err := NewEventSourceStore[event.BaseEvent]()
    if err == nil {
        t.Fatal("expected error when T is interface type without WithFactory")
    }
    if err.Error() != "EventSourceStore[T]: WithFactory is required when T is an interface type" {
        t.Errorf("unexpected error message: %v", err)
    }
}
```

Add test for Append aggregateID mismatch:
```go
func TestEventStore_Append_AggregateIDMismatch(t *testing.T) {
    store, err := NewEventSourceStore[*testStoreEvent]()
    if err != nil {
        t.Fatalf("create event store: %v", err)
    }
    ctx := context.Background()

    events := []*testStoreEvent{
        {BaseEvent: event.NewBaseEvent("wrong-id", time.Now()), Data: "e1"},
    }

    err = store.Append(ctx, "agg-1", 0, events)
    if err == nil {
        t.Fatal("expected error for aggregateID mismatch")
    }
    if !strings.Contains(err.Error(), "aggregateID parameter") || !strings.Contains(err.Error(), "does not match") {
        t.Errorf("unexpected error message: %v", err)
    }
}
```

Add `"strings"` to imports.

- [ ] **Step 3: Run tests**

Run: `go test ./cqrs/impl/memory/... -v`
Expected: All tests PASS, including new tests.

- [ ] **Step 4: Run full build**

Run: `go build ./...`
Expected: Success, no compilation errors.

---

### Task 2: ValueObject immutability (validator registry)

**Files:**
- Modify: `domain/valueobject/valueobject.go`
- Modify: `domain/valueobject/valueobject_test.go`

- [ ] **Step 1: Add validator registry and `validatorName` field to `valueobject.go`**

Add `sync` import. Add the registry and registration function after the imports:
```go
var validatorRegistry = &struct {
	mu   sync.RWMutex
	regs map[string]any
}{regs: make(map[string]any)}

func RegisterValidator[T comparable](name string, validate func(T) error) {
	validatorRegistry.mu.Lock()
	defer validatorRegistry.mu.Unlock()
	validatorRegistry.regs[name] = validate
}

func lookupValidator[T comparable](name string) (func(T) error, bool) {
	validatorRegistry.mu.RLock()
	defer validatorRegistry.mu.RUnlock()
	v, ok := validatorRegistry.regs[name]
	if !ok {
		return nil, false
	}
	fn, ok := v.(func(T) error)
	return fn, ok
}
```

Add `validatorName` field to `ValueObject[T]`:
```go
type ValueObject[T comparable] struct {
	value         T
	validate      func(T) error
	validatorName string
}
```

- [ ] **Step 2: Update `New` and `MustNew` to accept validator name via options**

Add option type:
```go
type ValueObjectOption[T comparable] func(*ValueObject[T])

func WithValidatorName[T comparable](name string) ValueObjectOption[T] {
	return func(vo *ValueObject[T]) { vo.validatorName = name }
}
```

Update `New`:
```go
func New[T comparable](value T, validate func(T) error, opts ...ValueObjectOption[T]) (ValueObject[T], error) {
	if validate != nil {
		if err := validate(value); err != nil {
			return ValueObject[T]{}, err
		}
	}
	vo := ValueObject[T]{value: value, validate: validate}
	for _, opt := range opts {
		opt(&vo)
	}
	return vo, nil
}
```

Update `MustNew`:
```go
func MustNew[T comparable](value T, validate func(T) error, opts ...ValueObjectOption[T]) ValueObject[T] {
	vo, err := New(value, validate, opts...)
	if err != nil {
		panic(err)
	}
	return vo
}
```

- [ ] **Step 3: Update `MarshalJSON` to include validator name**

```go
func (v ValueObject[T]) MarshalJSON() ([]byte, error) {
	if v.validatorName != "" {
		return json.Marshal(struct {
			Value         T      `json:"value"`
			ValidatorName string `json:"_validator,omitempty"`
		}{Value: v.value, ValidatorName: v.validatorName})
	}
	return json.Marshal(struct {
		Value T `json:"value"`
	}{Value: v.value})
}
```

- [ ] **Step 4: Update `UnmarshalJSON` to recover validator from registry**

```go
func (v *ValueObject[T]) UnmarshalJSON(data []byte) error {
	var aux struct {
		Value         T      `json:"value"`
		ValidatorName string `json:"_validator"`
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	v.value = aux.Value
	v.validatorName = aux.ValidatorName
	if aux.ValidatorName != "" {
		if fn, ok := lookupValidator[T](aux.ValidatorName); ok {
			v.validate = fn
		}
	}
	return v.Validate()
}
```

- [ ] **Step 5: Update tests in `valueobject_test.go`**

All existing calls to `New` and `MustNew` need updating since the signature changed (added variadic `opts`). Since `opts` is variadic, existing calls with 2 args still compile — no changes needed for existing test calls.

Add new tests:
```go
func TestValueObject_ValidatorRegistry_RoundTrip(t *testing.T) {
	RegisterValidator[money]("moneyValidator", validateMoney)

	vo := MustNew(money{Amount: 100, Currency: "USD"}, validateMoney, WithValidatorName[money]("moneyValidator"))
	data, err := json.Marshal(vo)
	if err != nil {
		t.Fatalf("MarshalJSON: %v", err)
	}

	var restored ValueObject[money]
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("UnmarshalJSON: %v", err)
	}

	if !vo.Equals(restored) {
		t.Errorf("round-trip failed: original=%v, restored=%v", vo.Value(), restored.Value())
	}
	if restored.validate == nil {
		t.Error("expected validator to be restored from registry")
	}
	if restored.validatorName != "moneyValidator" {
		t.Errorf("expected validatorName 'moneyValidator', got '%s'", restored.validatorName)
	}

	if err := restored.Validate(); err != nil {
		t.Errorf("Validate() should pass for valid restored value: %v", err)
	}
}

func TestValueObject_ValidatorRegistry_InvalidOnUnmarshal(t *testing.T) {
	RegisterValidator[money]("moneyValidator", validateMoney)

	vo := MustNew(money{Amount: 100, Currency: "USD"}, validateMoney, WithValidatorName[money]("moneyValidator"))
	data, err := json.Marshal(vo)
	if err != nil {
		t.Fatalf("MarshalJSON: %v", err)
	}

	invalidData := `{"value":{"Amount":-1,"Currency":"USD"},"_validator":"moneyValidator"}`
	var restored ValueObject[money]
	if err := json.Unmarshal([]byte(invalidData), &restored); err == nil {
		t.Error("expected validation error for invalid value with registry validator")
	}
}

func TestValueObject_ValidatorRegistry_UnknownName(t *testing.T) {
	data := `{"value":{"Amount":100,"Currency":"USD"},"_validator":"unknown"}`
	var restored ValueObject[money]
	if err := json.Unmarshal([]byte(data), &restored); err == nil {
		t.Error("expected error for zero value without recovered validator")
	}
}

func TestValueObject_MarshalJSON_WithValidatorName(t *testing.T) {
	RegisterValidator[money]("moneyValidator", validateMoney)

	vo := MustNew(money{Amount: 100, Currency: "USD"}, validateMoney, WithValidatorName[money]("moneyValidator"))
	data, err := json.Marshal(vo)
	if err != nil {
		t.Fatalf("MarshalJSON: %v", err)
	}

	var parsed struct {
		Value         money  `json:"value"`
		ValidatorName string `json:"_validator"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if parsed.ValidatorName != "moneyValidator" {
		t.Errorf("expected _validator 'moneyValidator', got '%s'", parsed.ValidatorName)
	}
}

func TestValueObject_MarshalJSON_WithoutValidatorName(t *testing.T) {
	vo, _ := New(money{Amount: 100, Currency: "USD"}, validateMoney)
	data, err := json.Marshal(vo)
	if err != nil {
		t.Fatalf("MarshalJSON: %v", err)
	}

	var parsed struct {
		Value         money  `json:"value"`
		ValidatorName string `json:"_validator"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if parsed.ValidatorName != "" {
		t.Errorf("expected no _validator, got '%s'", parsed.ValidatorName)
	}
}
```

- [ ] **Step 6: Run tests**

Run: `go test ./domain/valueobject/... -v`
Expected: All tests PASS.

- [ ] **Step 7: Run full build**

Run: `go build ./...`
Expected: Success.

---

### Final Verification

- [ ] **Run both test suites and build together**

Run: `go build ./... && go test ./domain/valueobject/... ./cqrs/impl/memory/... -v`
Expected: Build succeeds, all tests PASS.
