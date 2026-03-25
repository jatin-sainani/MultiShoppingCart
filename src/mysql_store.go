package main

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

type MySQLStore struct {
	db *sql.DB
}

func NewMySQLStore(ctx context.Context, cfg Config) (*MySQLStore, error) {
	dsn := fmt.Sprintf(
		"%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=true&loc=UTC",
		cfg.MySQLUser,
		cfg.MySQLPassword,
		cfg.MySQLHost,
		cfg.MySQLPort,
		cfg.MySQLDatabase,
	)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(cfg.MySQLMaxOpenConns)
	db.SetMaxIdleConns(cfg.MySQLMaxIdleConns)
	db.SetConnMaxIdleTime(cfg.MySQLConnMaxIdleTime)
	db.SetConnMaxLifetime(cfg.MySQLConnMaxLifetime)

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}

	store := &MySQLStore{db: db}
	if err := store.bootstrapSchema(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *MySQLStore) Close() error {
	return s.db.Close()
}

func (s *MySQLStore) bootstrapSchema(ctx context.Context) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS shopping_carts (
			cart_id BIGINT PRIMARY KEY,
			customer_id BIGINT NOT NULL,
			created_at DATETIME(6) NOT NULL,
			updated_at DATETIME(6) NOT NULL,
			INDEX idx_shopping_carts_customer_created (customer_id, created_at DESC)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS shopping_cart_items (
			cart_id BIGINT NOT NULL,
			product_id BIGINT NOT NULL,
			quantity INT NOT NULL,
			created_at DATETIME(6) NOT NULL,
			updated_at DATETIME(6) NOT NULL,
			PRIMARY KEY (cart_id, product_id),
			CONSTRAINT fk_cart_items_cart FOREIGN KEY (cart_id)
				REFERENCES shopping_carts(cart_id)
				ON DELETE CASCADE
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
	}

	for _, stmt := range statements {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

func (s *MySQLStore) CreateCart(ctx context.Context, cart Cart) (Cart, error) {
	_, err := s.db.ExecContext(
		ctx,
		`INSERT INTO shopping_carts (cart_id, customer_id, created_at, updated_at)
		 VALUES (?, ?, ?, ?)`,
		cart.ShoppingCartID,
		cart.CustomerID,
		cart.CreatedAt.UTC(),
		cart.UpdatedAt.UTC(),
	)
	if err != nil {
		return Cart{}, err
	}
	return cart, nil
}

func (s *MySQLStore) GetCart(ctx context.Context, cartID int64) (Cart, error) {
	rows, err := s.db.QueryContext(
		ctx,
		`SELECT c.cart_id, c.customer_id, c.created_at, c.updated_at, i.product_id, i.quantity
		 FROM shopping_carts c
		 LEFT JOIN shopping_cart_items i ON i.cart_id = c.cart_id
		 WHERE c.cart_id = ?
		 ORDER BY i.product_id`,
		cartID,
	)
	if err != nil {
		return Cart{}, err
	}
	defer rows.Close()

	carts, err := scanMySQLCartRows(rows)
	if err != nil {
		return Cart{}, err
	}
	if len(carts) == 0 {
		return Cart{}, ErrCartNotFound
	}
	return carts[0], nil
}

func (s *MySQLStore) UpsertItem(ctx context.Context, cartID int64, item CartItem) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	now := time.Now().UTC()
	result, err := tx.ExecContext(
		ctx,
		`UPDATE shopping_carts SET updated_at = ? WHERE cart_id = ?`,
		now,
		cartID,
	)
	if err != nil {
		return err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrCartNotFound
	}

	_, err = tx.ExecContext(
		ctx,
		`INSERT INTO shopping_cart_items (cart_id, product_id, quantity, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?)
		 ON DUPLICATE KEY UPDATE quantity = VALUES(quantity), updated_at = VALUES(updated_at)`,
		cartID,
		item.ProductID,
		item.Quantity,
		now,
		now,
	)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (s *MySQLStore) ListCartsByCustomer(ctx context.Context, customerID int64) ([]Cart, error) {
	rows, err := s.db.QueryContext(
		ctx,
		`SELECT c.cart_id, c.customer_id, c.created_at, c.updated_at, i.product_id, i.quantity
		 FROM shopping_carts c
		 LEFT JOIN shopping_cart_items i ON i.cart_id = c.cart_id
		 WHERE c.customer_id = ?
		 ORDER BY c.created_at DESC, c.cart_id, i.product_id`,
		customerID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanMySQLCartRows(rows)
}

func scanMySQLCartRows(rows *sql.Rows) ([]Cart, error) {
	var carts []Cart
	indexByID := make(map[int64]int)

	for rows.Next() {
		var (
			cartID     int64
			customerID int64
			createdAt  time.Time
			updatedAt  time.Time
			productID  sql.NullInt64
			quantity   sql.NullInt64
		)

		if err := rows.Scan(&cartID, &customerID, &createdAt, &updatedAt, &productID, &quantity); err != nil {
			return nil, err
		}

		idx, ok := indexByID[cartID]
		if !ok {
			carts = append(carts, Cart{
				ShoppingCartID: cartID,
				CustomerID:     customerID,
				Items:          []CartItem{},
				CreatedAt:      createdAt.UTC(),
				UpdatedAt:      updatedAt.UTC(),
			})
			idx = len(carts) - 1
			indexByID[cartID] = idx
		}

		if productID.Valid && quantity.Valid {
			carts[idx].Items = append(carts[idx].Items, CartItem{
				ProductID: productID.Int64,
				Quantity:  quantity.Int64,
			})
		}
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	for i := range carts {
		sort.Slice(carts[i].Items, func(a, b int) bool {
			return carts[i].Items[a].ProductID < carts[i].Items[b].ProductID
		})
	}

	return carts, nil
}
