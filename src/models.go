package main

import (
	"context"
	"errors"
	"time"
)

var ErrCartNotFound = errors.New("shopping cart not found")

type CartItem struct {
	ProductID int64 `json:"product_id"`
	Quantity  int64 `json:"quantity"`
}

type Cart struct {
	ShoppingCartID int64      `json:"shopping_cart_id"`
	CustomerID     int64      `json:"customer_id"`
	Items          []CartItem `json:"items"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

type CreateCartRequest struct {
	CustomerID int64 `json:"customer_id"`
}

type AddItemRequest struct {
	ProductID int64 `json:"product_id"`
	Quantity  int64 `json:"quantity"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

type CartStore interface {
	CreateCart(ctx context.Context, cart Cart) (Cart, error)
	GetCart(ctx context.Context, cartID int64) (Cart, error)
	UpsertItem(ctx context.Context, cartID int64, item CartItem) error
	ListCartsByCustomer(ctx context.Context, customerID int64) ([]Cart, error)
}
