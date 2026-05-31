package domain

import (
	"context"
	"database/sql"
	"fmt"
)

type ProductStore interface {
	LoadAll(ctx context.Context) ([]Product, error)
	Save(ctx context.Context, p Product) error
	UpdateStock(ctx context.Context, id string, stock int) error
}

type InMemoryProductStore struct {
	products map[string]Product
}

func NewInMemoryProductStore(products []Product) *InMemoryProductStore {
	m := &InMemoryProductStore{products: make(map[string]Product)}
	for _, p := range products {
		m.products[p.ID.String()] = p
	}
	return m
}

func (m *InMemoryProductStore) LoadAll(_ context.Context) ([]Product, error) {
	result := make([]Product, 0, len(m.products))
	for _, p := range m.products {
		result = append(result, p)
	}
	return result, nil
}

func (m *InMemoryProductStore) Save(_ context.Context, p Product) error {
	m.products[p.ID.String()] = p
	return nil
}

func (m *InMemoryProductStore) UpdateStock(_ context.Context, id string, stock int) error {
	p, ok := m.products[id]
	if !ok {
		return fmt.Errorf("product %s not found", id)
	}
	p.Stock = stock
	m.products[id] = p
	return nil
}

type PgProductStore struct {
	db *sql.DB
}

func NewPgProductStore(db *sql.DB) *PgProductStore {
	return &PgProductStore{db: db}
}

func (s *PgProductStore) LoadAll(ctx context.Context) ([]Product, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, price, stock FROM ddd_inventory_products ORDER BY id`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var products []Product
	for rows.Next() {
		var p Product
		if err := rows.Scan(&p.ID, &p.Name, &p.Price, &p.Stock); err != nil {
			return nil, err
		}
		products = append(products, p)
	}
	return products, rows.Err()
}

func (s *PgProductStore) Save(ctx context.Context, p Product) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO ddd_inventory_products (id, name, price, stock) VALUES ($1, $2, $3, $4)
		 ON CONFLICT (id) DO UPDATE SET name = $2, price = $3, stock = $4`,
		p.ID.String(), p.Name, p.Price, p.Stock,
	)
	return err
}

func (s *PgProductStore) UpdateStock(ctx context.Context, id string, stock int) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE ddd_inventory_products SET stock = $2 WHERE id = $1`,
		id, stock,
	)
	return err
}

func SeedProducts(ctx context.Context, store ProductStore, products []Product) error {
	for _, p := range products {
		if err := store.Save(ctx, p); err != nil {
			return fmt.Errorf("seed product %s: %w", p.ID, err)
		}
	}
	return nil
}
