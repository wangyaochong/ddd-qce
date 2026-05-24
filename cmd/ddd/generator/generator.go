package generator

import (
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

type AggregateData struct {
	Name           string
	NameLower      string
	NamePlural     string
	NamePluralLower string
	Module         string
}

type templateEntry struct {
	name     string
	filename string
	tmpl     string
}

func GenerateAggregate(name, module string) error {
	data := AggregateData{
		Name:           name,
		NameLower:      strings.ToLower(name),
		NamePlural:     name + "s",
		NamePluralLower: strings.ToLower(name) + "s",
		Module:         module,
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get current directory: %w", err)
	}

	entries := []templateEntry{
		{"domainModel", filepath.Join(cwd, "domain", data.NameLower+".go"), domainModelTmpl},
		{"domainEvents", filepath.Join(cwd, "domain", data.NameLower+"_events.go"), domainEventsTmpl},
		{"domainTest", filepath.Join(cwd, "domain", data.NameLower+"_test.go"), domainTestTmpl},
		{"appCommands", filepath.Join(cwd, "application", data.NameLower+"_commands.go"), appCommandsTmpl},
		{"appCmdHandler", filepath.Join(cwd, "application", data.NameLower+"_cmd_handler.go"), appCmdHandlerTmpl},
		{"appQueryHandler", filepath.Join(cwd, "application", data.NameLower+"_query_handler.go"), appQueryHandlerTmpl},
		{"appEventHandler", filepath.Join(cwd, "application", data.NameLower+"_event_handler.go"), appEventHandlerTmpl},
		{"appRepository", filepath.Join(cwd, "application", data.NameLower+"_repository.go"), appRepositoryTmpl},
	}

	for _, entry := range entries {
		if err := renderTemplate(data, entry); err != nil {
			return fmt.Errorf("render %s: %w", entry.name, err)
		}
	}

	fmt.Println("\n// Wire registration snippet:")
	fmt.Printf("// Add to your wire setup:\n")
	fmt.Printf("//   %sRepo := application.New%sRepository()\n", data.NameLower, data.Name)
	fmt.Printf("//   %sCmdHandler := application.NewCreate%sHandler(%sRepo, eventBus)\n", data.NameLower, data.Name, data.NameLower)
	fmt.Printf("//   %sQueryHandler := application.NewGet%sHandler(%sRepo)\n", data.NameLower, data.Name, data.NameLower)
	fmt.Printf("//   %sListHandler := application.NewList%sHandler(%sRepo)\n", data.NameLower, data.NamePlural, data.NameLower)
	fmt.Printf("//   %sCreatedHandler := application.New%sCreatedNotificationHandler()\n", data.NameLower, data.Name)

	return nil
}

func renderTemplate(data AggregateData, entry templateEntry) error {
	dir := filepath.Dir(entry.filename)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create directory %s: %w", dir, err)
	}

	tmpl, err := template.New(entry.name).Parse(entry.tmpl)
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("execute template: %w", err)
	}

	formatted, err := format.Source([]byte(buf.String()))
	if err != nil {
		return fmt.Errorf("go/format: %w\n--- raw output ---\n%s", err, buf.String())
	}

	if err := os.WriteFile(entry.filename, formatted, 0o644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	fmt.Printf("  created %s\n", entry.filename)
	return nil
}

var domainModelTmpl = `package domain

import (
	"fmt"
	"time"

	"github.com/ddd-qce/core/domain/aggregate"
	"github.com/ddd-qce/core/domain/entity"
	"github.com/ddd-qce/core/domain/event"
)

type {{.Name}}Status string

const (
	{{.Name}}StatusPending   {{.Name}}Status = "pending"
	{{.Name}}StatusConfirmed {{.Name}}Status = "confirmed"
	{{.Name}}StatusShipped   {{.Name}}Status = "shipped"
	{{.Name}}StatusCancelled {{.Name}}Status = "cancelled"
)

type {{.Name}}Item struct {
	entity.Entity
	ProductName string
	Price       float64
	Quantity    int
}

func New{{.Name}}Item(id, productName string, price float64, quantity int) *{{.Name}}Item {
	return &{{.Name}}Item{
		Entity:      *entity.NewEntity(id),
		ProductName: productName,
		Price:       price,
		Quantity:    quantity,
	}
}

func (i *{{.Name}}Item) Subtotal() float64 {
	return i.Price * float64(i.Quantity)
}

type {{.Name}} struct {
	aggregate.AggregateRoot
	UserID      string
	Items       []*{{.Name}}Item
	Status      {{.Name}}Status
	TotalAmount float64
	CreatedAt   time.Time
}

func New{{.Name}}(id, userID string, items []*{{.Name}}Item) (*{{.Name}}, error) {
	o := &{{.Name}}{
		UserID:    userID,
		Items:     items,
		Status:    {{.Name}}StatusPending,
		CreatedAt: time.Now(),
	}
	o.AggregateRoot = *aggregate.NewAggregateRootWithApplier(id, o)
	if err := o.validate(); err != nil {
		return nil, err
	}
	o.TotalAmount = o.calculateTotal()
	if err := o.Apply(&{{.Name}}CreatedEvent{
		BaseEvent:   event.NewBaseEvent(o.GetID(), time.Now()),
		UserID:      o.UserID,
		TotalAmount: o.TotalAmount,
	}); err != nil {
		return nil, err
	}
	return o, nil
}

func New{{.Name}}ForReplay(id string) *{{.Name}} {
	o := &{{.Name}}{}
	o.AggregateRoot = *aggregate.NewAggregateRootWithApplier(id, o)
	return o
}

func (o *{{.Name}}) When(evt event.DomainEvent) {
	switch e := evt.(type) {
	case *{{.Name}}CreatedEvent:
		o.UserID = e.UserID
		o.TotalAmount = e.TotalAmount
		o.Status = {{.Name}}StatusPending
		o.CreatedAt = e.OccurredAt()
	case *{{.Name}}ConfirmedEvent:
		o.Status = {{.Name}}StatusConfirmed
	case *{{.Name}}CancelledEvent:
		o.Status = {{.Name}}StatusCancelled
	}
}

func (o *{{.Name}}) Confirm() error {
	if o.Status != {{.Name}}StatusPending {
		return fmt.Errorf("{{.NameLower}} can only be confirmed from pending status")
	}
	if err := o.Apply(&{{.Name}}ConfirmedEvent{
		BaseEvent: event.NewBaseEvent(o.GetID(), time.Now()),
	}); err != nil {
		return err
	}
	return nil
}

func (o *{{.Name}}) Cancel() error {
	if o.Status == {{.Name}}StatusShipped {
		return fmt.Errorf("cannot cancel shipped {{.NameLower}}")
	}
	if err := o.Apply(&{{.Name}}CancelledEvent{
		BaseEvent: event.NewBaseEvent(o.GetID(), time.Now()),
	}); err != nil {
		return err
	}
	return nil
}

func (o *{{.Name}}) validate() error {
	if err := o.AggregateRoot.Validate(); err != nil {
		return err
	}
	if o.UserID == "" {
		return fmt.Errorf("{{.NameLower}} must have a user ID")
	}
	if len(o.Items) == 0 {
		return fmt.Errorf("{{.NameLower}} must have at least one item")
	}
	for _, item := range o.Items {
		if item.IsEmpty() {
			return fmt.Errorf("{{.NameLower}} item has empty product ID")
		}
	}
	return nil
}

func (o *{{.Name}}) calculateTotal() float64 {
	var total float64
	for _, item := range o.Items {
		total += item.Subtotal()
	}
	return total
}
`

var domainEventsTmpl = `package domain

import (
	"github.com/ddd-qce/core/domain/event"
)

type {{.Name}}CreatedEvent struct {
	event.BaseEvent
	UserID      string
	TotalAmount float64
}

type {{.Name}}ConfirmedEvent struct {
	event.BaseEvent
}

type {{.Name}}ShippedEvent struct {
	event.BaseEvent
}

type {{.Name}}CancelledEvent struct {
	event.BaseEvent
	Reason string
}
`

var domainTestTmpl = `package domain

import (
	"testing"
	"time"

	"github.com/ddd-qce/core/domain/aggregate"
	"github.com/ddd-qce/core/domain/event"
)

func Test{{.Name}}Aggregate_Create(t *testing.T) {
	items := []*{{.Name}}Item{
		New{{.Name}}Item("prod-1", "Product A", 100.0, 2),
	}
	{{.NameLower}}, err := New{{.Name}}("{{.NameLower}}-1", "user-1", items)
	if err != nil {
		t.Fatalf("failed to create {{.NameLower}}: %v", err)
	}
	if {{.NameLower}}.Status != {{.Name}}StatusPending {
		t.Errorf("expected pending status, got %s", {{.NameLower}}.Status)
	}
	if {{.NameLower}}.TotalAmount != 200.0 {
		t.Errorf("expected 200.0, got %f", {{.NameLower}}.TotalAmount)
	}
	events := {{.NameLower}}.UncommittedEvents()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
}

func Test{{.Name}}Aggregate_Confirm(t *testing.T) {
	items := []*{{.Name}}Item{New{{.Name}}Item("prod-1", "Product A", 100.0, 1)}
	{{.NameLower}}, _ := New{{.Name}}("{{.NameLower}}-1", "user-1", items)
	{{.NameLower}}.MarkEventsAsCommitted()

	if err := {{.NameLower}}.Confirm(); err != nil {
		t.Fatalf("confirm failed: %v", err)
	}
	if {{.NameLower}}.Status != {{.Name}}StatusConfirmed {
		t.Errorf("expected confirmed, got %s", {{.NameLower}}.Status)
	}
}

func Test{{.Name}}Aggregate_Cancel(t *testing.T) {
	items := []*{{.Name}}Item{New{{.Name}}Item("prod-1", "Product A", 100.0, 1)}
	{{.NameLower}}, _ := New{{.Name}}("{{.NameLower}}-1", "user-1", items)
	{{.NameLower}}.MarkEventsAsCommitted()

	if err := {{.NameLower}}.Cancel(); err != nil {
		t.Fatalf("cancel failed: %v", err)
	}
	if {{.NameLower}}.Status != {{.Name}}StatusCancelled {
		t.Errorf("expected cancelled, got %s", {{.NameLower}}.Status)
	}
}

func Test{{.Name}}Aggregate_When(t *testing.T) {
	o := &{{.Name}}{}
	o.AggregateRoot = *aggregate.NewAggregateRootWithApplier("{{.NameLower}}-1", o)
	_ = o.LoadFromHistory([]event.DomainEvent{
		&{{.Name}}CreatedEvent{BaseEvent: event.NewBaseEvent("{{.NameLower}}-1", time.Now()), UserID: "user-1", TotalAmount: 100},
	})
	if o.Status != {{.Name}}StatusPending {
		t.Errorf("expected pending, got %s", o.Status)
	}
	if o.UserID != "user-1" {
		t.Errorf("expected user-1, got %s", o.UserID)
	}
}
`

var appCommandsTmpl = `package application

import (
	"github.com/ddd-qce/core/cqrs/command"
	"github.com/ddd-qce/core/cqrs/query"
)

type ItemInput struct {
	ProductID   string
	ProductName string
	Price       float64
	Quantity    int
}

type Create{{.Name}}Command struct {
	command.BaseCommand
	UserID string
	Items  []ItemInput
}

type Create{{.Name}}Result struct {
	{{.Name}}ID     string
	TotalAmount float64
}

type Get{{.Name}}Query struct {
	query.BaseQuery
	{{.Name}}ID string
}

type {{.Name}}ViewItem struct {
	ProductID   string
	ProductName string
	Price       float64
	Quantity    int
	Subtotal    float64
}

type Get{{.Name}}Result struct {
	{{.Name}}ID     string
	UserID      string
	Status      string
	TotalAmount float64
	Items       []{{.Name}}ViewItem
	CreatedAt   string
}

type List{{.NamePlural}}Query struct {
	query.BaseQuery
}

type List{{.NamePlural}}Result struct {
	{{.NamePlural}} []Get{{.Name}}Result
}
`

var appCmdHandlerTmpl = `package application

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/ddd-qce/core/cqrs/command"
	cqrsevent "github.com/ddd-qce/core/cqrs/event"
	domainevent "github.com/ddd-qce/core/domain/event"
	"{{.Module}}/domain"
)

type Create{{.Name}}Handler struct {
	repo     {{.Name}}RepositoryAdapter
	eventBus cqrsevent.EventBus
}

func NewCreate{{.Name}}Handler(repo {{.Name}}RepositoryAdapter, eventBus cqrsevent.EventBus) *Create{{.Name}}Handler {
	return &Create{{.Name}}Handler{repo: repo, eventBus: eventBus}
}

func (h *Create{{.Name}}Handler) Handle(ctx context.Context, cmd *Create{{.Name}}Command) (*Create{{.Name}}Result, error) {
	items := make([]*domain.{{.Name}}Item, len(cmd.Items))
	for i, input := range cmd.Items {
		items[i] = domain.New{{.Name}}Item(uuid.New().String(), input.ProductName, input.Price, input.Quantity)
	}

	{{.NameLower}}ID := uuid.New().String()
	{{.NameLower}}, err := domain.New{{.Name}}({{.NameLower}}ID, cmd.UserID, items)
	if err != nil {
		return nil, err
	}

	if err := h.repo.Save(ctx, {{.NameLower}}); err != nil {
		return nil, err
	}

	cqrsevent.Dispatch[*domain.{{.Name}}CreatedEvent](ctx, h.eventBus, &domain.{{.Name}}CreatedEvent{
		BaseEvent:   domainevent.NewBaseEvent({{.NameLower}}.GetID(), time.Now()),
		UserID:      {{.NameLower}}.UserID,
		TotalAmount: {{.NameLower}}.TotalAmount,
	})

	return &Create{{.Name}}Result{ {{.Name}}ID: {{.NameLower}}.GetID(), TotalAmount: {{.NameLower}}.TotalAmount}, nil
}

var _ command.CommandHandler[*Create{{.Name}}Command, *Create{{.Name}}Result] = (*Create{{.Name}}Handler)(nil)
`

var appQueryHandlerTmpl = `package application

import (
	"context"
	"time"

	"github.com/ddd-qce/core/cqrs/query"
	"{{.Module}}/domain"
)

type Get{{.Name}}Handler struct {
	repo {{.Name}}RepositoryAdapter
}

func NewGet{{.Name}}Handler(repo {{.Name}}RepositoryAdapter) *Get{{.Name}}Handler {
	return &Get{{.Name}}Handler{repo: repo}
}

func (h *Get{{.Name}}Handler) Handle(ctx context.Context, q *Get{{.Name}}Query) (*Get{{.Name}}Result, error) {
	{{.NameLower}}, err := h.repo.FindByID(ctx, q.{{.Name}}ID)
	if err != nil {
		return nil, err
	}
	return to{{.Name}}View({{.NameLower}}), nil
}

type List{{.NamePlural}}Handler struct {
	repo {{.Name}}RepositoryAdapter
}

func NewList{{.NamePlural}}Handler(repo {{.Name}}RepositoryAdapter) *List{{.NamePlural}}Handler {
	return &List{{.NamePlural}}Handler{repo: repo}
}

func (h *List{{.NamePlural}}Handler) Handle(ctx context.Context, q *List{{.NamePlural}}Query) (*List{{.NamePlural}}Result, error) {
	{{.NamePluralLower}} := h.repo.FindAll()
	result := make([]Get{{.Name}}Result, len({{.NamePluralLower}}))
	for i, o := range {{.NamePluralLower}} {
		result[i] = *to{{.Name}}View(o)
	}
	return &List{{.NamePlural}}Result{ {{.NamePlural}}: result}, nil
}

var _ query.QueryHandler[*Get{{.Name}}Query, *Get{{.Name}}Result] = (*Get{{.Name}}Handler)(nil)
var _ query.QueryHandler[*List{{.NamePlural}}Query, *List{{.NamePlural}}Result] = (*List{{.NamePlural}}Handler)(nil)

func to{{.Name}}View(o *domain.{{.Name}}) *Get{{.Name}}Result {
	items := make([]{{.Name}}ViewItem, len(o.Items))
	for i, item := range o.Items {
		items[i] = {{.Name}}ViewItem{
			ProductID:   item.GetID(),
			ProductName: item.ProductName,
			Price:       item.Price,
			Quantity:    item.Quantity,
			Subtotal:    item.Subtotal(),
		}
	}
	result := &Get{{.Name}}Result{
		{{.Name}}ID:     o.GetID(),
		UserID:      o.UserID,
		Status:      string(o.Status),
		TotalAmount: o.TotalAmount,
		Items:       items,
	}
	if !o.CreatedAt.IsZero() {
		result.CreatedAt = o.CreatedAt.Format(time.RFC3339)
	}
	return result
}
`

var appEventHandlerTmpl = `package application

import (
	"context"
	"log"

	"{{.Module}}/domain"
)

type {{.Name}}CreatedNotificationHandler struct{}

func New{{.Name}}CreatedNotificationHandler() *{{.Name}}CreatedNotificationHandler {
	return &{{.Name}}CreatedNotificationHandler{}
}

func (h *{{.Name}}CreatedNotificationHandler) Handle(ctx context.Context, evt *domain.{{.Name}}CreatedEvent) error {
	log.Printf("[Notification] {{.Name}} %s created by user %s, total: $%.2f",
		evt.AggregateID(), evt.UserID, evt.TotalAmount)
	return nil
}
`

var appRepositoryTmpl = `package application

import (
	"context"
	"fmt"
	"sync"

	"github.com/ddd-qce/core/domain/event"
	"github.com/ddd-qce/core/domain/repository"
	"{{.Module}}/domain"
)

type {{.Name}}RepositoryAdapter interface {
	Save(ctx context.Context, {{.NameLower}} *domain.{{.Name}}) error
	FindByID(ctx context.Context, id string) (*domain.{{.Name}}, error)
	Delete(ctx context.Context, id string) error
	FindAll() []*domain.{{.Name}}
}

type {{.Name}}Repository struct {
	mu                sync.RWMutex
	{{.NamePluralLower}} map[string]*domain.{{.Name}}
}

func New{{.Name}}Repository() *{{.Name}}Repository {
	return &{{.Name}}Repository{ {{.NamePluralLower}}: make(map[string]*domain.{{.Name}})}
}

func (r *{{.Name}}Repository) Save(ctx context.Context, {{.NameLower}} *domain.{{.Name}}) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.{{.NamePluralLower}}[{{.NameLower}}.GetID()] = {{.NameLower}}
	return nil
}

func (r *{{.Name}}Repository) FindByID(ctx context.Context, id string) (*domain.{{.Name}}, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	{{.NameLower}}, ok := r.{{.NamePluralLower}}[id]
	if !ok {
		return nil, fmt.Errorf("{{.NameLower}} %s not found", id)
	}
	return {{.NameLower}}, nil
}

func (r *{{.Name}}Repository) Delete(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.{{.NamePluralLower}}[id]; !ok {
		return fmt.Errorf("{{.NameLower}} %s not found", id)
	}
	delete(r.{{.NamePluralLower}}, id)
	return nil
}

func (r *{{.Name}}Repository) FindAll() []*domain.{{.Name}} {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*domain.{{.Name}}, 0, len(r.{{.NamePluralLower}}))
	for _, o := range r.{{.NamePluralLower}} {
		result = append(result, o)
	}
	return result
}

var _ repository.Repository[*domain.{{.Name}}] = (*{{.Name}}Repository)(nil)

type {{.Name}}EventSourcedRepository struct {
	eventStore      event.EventStore[event.DomainEvent]
	{{.NameLower}}Repo {{.Name}}RepositoryAdapter
}

func New{{.Name}}EventSourcedRepository(
	eventStore event.EventStore[event.DomainEvent],
	{{.NameLower}}Repo {{.Name}}RepositoryAdapter,
) *{{.Name}}EventSourcedRepository {
	return &{{.Name}}EventSourcedRepository{
		eventStore:      eventStore,
		{{.NameLower}}Repo: {{.NameLower}}Repo,
	}
}

func (r *{{.Name}}EventSourcedRepository) Save(ctx context.Context, {{.NameLower}} *domain.{{.Name}}) error {
	uncommitted := {{.NameLower}}.UncommittedEvents()
	if len(uncommitted) > 0 {
		if err := r.eventStore.Append(ctx, {{.NameLower}}.GetID(), {{.NameLower}}.Version()-len(uncommitted), uncommitted); err != nil {
			return err
		}
		{{.NameLower}}.MarkEventsAsCommitted()
	}
	return r.{{.NameLower}}Repo.Save(ctx, {{.NameLower}})
}

func (r *{{.Name}}EventSourcedRepository) Load(ctx context.Context, id string) (*domain.{{.Name}}, error) {
	events, err := r.eventStore.Load(ctx, id, 0)
	if err != nil {
		return nil, err
	}
	if len(events) == 0 {
		return nil, fmt.Errorf("{{.NameLower}} %s not found in event store", id)
	}
	{{.NameLower}} := domain.New{{.Name}}ForReplay(id)
	if err := {{.NameLower}}.LoadFromHistory(events); err != nil {
		return nil, fmt.Errorf("load {{.NameLower}} %s from history: %w", id, err)
	}
	return {{.NameLower}}, nil
}

var _ repository.EventSourcingRepository[*domain.{{.Name}}] = (*{{.Name}}EventSourcedRepository)(nil)
`
