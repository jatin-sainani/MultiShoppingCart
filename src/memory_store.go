package main

import (
	"context"
	"sort"
	"sync"
	"time"
)

type MemoryStore struct {
	mu    sync.RWMutex
	carts map[int64]Cart
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{carts: make(map[int64]Cart)}
}

func (s *MemoryStore) CreateCart(_ context.Context, cart Cart) (Cart, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cart.Items = append([]CartItem(nil), cart.Items...)
	s.carts[cart.ShoppingCartID] = cart
	return cart, nil
}

func (s *MemoryStore) GetCart(_ context.Context, cartID int64) (Cart, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cart, ok := s.carts[cartID]
	if !ok {
		return Cart{}, ErrCartNotFound
	}
	return cloneCart(cart), nil
}

func (s *MemoryStore) UpsertItem(_ context.Context, cartID int64, item CartItem) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cart, ok := s.carts[cartID]
	if !ok {
		return ErrCartNotFound
	}

	updated := false
	for i := range cart.Items {
		if cart.Items[i].ProductID == item.ProductID {
			cart.Items[i].Quantity = item.Quantity
			updated = true
			break
		}
	}
	if !updated {
		cart.Items = append(cart.Items, item)
		sort.Slice(cart.Items, func(i, j int) bool {
			return cart.Items[i].ProductID < cart.Items[j].ProductID
		})
	}

	cart.UpdatedAt = time.Now().UTC()
	s.carts[cartID] = cart
	return nil
}

func (s *MemoryStore) ListCartsByCustomer(_ context.Context, customerID int64) ([]Cart, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var carts []Cart
	for _, cart := range s.carts {
		if cart.CustomerID == customerID {
			carts = append(carts, cloneCart(cart))
		}
	}

	sort.Slice(carts, func(i, j int) bool {
		return carts[i].CreatedAt.After(carts[j].CreatedAt)
	})
	return carts, nil
}

func cloneCart(cart Cart) Cart {
	cloned := cart
	cloned.Items = append([]CartItem(nil), cart.Items...)
	return cloned
}
