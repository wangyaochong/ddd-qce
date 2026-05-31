package observability

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"
)

type FieldInfo struct {
	Name string
	Type string
}

type TypePrototype struct {
	Name       string
	Category   string
	Domain     string
	Fields     []FieldInfo
	ResultType string
	Result     []FieldInfo
}

type DomainStats struct {
	CommandCount  int
	CommandErrors int
	QueryCount    int
	QueryErrors   int
	EventCount    int
	EventErrors   int
}

type DomainInfo struct {
	Name     string
	Commands []*TypePrototype
	Queries  []*TypePrototype
	Events   []*TypePrototype
}

type DomainEntry struct {
	Type      string
	Category  string
	Status    string
	Duration  string
	CreatedAt string
	Error     string
	Data      string
	Result    string
}

type TypePrototypeRegistry struct {
	prototypes map[string]*TypePrototype
	byCategory map[string][]*TypePrototype
	byDomain   map[string][]*TypePrototype
	domains    []string
	mu         sync.RWMutex
}

func NewTypePrototypeRegistry() *TypePrototypeRegistry {
	return &TypePrototypeRegistry{
		prototypes: make(map[string]*TypePrototype),
		byCategory: make(map[string][]*TypePrototype),
		byDomain:   make(map[string][]*TypePrototype),
		domains:    []string{},
	}
}

func (r *TypePrototypeRegistry) Register(cat, name string, fields []FieldInfo, resultType string, result []FieldInfo) {
	r.RegisterWithDomain(cat, "", name, fields, resultType, result)
}

func (r *TypePrototypeRegistry) RegisterWithDomain(cat, domain, name string, fields []FieldInfo, resultType string, result []FieldInfo) {
	r.mu.Lock()
	defer r.mu.Unlock()

	proto := &TypePrototype{
		Name:       name,
		Category:   cat,
		Domain:     domain,
		Fields:     fields,
		ResultType: resultType,
		Result:     result,
	}
	r.prototypes[name] = proto
	r.byCategory[cat] = append(r.byCategory[cat], proto)

	if domain != "" {
		if _, exists := r.byDomain[domain]; !exists {
			r.domains = append(r.domains, domain)
			sort.Strings(r.domains)
		}
		r.byDomain[domain] = append(r.byDomain[domain], proto)
	}
}

func (r *TypePrototypeRegistry) RegisterFromType(cat, name string, t reflect.Type, resultType reflect.Type) {
	r.RegisterFromTypeWithDomain(cat, "", name, t, resultType)
}

func (r *TypePrototypeRegistry) RegisterFromTypeWithDomain(cat, domain, name string, t reflect.Type, resultType reflect.Type) {
	fields := extractFields(t)
	resultFields := extractFields(resultType)
	resultName := ""
	if resultType != nil {
		resultName = resultType.Name()
	}
	r.RegisterWithDomain(cat, domain, name, fields, resultName, resultFields)
}

func (r *TypePrototypeRegistry) RegisterFromSample(cat, name string, sample any, resultSample any) {
	domain := inferDomainFromSample(sample)
	r.RegisterFromSampleWithDomain(cat, domain, name, sample, resultSample)
}

func (r *TypePrototypeRegistry) RegisterFromSampleWithDomain(cat, domain, name string, sample any, resultSample any) {
	t := reflect.TypeOf(sample)
	if t == nil {
		return
	}
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	var resultType reflect.Type
	if resultSample != nil {
		resultType = reflect.TypeOf(resultSample)
		if resultType != nil && resultType.Kind() == reflect.Ptr {
			resultType = resultType.Elem()
		}
	}

	r.RegisterFromTypeWithDomain(cat, domain, name, t, resultType)
}

func inferDomainFromSample(sample any) string {
	t := reflect.TypeOf(sample)
	if t == nil {
		return ""
	}
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return inferDomainFromPkgPath(t.PkgPath())
}

func inferDomainFromPkgPath(pkgPath string) string {
	if pkgPath == "" {
		return ""
	}
	idx := strings.Index(pkgPath, "/ddd/")
	if idx != -1 {
		start := idx + 5
		end := strings.Index(pkgPath[start:], "/")
		if end == -1 {
			if start < len(pkgPath) {
				return pkgPath[start:]
			}
			return ""
		}
		return pkgPath[start : start+end]
	}
	if strings.HasPrefix(pkgPath, "ddd/") {
		start := 4
		end := strings.Index(pkgPath[start:], "/")
		if end == -1 {
			if start < len(pkgPath) {
				return pkgPath[start:]
			}
			return ""
		}
		return pkgPath[start : start+end]
	}
	return ""
}

func (r *TypePrototypeRegistry) Get(name string) *TypePrototype {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.prototypes[name]
}

func (r *TypePrototypeRegistry) ListByCategory(cat string) []*TypePrototype {
	r.mu.RLock()
	defer r.mu.RUnlock()

	list := r.byCategory[cat]
	sort.Slice(list, func(i, j int) bool {
		return list[i].Name < list[j].Name
	})
	return list
}

func (r *TypePrototypeRegistry) ListAll() []*TypePrototype {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var all []*TypePrototype
	for _, proto := range r.prototypes {
		all = append(all, proto)
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].Category != all[j].Category {
			return all[i].Category < all[j].Category
		}
		return all[i].Name < all[j].Name
	})
	return all
}

func (r *TypePrototypeRegistry) CountByCategory(cat string) int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.byCategory[cat])
}

func (r *TypePrototypeRegistry) ListDomains() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.domains
}

func (r *TypePrototypeRegistry) GetDomainInfo(domain string) *DomainInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	info := &DomainInfo{Name: domain}
	types := r.byDomain[domain]
	for _, t := range types {
		switch t.Category {
		case "command":
			info.Commands = append(info.Commands, t)
		case "query":
			info.Queries = append(info.Queries, t)
		case "event":
			info.Events = append(info.Events, t)
		}
	}
	sort.Slice(info.Commands, func(i, j int) bool { return info.Commands[i].Name < info.Commands[j].Name })
	sort.Slice(info.Queries, func(i, j int) bool { return info.Queries[i].Name < info.Queries[j].Name })
	sort.Slice(info.Events, func(i, j int) bool { return info.Events[i].Name < info.Events[j].Name })
	return info
}

func (r *TypePrototypeRegistry) ListByDomain(domain string) []*TypePrototype {
	r.mu.RLock()
	defer r.mu.RUnlock()

	list := r.byDomain[domain]
	sort.Slice(list, func(i, j int) bool {
		if list[i].Category != list[j].Category {
			return list[i].Category < list[j].Category
		}
		return list[i].Name < list[j].Name
	})
	return list
}

func (r *TypePrototypeRegistry) GetTypeDomain(typeName string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	normalized := normalizeTypeName(typeName)
	if proto, ok := r.prototypes[normalized]; ok {
		return proto.Domain
	}
	return ""
}

func normalizeTypeName(name string) string {
	name = strings.TrimPrefix(name, "Command/")
	name = strings.TrimPrefix(name, "Query/")
	name = strings.TrimPrefix(name, "Event/")
	name = strings.TrimPrefix(name, "*")
	if idx := strings.LastIndex(name, "."); idx != -1 {
		name = name[idx+1:]
	}
	return name
}

func extractFields(t reflect.Type) []FieldInfo {
	if t == nil {
		return nil
	}
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return nil
	}

	var fields []FieldInfo
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.IsExported() {
			fieldInfo := FieldInfo{
				Name: f.Name,
				Type: formatFieldType(f.Type),
			}
			fields = append(fields, fieldInfo)
		}
	}
	return fields
}

func formatFieldType(t reflect.Type) string {
	if t == nil {
		return ""
	}

	if t.Kind() == reflect.Ptr {
		return "*" + formatFieldType(t.Elem())
	}

	if t.Kind() == reflect.Slice {
		return "[]" + formatFieldType(t.Elem())
	}

	if t.Kind() == reflect.Array {
		return fmt.Sprintf("[%d]%s", t.Len(), formatFieldType(t.Elem()))
	}

	if t.Kind() == reflect.Map {
		return fmt.Sprintf("map[%s]%s", formatFieldType(t.Key()), formatFieldType(t.Elem()))
	}

	if t.PkgPath() != "" {
		shortPkg := shortPackageName(t.PkgPath())
		return shortPkg + "." + t.Name()
	}

	return t.Name()
}

func shortPackageName(pkgPath string) string {
	parts := splitPkgPath(pkgPath)
	for i := len(parts) - 1; i >= 0; i-- {
		if parts[i] != "" && parts[i] != "core" && parts[i] != "exampleapp" {
			return parts[i]
		}
	}
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return pkgPath
}

func splitPkgPath(pkgPath string) []string {
	var parts []string
	start := 0
	for i := 0; i < len(pkgPath); i++ {
		if pkgPath[i] == '/' {
			if i > start {
				parts = append(parts, pkgPath[start:i])
			}
			start = i + 1
		}
	}
	if start < len(pkgPath) {
		parts = append(parts, pkgPath[start:])
	}
	return parts
}
