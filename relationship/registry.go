package relationship

import (
	"sort"
	"strings"
	"sync"
)

type RelationshipRegistry struct {
	commandHandlers map[string]string
	queryHandlers   map[string]string
	eventHandlers   map[string][]string
	handlerEmits    map[string][]string
	typeDomains     map[string]string
	mu              sync.RWMutex
}

func NewRegistry() *RelationshipRegistry {
	return &RelationshipRegistry{
		commandHandlers: make(map[string]string),
		queryHandlers:   make(map[string]string),
		eventHandlers:   make(map[string][]string),
		handlerEmits:    make(map[string][]string),
		typeDomains:     make(map[string]string),
	}
}

func (r *RelationshipRegistry) RecordCommandHandler(commandName, handlerName string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.commandHandlers[commandName] = handlerName
}

func (r *RelationshipRegistry) RecordQueryHandler(queryName, handlerName string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.queryHandlers[queryName] = handlerName
}

func (r *RelationshipRegistry) RecordEventHandler(eventName, handlerName string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	handlers := r.eventHandlers[eventName]
	for _, h := range handlers {
		if h == handlerName {
			return
		}
	}
	r.eventHandlers[eventName] = append(handlers, handlerName)
}

func (r *RelationshipRegistry) RecordTypeDomain(typeName, domain string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.typeDomains[typeName] = domain
}

func (r *RelationshipRegistry) RecordHandlerEmits(handlerName, eventName string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	events := r.handlerEmits[handlerName]
	for _, e := range events {
		if e == eventName {
			return
		}
	}
	r.handlerEmits[handlerName] = append(events, eventName)
	if handlerDomain := r.typeDomains[handlerName]; handlerDomain != "" {
		if r.typeDomains[eventName] == "" {
			r.typeDomains[eventName] = handlerDomain
		}
	}
}

func (r *RelationshipRegistry) GetTypeDomain(typeName string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.typeDomains[typeName]
}

type Node struct {
	ID     string `json:"id"`
	Type   string `json:"type"`
	Domain string `json:"domain"`
}

type Edge struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Type   string `json:"type"`
}

type Graph struct {
	Nodes   []Node   `json:"nodes"`
	Edges   []Edge   `json:"edges"`
	Domains []string `json:"domains"`
}

func (r *RelationshipRegistry) BuildGraph(domainFilter string, typeFilter string) *Graph {
	r.mu.RLock()
	defer r.mu.RUnlock()

	nodesMap := make(map[string]Node)
	edges := []Edge{}
	domainsMap := make(map[string]bool)

	addNode := func(id, nodeType, domain string) {
		if domainFilter != "" && domain != domainFilter {
			return
		}
		if typeFilter != "" && nodeType != typeFilter {
			return
		}
		nodesMap[id] = Node{ID: id, Type: nodeType, Domain: domain}
		if domain != "" {
			domainsMap[domain] = true
		}
	}

	addEdge := func(source, target, edgeType string) {
		edges = append(edges, Edge{Source: source, Target: target, Type: edgeType})
	}

	for cmd, handler := range r.commandHandlers {
		domain := r.typeDomains[cmd]
		addNode(cmd, "command", domain)
		addNode(handler, "handler", domain)
		addEdge(cmd, handler, "handles")
	}

	for qry, handler := range r.queryHandlers {
		domain := r.typeDomains[qry]
		addNode(qry, "query", domain)
		addNode(handler, "handler", domain)
		addEdge(qry, handler, "handles")
	}

	for evt, handlers := range r.eventHandlers {
		domain := r.typeDomains[evt]
		addNode(evt, "event", domain)
		for _, handler := range handlers {
			addNode(handler, "handler", domain)
			addEdge(evt, handler, "subscribes")
		}
	}

	for handler, events := range r.handlerEmits {
		handlerDomain := r.typeDomains[handler]
		addNode(handler, "handler", handlerDomain)
		for _, evt := range events {
			evtDomain := r.typeDomains[evt]
			if evtDomain == "" {
				evtDomain = handlerDomain
			}
			addNode(evt, "event", evtDomain)
			addEdge(handler, evt, "emits")
		}
	}

	nodes := make([]Node, 0, len(nodesMap))
	for _, n := range nodesMap {
		nodes = append(nodes, n)
	}
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Type != nodes[j].Type {
			return nodes[i].Type < nodes[j].Type
		}
		return nodes[i].ID < nodes[j].ID
	})

	domains := make([]string, 0, len(domainsMap))
	for d := range domainsMap {
		domains = append(domains, d)
	}
	sort.Strings(domains)

	return &Graph{Nodes: nodes, Edges: edges, Domains: domains}
}

func (r *RelationshipRegistry) GetCommandHandler(commandName string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.commandHandlers[commandName]
}

func (r *RelationshipRegistry) GetQueryHandler(queryName string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.queryHandlers[queryName]
}

func (r *RelationshipRegistry) GetEventHandlers(eventName string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.eventHandlers[eventName]
}

func (r *RelationshipRegistry) ListCommandTypes() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]string, 0, len(r.commandHandlers))
	for k := range r.commandHandlers {
		result = append(result, k)
	}
	sort.Strings(result)
	return result
}

func (r *RelationshipRegistry) ListQueryTypes() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]string, 0, len(r.queryHandlers))
	for k := range r.queryHandlers {
		result = append(result, k)
	}
	sort.Strings(result)
	return result
}

func (r *RelationshipRegistry) ListEventTypes() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]string, 0, len(r.eventHandlers))
	for k := range r.eventHandlers {
		result = append(result, k)
	}
	sort.Strings(result)
	return result
}

func (r *RelationshipRegistry) GetHandlerEmits(handlerName string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.handlerEmits[handlerName]
}

func InferDomainFromPkgPath(pkgPath string) string {
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
