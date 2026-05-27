package domain

import (
	"fmt"
	"sync"
)

type Product struct {
	ID    string
	Name  string
	Price float64
	Stock int
}

type Inventory struct {
	mu       sync.RWMutex
	products map[string]*Product
}

func NewInventory() *Inventory {
	inv := &Inventory{
		products: make(map[string]*Product),
	}
	inv.seed()
	return inv
}

func (inv *Inventory) Reserve(productID string, quantity int) error {
	inv.mu.Lock()
	defer inv.mu.Unlock()
	p, ok := inv.products[productID]
	if !ok {
		return fmt.Errorf("product %s not found", productID)
	}
	if p.Stock < quantity {
		return fmt.Errorf("insufficient stock for %s: have %d, need %d", p.Name, p.Stock, quantity)
	}
	p.Stock -= quantity
	return nil
}

func (inv *Inventory) Release(productID string, quantity int) error {
	inv.mu.Lock()
	defer inv.mu.Unlock()
	p, ok := inv.products[productID]
	if !ok {
		return fmt.Errorf("product %s not found", productID)
	}
	p.Stock += quantity
	return nil
}

func (inv *Inventory) GetAll() []Product {
	inv.mu.RLock()
	defer inv.mu.RUnlock()
	result := make([]Product, 0, len(inv.products))
	for _, p := range inv.products {
		result = append(result, *p)
	}
	return result
}

func (inv *Inventory) GetByID(id string) (Product, bool) {
	inv.mu.RLock()
	defer inv.mu.RUnlock()
	p, ok := inv.products[id]
	if !ok {
		return Product{}, false
	}
	return *p, true
}

func (inv *Inventory) seed() {
	items := []Product{
		{ID: "laptop", Name: "Laptop", Price: 999.99, Stock: 10},
		{ID: "mouse", Name: "Mouse", Price: 29.99, Stock: 50},
		{ID: "keyboard", Name: "Keyboard", Price: 79.99, Stock: 30},
		{ID: "monitor", Name: "Monitor", Price: 499.99, Stock: 15},
		{ID: "headphone", Name: "Headphone", Price: 149.99, Stock: 25},
	}
	for _, item := range items {
		p := item
		inv.products[p.ID] = &p
	}
}
