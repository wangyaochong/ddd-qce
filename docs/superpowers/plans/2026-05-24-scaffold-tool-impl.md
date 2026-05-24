# DDD-QCE 脚手架工具实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 实现 `ddd new aggregate <Name>` 命令，自动生成符合框架约定的聚合骨架代码

**Architecture:** 采用 Go CLI + 内嵌 text/template 方案。CLI 使用标准库 flag 解析参数，模板文件内嵌在 Go 代码中使用 embed.FS，生成代码遵循框架分层规则

**Tech Stack:** Go 1.26, text/template, embed

---

## 文件结构

在开始任务前，先明确需要创建的文件：

```
cmd/ddd/
├── main.go                    # CLI 入口，命令解析和执行
├── templates/
│   └── aggregate/
│       ├── domain_model.go.txt
│       ├── domain_events.go.txt
│       ├── domain_test.go.txt
│       ├── application_commands.go.txt
│       ├── application_cmd_handler.go.txt
│       ├── application_query_handler.go.txt
│       ├── application_event_handler.go.txt
│       └── application_repository.go.txt
��── generator/
    └── generator.go           # 模板生成逻辑
```

---

## 实施任务

### Task 1: 创建 CLI 框架

**Files:**
- Create: `cmd/ddd/main.go`

- [ ] **Step 1: 创建 cmd/ddd 目录结构**

```bash
mkdir -p cmd/ddd/templates/aggregate cmd/ddd/generator
```

- [ ] **Step 2: 编写 main.go 基础框架**

```go
// cmd/ddd/main.go
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

func main() {
	if err := run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("ddd", flag.ExitOnError)
	fs.Usage = usage
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}

	if fs.NArg() < 2 {
		return fmt.Errorf("missing arguments")
	}

	cmd := fs.Arg(0)
	subCmd := fs.Arg(1)

	switch cmd + " " + subCmd {
	case "new aggregate":
		return handleNewAggregate(fs)
	default:
		return fmt.Errorf("unknown command: %s %s", cmd, subCmd)
	}
}

func handleNewAggregate(fs *flag.FlagSet) error {
	name := fs.Arg(2)
	module := fs.String("module", "", "target module name (e.g., github.com/myorg/myapp)")

	if *module == "" {
		return fmt.Errorf("--module is required")
	}
	if name == "" {
		return fmt.Errorf("aggregate name is required")
	}

	fmt.Printf("Generating aggregate %s for module %s\n", name, *module)
	// TODO: implement generator
	return nil
}

func usage() {
	fmt.Print(`Usage: ddd <command> <subcommand> [options]

Commands:
  new aggregate <name>  Create a new aggregate scaffold

Options:
  --module string       Target module name (required)

Examples:
  ddd new aggregate Order --module github.com/myorg/myapp
`)
}
```

- [ ] **Step 3: 测试 CLI 运行**

```bash
cd /home/wyc/projects/ddd-qce
go run ./cmd/ddd --help
```

Expected: 显示 usage 信息

- [ ] **Step 4: 测试 new aggregate 命令**

```bash
go run ./cmd/ddd new aggregate Order --module github.com/myorg/myapp
```

Expected: 输出 "Generating aggregate Order for module github.com/myorg/myapp"

- [ ] **Step 5: Commit**

```bash
git add cmd/ddd/
git commit -m "feat(scaffold): create CLI framework"
```

---

### Task 2: 实现模板生成器

**Files:**
- Create: `cmd/ddd/generator/generator.go`
- Create: `cmd/ddd/templates/aggregate/` (8 个模板文件)

- [ ] **Step 1: 创建 generator.go 基础结构**

```go
// cmd/ddd/generator/generator.go
package generator

import (
	"bytes"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

type AggregateData struct {
	Name         string
	NameLower    string
	NamePlural   string
	Module       string
}

func GenerateAggregate(name, module string) error {
	data := AggregateData{
		Name:         name,
		NameLower:    strings.ToLower(name[:1]) + name[1:],
		NamePlural:   name + "s",
		Module:       module,
	}

	// Create domain directory
	domainDir := "domain"
	if err := os.MkdirAll(domainDir, 0755); err != nil {
		return fmt.Errorf("create domain dir: %w", err)
	}

	// Generate domain/model.go
	if err := generateFile(domainDir, nameLower(data)+".go", domainModelTmpl, data); err != nil {
		return err
	}

	// Generate domain/events.go
	if err := generateFile(domainDir, nameLower(data)+"_events.go", domainEventsTmpl, data); err != nil {
		return err
	}

	// Generate domain/test.go
	if err := generateFile(domainDir, nameLower(data)+"_test.go", domainTestTmpl, data); err != nil {
		return err
	}

	// Create application directory
	appDir := "application"
	if err := os.MkdirAll(appDir, 0755); err != nil {
		return fmt.Errorf("create application dir: %w", err)
	}

	// Generate application/commands.go
	if err := generateFile(appDir, nameLower(data)+"_commands.go", appCommandsTmpl, data); err != nil {
		return err
	}

	// Generate application/command_handler.go
	if err := generateFile(appDir, nameLower(data)+"_cmd_handler.go", appCmdHandlerTmpl, data); err != nil {
		return err
	}

	// Generate application/query_handler.go
	if err := generateFile(appDir, nameLower(data)+"_query_handler.go", appQueryHandlerTmpl, data); err != nil {
		return err
	}

	// Generate application/event_handler.go
	if err := generateFile(appDir, nameLower(data)+"_event_handler.go", appEventHandlerTmpl, data); err != nil {
		return err
	}

	// Generate application/repository.go
	if err := generateFile(appDir, nameLower(data)+"_repository.go", appRepositoryTmpl, data); err != nil {
		return err
	}

	// Print wire registration snippet
	printWireRegistration(data)

	return nil
}

func nameLower(d AggregateData) string {
	if len(d.Name) == 0 {
		return ""
	}
	return strings.ToLower(d.Name[:1]) + d.Name[1:]
}

func generateFile(dir, filename, tmpl string, data AggregateData) error {
	t, err := template.New(filename).Parse(tmpl)
	if err != nil {
		return fmt.Errorf("parse template %s: %w", filename, err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return fmt.Errorf("execute template %s: %w", filename, err)
	}

	// Format Go code
	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		// If format fails, write unformatted
		fmt.Fprintf(os.Stderr, "Warning: format failed for %s: %v\n", filename, err)
		formatted = buf.Bytes()
	}

	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, formatted, 0644); err != nil {
		return fmt.Errorf("write file %s: %w", path, err)
	}

	fmt.Printf("Created: %s\n", path)
	return nil
}

func printWireRegistration(data AggregateData) {
	fmt.Println("\n// ============================================================")
	fmt.Println("// Wire Registration Snippet - Copy to infrastructure/wire.go")
	fmt.Println("// ============================================================")
	fmt.Printf("\n// %s handlers\n", data.Name)
	fmt.Printf("if err := cmdBus.RegisterHandler(application.NewCreate%sHandler(orderRepo, eventBus)); err != nil {\n", data.Name)
	fmt.Printf("    return nil, fmt.Errorf(\"register Create%sHandler: %%w\", err)\n", data.Name)
	fmt.Println("}")
	fmt.Printf("\nif err := queryBus.RegisterHandler(application.NewGet%sHandler(orderRepo)); err != nil {\n", data.Name)
	fmt.Printf("    return nil, fmt.Errorf(\"register Get%sHandler: %%w\", err)\n", data.Name)
	fmt.Println("}")
	fmt.Printf("if err := queryBus.RegisterHandler(application.NewList%sHandler(orderRepo)); err != nil {\n", data.Name)
	fmt.Printf("    return nil, fmt.Errorf(\"register List%sHandler: %%w\", err)\n", data.Name)
	fmt.Println("}")
	fmt.Printf("\nif err := eventBus.SubscribeHandler(application.New%sCreatedNotificationHandler()); err != nil {\n", data.Name)
	fmt.Printf("    return nil, fmt.Errorf(\"register %sCreatedNotificationHandler: %%w\", err)\n", data.Name)
	fmt.Println("}")
}
```

- [ ] **Step 2: 添加模板变量和核心模板**

在 generator.go 中添加所有模板字符串（因为使用 embed 需要额外的 embedFS 设置，先用字符串内嵌方式简化实现）

- [ ] **Step 3: 创建 domain_model.go 模板**

```go
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
	// TODO: Add event handlers for other events
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
```

- [ ] **Step 4: ��建其他 7 个模板**

按照相同模式创建：
- domain_events.go.txt
- domain_test.go.txt
- application_commands.go.txt
- application_cmd_handler.go.txt
- application_query_handler.go.txt
- application_event_handler.go.txt
- application_repository.go.txt

- [ ] **Step 5: 修改 main.go 集成 generator**

```go
func handleNewAggregate(fs *flag.FlagSet) error {
	name := fs.Arg(2)
	module := fs.String("module", "", "target module name (e.g., github.com/myorg/myapp)")

	if *module == "" {
		return fmt.Errorf("--module is required")
	}
	if name == "" {
		return fmt.Errorf("aggregate name is required")
	}

	// Validate name is PascalCase
	if len(name) == 0 || !isUpperCase(name[0]) {
		return fmt.Errorf("aggregate name must start with uppercase letter")
	}

	if err := generator.GenerateAggregate(name, *module); err != nil {
		return fmt.Errorf("generate: %w", err)
	}

	return nil
}

func isUpperCase(c byte) bool {
	return c >= 'A' && c <= 'Z'
}
```

- [ ] **Step 6: 测试生成功能**

```bash
cd /tmp/test-scaffold
go run /home/wyc/projects/ddd-qce/cmd/ddd new aggregate TestAgg --module github.com/test/test
```

Expected:
- 创建 domain/test_agg.go
- 创建 domain/test_agg_events.go
- 创建 domain/test_agg_test.go
- 创建 application/test_agg_commands.go
- 创建 application/test_agg_cmd_handler.go
- 创建 application/test_agg_query_handler.go
- 创建 application/test_agg_event_handler.go
- 创建 application/test_agg_repository.go
- 输出 wire registration snippet

- [ ] **Step 7: Commit**

```bash
git add cmd/ddd/
git commit -m "feat(scaffold): implement template generator"
```

---

### Task 3: 完善模板内容

**Files:**
- Modify: `cmd/ddd/generator/generator.go` (补充所有模板)

- [ ] **Step 1: 检查生成的代码是否可编译**

```bash
cd /tmp/test-scaffold
go build ./...
```

修复任何编译错误

- [ ] **Step 2: 补充 domain_test.go 模板的完整测试用例**

参考 exampleapp/domain/domain_test.go 添加更多测试：
- 测试 Validate 逻辑
- 测试状态转换
- 测试 When 事件回放

- [ ] **Step 3: 补充 application 层的 import 路径**

确保所有 import 路径使用正确的模块路径：
```go
import (
    "github.com/ddd-qce/core/domain/aggregate"
    "github.com/ddd-qce/core/domain/entity"
    "github.com/ddd-qce/core/domain/event"
    "github.com/ddd-qce/core/domain/repository"
    "github.com/ddd-qce/core/cqrs/command"
    "github.com/ddd-qce/core/cqrs/query"
    cqrsevent "github.com/ddd-qce/core/cqrs/event"
    "github.com/google/uuid"
    "{{.Module}}/domain"
)
```

- [ ] **Step 4: 验证生成的代码风格一致**

检查：
- 是否有 var _ Interface = (*Impl)(nil) 编译检查
- 是否遵循 Go 代码规范
- 注释和命名是否一致

- [ ] **Step 5: Commit**

```bash
git add cmd/ddd/
git commit -m "feat(scaffold): improve template content"
```

---

### Task 4: 集成测试验证

**Files:**
- Create: `cmd/ddd/generator/generator_test.go`
- Test: 实际运行命令验证

- [ ] **Step 1: 编写 generator 单元测试**

```go
package generator

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateAggregate(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "scaffold-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Change to temp dir
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// Generate
	if err := GenerateAggregate("TestOrder", "github.com/test/mymodule"); err != nil {
		t.Fatalf("GenerateAggregate failed: %v", err)
	}

	// Verify files exist
	files := []string{
		"domain/test_order.go",
		"domain/test_order_events.go",
		"domain/test_order_test.go",
		"application/test_order_commands.go",
		"application/test_order_cmd_handler.go",
		"application/test_order_query_handler.go",
		"application/test_order_event_handler.go",
		"application/test_order_repository.go",
	}

	for _, f := range files {
		if _, err := os.Stat(filepath.Join(tmpDir, f)); os.IsNotExist(err) {
			t.Errorf("expected file %s not found", f)
		}
	}

	// Verify content contains expected patterns
	testOrderGo, _ := os.ReadFile(filepath.Join(tmpDir, "domain/test_order.go"))
	content := string(testOrderGo)

	if !contains(content, "type TestOrder struct") {
		t.Error("missing TestOrder struct definition")
	}
	if !contains(content, "func NewTestOrder(") {
		t.Error("missing NewTestOrder constructor")
	}
	if !contains(content, "func (o *TestOrder) When(") {
		t.Error("missing When method for event sourcing")
	}
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && 
	       (len(s) >= len(substr)) && 
	       (s == substr || 
	        len(s) > 0 && (s[:len(substr)] == substr || 
	         contains(s[1:], substr)))
}
```

- [ ] **Step 2: 运行单元测试**

```bash
cd /home/wyc/projects/ddd-qce
go test ./cmd/ddd/generator/... -v
```

- [ ] **Step 3: 实际运行完整测试**

```bash
mkdir -p /tmp/scaffold-e2e-test
cd /tmp/scaffold-e2e-test
go mod init github.com/test/myapp
go run /home/wyc/projects/ddd-qce/cmd/ddd new aggregate Product --module github.com/test/myapp
go build ./...
```

- [ ] **Step 4: Commit**

```bash
git add cmd/ddd/
git commit -m "test(scaffold): add generator tests"
```

---

### Task 5: 更新文档

**Files:**
- Modify: `docs/guide.md`
- Modify: `docs/architecture.md`

- [ ] **Step 1: 在 docs/guide.md 新增章节**

在 guide.md 末尾添加：

```markdown
---

## 十四、使用脚手架创建聚合

### 1. 脚手架工具简介

ddd-qce 提供 `ddd new aggregate` 命令，自动生成符合框架约定的聚合骨架代码。

### 2. 安装与使用

```bash
# 进入你的模块目录
cd my-ddd-app

# 生成聚合骨架
go run ./cmd/ddd new aggregate Order --module github.com/myorg/myapp
```

### 3. 生成的文件

执行命令后，会在当前目录生成以下文件：

```
domain/
├── order.go           # 聚合根 + 实体 + 状态常量
├── order_events.go    # 领域事件定义
└── order_test.go      # 基础测试用例

application/
├── order_commands.go    # Command + Result 定义
├── order_cmd_handler.go # Command Handler
├── order_query_handler.go # Query Handler
├── order_event_handler.go # Event Handler
└── order_repository.go  # Repository 适配器
```

### 4. 后续步骤

1. 补充业务逻辑（domain/order.go 中的业务方法）
2. 在 infrastructure/wire.go 中注册 Handler（参考输出的 registration snippet）
3. 运行测试验证：`go test ./...`

### 5. 示例：创建一个新的 Product 聚合

```bash
# 1. 创建模块（如果还没有）
go mod init github.com/myorg/shop

# 2. 添加框架依赖
go get github.com/ddd-qce/core

# 3. 生成聚合
go run ./cmd/ddd new aggregate Product --module github.com/myorg/shop

# 4. 查看生成的文件
ls -la domain/ application/
```

### 6. 命名规范

- 聚合名称使用 PascalCase：如 `Order`、`Product`、`Inventory`
- 生成的文件名自动转换为 camelCase：如 `order.go`、`order_events.go`
- 状态常量使用 `StatusName` 格式：如 `OrderStatusPending`

### 7. 自定义模板

（预留）未来版本将支持自定义模板目录。
```

- [ ] **Step 2: 在 docs/architecture.md 添加脚手架说明**

找到合适位置添加：

```markdown
## 脚手架工具

为了确保 AI 生成代码符合框架约定，降低新用户入门门槛，ddd-qce 提供脚手架工具。

详见 [实战指南 - 使用脚手架创建聚合](guide.md#十四使用脚手架创建聚合)。
```

- [ ] **Step 3: 验证文档可编译**

```bash
cd /home/wyc/projects/ddd-qce
go build ./docs/...
```

（如果有错误需要修复）

- [ ] **Step 4: Commit**

```bash
git add docs/
git commit -m "docs: add scaffold tool documentation"
```

---

### Task 6: 最终验证

**Files:**
- Test: 完整功能验证

- [ ] **Step 1: 在 exampleapp 目录测试脚手架**

```bash
cd /home/wyc/projects/ddd-qce/exampleapp
mkdir -p /tmp/scaffold-final-test
cd /tmp/scaffold-final-test
go mod init github.com/ddd-qce/testapp
go run /home/wyc/projects/ddd-qce/cmd/ddd new aggregate Payment --module github.com/ddd-qce/testapp
go build ./...
go test ./...
```

- [ ] **Step 2: 清理测试目录**

```bash
rm -rf /tmp/scaffold-final-test /tmp/test-scaffold
```

- [ ] **Step 3: 最终 commit**

```bash
git add .
git commit -m "feat(scaffold): complete scaffold tool implementation"
```

---

## 实施完成检查清单

- [ ] Task 1: CLI 框架创建完成
- [ ] Task 2: 模板生成器实现完成（8 个模板文件）
- [ ] Task 3: 模板内容完善，代码可编译
- [ ] Task 4: 集成测试通过
- [ ] Task 5: 文档更新完成
- [ ] Task 6: 最终验证通过

---

## 后续扩展（Out of Scope）

以下功能在当前版本中不考虑，但为未来扩展预留设计空间：

1. **Entity 脚手架**：`ddd new entity <Name>` 生成独立的 Entity 模板
2. **ValueObject 脚手架**：`ddd new valueobject <Name>` 生成值对象模板
3. **交互式模式**：`ddd new` 进入交互式问答模式
4. **模板自定义**：用户可以提供自定义模板目录覆盖默认模板