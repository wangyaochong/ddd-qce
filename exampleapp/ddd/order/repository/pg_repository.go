package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	orderdomain "github.com/ddd-qce/exampleapp/ddd/order/domain"
)

type PgOrderRepository struct {
	db *sql.DB
}

func NewPgOrderRepository(db *sql.DB) *PgOrderRepository {
	return &PgOrderRepository{db: db}
}

func (r *PgOrderRepository) Save(ctx context.Context, order *orderdomain.Order) error {
	data, err := json.Marshal(order)
	if err != nil {
		return fmt.Errorf("marshal order: %w", err)
	}
	root := order.GetAggregateRoot()
	version := root.Version()

	var existingVersion int
	err = r.db.QueryRowContext(ctx,
		`SELECT version FROM ddd_aggregate_snapshots WHERE aggregate_id = $1 AND aggregate_type = 'Order'`,
		order.ID(),
	).Scan(&existingVersion)

	if err == sql.ErrNoRows {
		_, err = r.db.ExecContext(ctx,
			`INSERT INTO ddd_aggregate_snapshots (aggregate_id, aggregate_type, snapshot_data, version, updated_at)
			 VALUES ($1, 'Order', $2, $3, NOW())`,
			order.ID(), data, version,
		)
		return err
	}
	if err != nil {
		return fmt.Errorf("query existing order: %w", err)
	}

	_, err = r.db.ExecContext(ctx,
		`UPDATE ddd_aggregate_snapshots SET snapshot_data = $2, version = $3, updated_at = NOW()
		 WHERE aggregate_id = $1 AND aggregate_type = 'Order'`,
		order.ID(), data, version,
	)
	return err
}

func (r *PgOrderRepository) FindByID(ctx context.Context, id string) (*orderdomain.Order, error) {
	var data []byte
	err := r.db.QueryRowContext(ctx,
		`SELECT snapshot_data FROM ddd_aggregate_snapshots WHERE aggregate_id = $1 AND aggregate_type = 'Order'`,
		id,
	).Scan(&data)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("order %s not found", id)
	}
	if err != nil {
		return nil, err
	}
	order := orderdomain.NewOrderForReplay(orderdomain.OrderID(id))
	if err := json.Unmarshal(data, order); err != nil {
		return nil, fmt.Errorf("unmarshal order: %w", err)
	}
	return order, nil
}

func (r *PgOrderRepository) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM ddd_aggregate_snapshots WHERE aggregate_id = $1 AND aggregate_type = 'Order'`,
		id,
	)
	return err
}

func (r *PgOrderRepository) FindAll(ctx context.Context) ([]*orderdomain.Order, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT aggregate_id, snapshot_data FROM ddd_aggregate_snapshots WHERE aggregate_type = 'Order' ORDER BY aggregate_id`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []*orderdomain.Order
	for rows.Next() {
		var id string
		var data []byte
		if err := rows.Scan(&id, &data); err != nil {
			return nil, err
		}
		order := orderdomain.NewOrderForReplay(orderdomain.OrderID(id))
		if err := json.Unmarshal(data, order); err != nil {
			return nil, fmt.Errorf("unmarshal order %s: %w", id, err)
		}
		orders = append(orders, order)
	}
	return orders, rows.Err()
}

var _ OrderRepositoryAdapter = (*PgOrderRepository)(nil)
