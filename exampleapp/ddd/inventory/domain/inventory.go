package domain

import (
	"fmt"
	"sync"

	orderdomain "github.com/ddd-qce/exampleapp/ddd/order/domain"
)

type Product struct {
	ID    orderdomain.ProductID
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

func (inv *Inventory) Reserve(productID orderdomain.ProductID, quantity int) error {
	inv.mu.Lock()
	defer inv.mu.Unlock()
	p, ok := inv.products[productID.String()]
	if !ok {
		return fmt.Errorf("product %s not found", productID)
	}
	if p.Stock < quantity {
		return fmt.Errorf("insufficient stock for %s: have %d, need %d", p.Name, p.Stock, quantity)
	}
	p.Stock -= quantity
	return nil
}

func (inv *Inventory) Release(productID orderdomain.ProductID, quantity int) error {
	inv.mu.Lock()
	defer inv.mu.Unlock()
	p, ok := inv.products[productID.String()]
	if !ok {
		return fmt.Errorf("product %s not found", productID)
	}
	p.Stock += quantity
	return nil
}

func (inv *Inventory) AddStock(productID orderdomain.ProductID, quantity int) error {
	inv.mu.Lock()
	defer inv.mu.Unlock()
	p, ok := inv.products[productID.String()]
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

func (inv *Inventory) GetByID(id orderdomain.ProductID) (Product, bool) {
	inv.mu.RLock()
	defer inv.mu.RUnlock()
	p, ok := inv.products[id.String()]
	if !ok {
		return Product{}, false
	}
	return *p, true
}

func (inv *Inventory) seed() {
	items := DefaultProducts()
	for _, item := range items {
		p := item
		inv.products[p.ID.String()] = &p
	}
}

func DefaultProducts() []Product {
	return []Product{
		{ID: orderdomain.ProductID("laptop"), Name: "Laptop", Price: 999.99, Stock: 10},
		{ID: orderdomain.ProductID("mouse"), Name: "Mouse", Price: 29.99, Stock: 50},
		{ID: orderdomain.ProductID("keyboard"), Name: "Keyboard", Price: 79.99, Stock: 30},
		{ID: orderdomain.ProductID("monitor"), Name: "Monitor", Price: 499.99, Stock: 15},
		{ID: orderdomain.ProductID("headphone"), Name: "Headphone", Price: 149.99, Stock: 25},
	}
}
