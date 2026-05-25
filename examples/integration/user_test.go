package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/ddd-qce/core/aspect"
	"github.com/ddd-qce/core/cqrs/command"
	eventbus "github.com/ddd-qce/core/cqrs/event"
	commandmemory "github.com/ddd-qce/core/cqrs/impl/memory"
	eventmemory "github.com/ddd-qce/core/cqrs/impl/memory"
	"github.com/ddd-qce/core/cqrs/query"
	querymemory "github.com/ddd-qce/core/cqrs/impl/memory"
	"github.com/ddd-qce/core/cqrs/event"
)

type User struct {
	ID        string
	Name      string
	Email     string
	Active    bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

func NewUser(id, name, email string) *User {
	now := time.Now()
	return &User{
		ID:        id,
		Name:      name,
		Email:     email,
		Active:    true,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func (u *User) UpdateName(name string) error {
	if name == "" {
		return fmt.Errorf("name cannot be empty")
	}
	u.Name = name
	u.UpdatedAt = time.Now()
	return nil
}

func (u *User) UpdateEmail(email string) error {
	if email == "" {
		return fmt.Errorf("email cannot be empty")
	}
	u.Email = email
	u.UpdatedAt = time.Now()
	return nil
}

func (u *User) Deactivate() error {
	if !u.Active {
		return fmt.Errorf("user %s already deactivated", u.ID)
	}
	u.Active = false
	u.UpdatedAt = time.Now()
	return nil
}

func (u *User) Activate() error {
	if u.Active {
		return fmt.Errorf("user %s already active", u.ID)
	}
	u.Active = true
	u.UpdatedAt = time.Now()
	return nil
}

type UserRepository struct {
	users map[string]*User
}

func NewUserRepository() *UserRepository {
	return &UserRepository{
		users: make(map[string]*User),
	}
}

func (r *UserRepository) Save(user *User) error {
	r.users[user.ID] = user
	return nil
}

func (r *UserRepository) FindByID(id string) (*User, error) {
	user, exists := r.users[id]
	if !exists {
		return nil, fmt.Errorf("user %s not found", id)
	}
	return user, nil
}

func (r *UserRepository) FindByEmail(email string) (*User, error) {
	for _, user := range r.users {
		if user.Email == email {
			return user, nil
		}
	}
	return nil, fmt.Errorf("user with email %s not found", email)
}

type testCreateUserCommand struct {
	command.BaseCommand
	UserID string
	Name   string
	Email  string
}

type testCreateUserResult struct {
	UserID string
}

type testCreateUserHandler struct {
	repo       *UserRepository
	eventBus *eventmemory.EventBus
}

func (h *testCreateUserHandler) Handle(ctx context.Context, cmd *testCreateUserCommand) (*testCreateUserResult, error) {
	user := NewUser(cmd.UserID, cmd.Name, cmd.Email)
	if err := h.repo.Save(user); err != nil {
		return nil, err
	}
	eventbus.Dispatch(ctx, h.eventBus, &testUserCreatedEvent{
		BaseEvent: event.NewBaseEvent(user.ID, time.Now()),
		Name:            user.Name,
	})
	return &testCreateUserResult{UserID: user.ID}, nil
}

func (h *testCreateUserHandler) SetEventBus(bus *eventmemory.EventBus) {
	h.eventBus = bus
}

type testUserCreatedEvent struct {
	event.BaseEvent
	Name string
}

type testUserCreatedEventHandler struct {
	called bool
}

func (h *testUserCreatedEventHandler) Handle(ctx context.Context, event *testUserCreatedEvent) error {
	h.called = true
	return nil
}

type testUpdateUserCommand struct {
	command.BaseCommand
	UserID string
	Name   string
	Email  string
}

type testUpdateUserResult struct {
	Success bool
}

type testUpdateUserHandler struct {
	repo *UserRepository
}

func (h *testUpdateUserHandler) Handle(ctx context.Context, cmd *testUpdateUserCommand) (*testUpdateUserResult, error) {
	user, err := h.repo.FindByID(cmd.UserID)
	if err != nil {
		return nil, err
	}
	if cmd.Name != "" {
		if err := user.UpdateName(cmd.Name); err != nil {
			return nil, err
		}
	}
	if cmd.Email != "" {
		if err := user.UpdateEmail(cmd.Email); err != nil {
			return nil, err
		}
	}
	return &testUpdateUserResult{Success: true}, nil
}

type testGetUserQuery struct {
	query.BaseQuery
	UserID string
}

type testGetUserResult struct {
	ID     string
	Name   string
	Email  string
	Active bool
}

type testGetUserHandler struct {
	repo *UserRepository
}

func (h *testGetUserHandler) Handle(ctx context.Context, query *testGetUserQuery) (*testGetUserResult, error) {
	user, err := h.repo.FindByID(query.UserID)
	if err != nil {
		return nil, err
	}
	return &testGetUserResult{
		ID:     user.ID,
		Name:   user.Name,
		Email:  user.Email,
		Active: user.Active,
	}, nil
}

func TestUserEntity_CreateAndUpdateFlow(t *testing.T) {
	ctx := context.Background()
	chain := aspect.NewAspectChain()

	cmdBus := commandmemory.NewCommandBus(commandmemory.WithCommandBusAspectChain(chain))
	eventBus := eventmemory.NewEventBus(eventmemory.WithBusAspectChain(chain))
	qBus := querymemory.NewQueryBus(querymemory.WithQueryBusAspectChain(chain))
	repo := NewUserRepository()

	eventHandler := &testUserCreatedEventHandler{}
	createHandler := &testCreateUserHandler{repo: repo}
	createHandler.SetEventBus(eventBus)
	updateHandler := &testUpdateUserHandler{repo: repo}
	getHandler := &testGetUserHandler{repo: repo}

	commandmemory.RegisterCommand(cmdBus, createHandler)
	commandmemory.RegisterCommand(cmdBus, updateHandler)
	eventmemory.RegisterEvent[*testUserCreatedEvent](eventBus, eventHandler)
	querymemory.RegisterQuery(qBus, getHandler)

	result, err := 	command.Dispatch(ctx, cmdBus, &testCreateUserCommand{
		UserID: "user-001",
		Name:   "张三",
		Email:  "zhangsan@example.com",
	})
	if err != nil {
		t.Fatalf("create user failed: %v", err)
	}
	if !eventHandler.called {
		t.Error("user created event handler was not called")
	}

	_, err = 	command.Dispatch(ctx, cmdBus, &testUpdateUserCommand{
		UserID: result.UserID,
		Name:   "张三更新",
	})
	if err != nil {
		t.Fatalf("update user failed: %v", err)
	}

	qResult, err := query.Dispatch(ctx, qBus, &testGetUserQuery{
		UserID: result.UserID,
	})
	if err != nil {
		t.Fatalf("get user failed: %v", err)
	}
	if qResult.Name != "张三更新" {
		t.Errorf("expected name '张三更新', got %s", qResult.Name)
	}
	if qResult.Email != "zhangsan@example.com" {
		t.Errorf("expected email 'zhangsan@example.com', got %s", qResult.Email)
	}
}

func TestUserEntity_StateTransitions(t *testing.T) {
	user := NewUser("user-002", "李四", "lisi@example.com")

	if !user.Active {
		t.Error("new user should be active")
	}

	err := user.Deactivate()
	if err != nil {
		t.Fatalf("deactivate failed: %v", err)
	}
	if user.Active {
		t.Error("user should be deactivated")
	}

	err = user.Deactivate()
	if err == nil {
		t.Fatal("expected error when deactivating already deactivated user")
	}

	err = user.Activate()
	if err != nil {
		t.Fatalf("activate failed: %v", err)
	}
	if !user.Active {
		t.Error("user should be active again")
	}

	err = user.Activate()
	if err == nil {
		t.Fatal("expected error when activating already active user")
	}
}

func TestUserEntity_Validation(t *testing.T) {
	user := NewUser("user-003", "王五", "wangwu@example.com")

	err := user.UpdateName("")
	if err == nil {
		t.Fatal("expected error when updating name to empty")
	}

	err = user.UpdateEmail("")
	if err == nil {
		t.Fatal("expected error when updating email to empty")
	}
}

func TestUserEntity_Repository(t *testing.T) {
	repo := NewUserRepository()

	user1 := NewUser("user-004", "赵六", "zhaoliu@example.com")
	repo.Save(user1)

	found, err := repo.FindByID("user-004")
	if err != nil {
		t.Fatalf("find by ID failed: %v", err)
	}
	if found.Name != "赵六" {
		t.Errorf("expected name '赵六', got %s", found.Name)
	}

	_, err = repo.FindByID("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent user")
	}

	foundByEmail, err := repo.FindByEmail("zhaoliu@example.com")
	if err != nil {
		t.Fatalf("find by email failed: %v", err)
	}
	if foundByEmail.ID != "user-004" {
		t.Errorf("expected ID 'user-004', got %s", foundByEmail.ID)
	}
}
