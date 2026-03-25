package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

type failingStore struct {
	err error
}

func (s *failingStore) CreateCart(context.Context, Cart) (Cart, error) {
	return Cart{}, s.err
}

func (s *failingStore) GetCart(context.Context, int64) (Cart, error) {
	return Cart{}, s.err
}

func (s *failingStore) UpsertItem(context.Context, int64, CartItem) error {
	return s.err
}

func (s *failingStore) ListCartsByCustomer(context.Context, int64) ([]Cart, error) {
	return nil, s.err
}

func TestCreateCartThenGetCart(t *testing.T) {
	store := NewMemoryStore()
	server := NewServer(store)

	createReq := httptest.NewRequest(http.MethodPost, "/shopping-carts", strings.NewReader(`{"customer_id":123}`))
	createRec := httptest.NewRecorder()
	server.routes().ServeHTTP(createRec, createReq)

	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", createRec.Code)
	}

	var created Cart
	if err := json.NewDecoder(createRec.Body).Decode(&created); err != nil {
		t.Fatalf("decode cart: %v", err)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/shopping-carts/"+strconv.FormatInt(created.ShoppingCartID, 10), nil)
	getRec := httptest.NewRecorder()
	server.routes().ServeHTTP(getRec, getReq)

	if getRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", getRec.Code)
	}
}

func TestAddItemUpsertsWithoutDuplicates(t *testing.T) {
	store := NewMemoryStore()
	server := NewServer(store)
	created := createCartForTest(t, server, 77)

	addReq := httptest.NewRequest(http.MethodPost, "/shopping-carts/"+strconv.FormatInt(created.ShoppingCartID, 10)+"/items", strings.NewReader(`{"product_id":10,"quantity":2}`))
	addRec := httptest.NewRecorder()
	server.routes().ServeHTTP(addRec, addReq)

	if addRec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", addRec.Code)
	}

	updateReq := httptest.NewRequest(http.MethodPost, "/shopping-carts/"+strconv.FormatInt(created.ShoppingCartID, 10)+"/items", strings.NewReader(`{"product_id":10,"quantity":7}`))
	updateRec := httptest.NewRecorder()
	server.routes().ServeHTTP(updateRec, updateReq)

	if updateRec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", updateRec.Code)
	}

	cart, err := store.GetCart(context.Background(), created.ShoppingCartID)
	if err != nil {
		t.Fatalf("get cart from store: %v", err)
	}
	if len(cart.Items) != 1 {
		t.Fatalf("expected one item, got %d", len(cart.Items))
	}
	if cart.Items[0].Quantity != 7 {
		t.Fatalf("expected quantity 7, got %d", cart.Items[0].Quantity)
	}
}

func TestMissingCartReturns404(t *testing.T) {
	server := NewServer(NewMemoryStore())
	req := httptest.NewRequest(http.MethodGet, "/shopping-carts/999", nil)
	rec := httptest.NewRecorder()
	server.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestInvalidPayloadReturns400(t *testing.T) {
	server := NewServer(NewMemoryStore())
	req := httptest.NewRequest(http.MethodPost, "/shopping-carts", strings.NewReader(`{"customer_id":0}`))
	rec := httptest.NewRecorder()
	server.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestAddItemToMissingCartReturns404(t *testing.T) {
	server := NewServer(NewMemoryStore())
	req := httptest.NewRequest(http.MethodPost, "/shopping-carts/123/items", strings.NewReader(`{"product_id":4,"quantity":1}`))
	rec := httptest.NewRecorder()
	server.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestResponseShapeUsesExpectedFields(t *testing.T) {
	server := NewServer(NewMemoryStore())
	created := createCartForTest(t, server, 88)

	req := httptest.NewRequest(http.MethodGet, "/shopping-carts/"+strconv.FormatInt(created.ShoppingCartID, 10), nil)
	rec := httptest.NewRecorder()
	server.routes().ServeHTTP(rec, req)

	var payload map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode map: %v", err)
	}

	expectedFields := []string{"shopping_cart_id", "customer_id", "items", "created_at", "updated_at"}
	for _, field := range expectedFields {
		if _, ok := payload[field]; !ok {
			t.Fatalf("expected field %q in payload: %+v", field, payload)
		}
	}
}

func TestStoreErrorsReturn500(t *testing.T) {
	server := NewServer(&failingStore{err: errors.New("boom")})
	req := httptest.NewRequest(http.MethodPost, "/shopping-carts", strings.NewReader(`{"customer_id":5}`))
	rec := httptest.NewRecorder()
	server.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}

func createCartForTest(t *testing.T, server *Server, customerID int64) Cart {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/shopping-carts", strings.NewReader(`{"customer_id":`+strconv.FormatInt(customerID, 10)+`}`))
	rec := httptest.NewRecorder()
	server.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected create cart 201, got %d", rec.Code)
	}

	var cart Cart
	if err := json.NewDecoder(rec.Body).Decode(&cart); err != nil {
		t.Fatalf("decode created cart: %v", err)
	}
	return cart
}
