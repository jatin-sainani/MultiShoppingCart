package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

type cartIDGenerator struct {
	next atomic.Int64
}

func newCartIDGenerator() *cartIDGenerator {
	g := &cartIDGenerator{}
	g.next.Store(time.Now().UTC().UnixNano())
	return g
}

func (g *cartIDGenerator) Next() int64 {
	return g.next.Add(1)
}

type Server struct {
	store CartStore
	idGen *cartIDGenerator
	clock func() time.Time
}

func NewServer(store CartStore) *Server {
	return &Server{
		store: store,
		idGen: newCartIDGenerator(),
		clock: func() time.Time { return time.Now().UTC() },
	}
}

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/shopping-carts", s.handleShoppingCarts)
	mux.HandleFunc("/shopping-carts/", s.handleCartRoutes)
	return mux
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.NotFound(w, r)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleShoppingCarts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || r.URL.Path != "/shopping-carts" {
		http.NotFound(w, r)
		return
	}

	var req CreateCartRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_INPUT", "Invalid input data", err.Error())
		return
	}
	if req.CustomerID < 1 {
		writeError(w, http.StatusBadRequest, "INVALID_INPUT", "Invalid input data", "customer_id must be a positive integer")
		return
	}

	now := s.clock()
	cart := Cart{
		ShoppingCartID: s.idGen.Next(),
		CustomerID:     req.CustomerID,
		Items:          []CartItem{},
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	created, err := s.store.CreateCart(r.Context(), cart)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "STORE_ERROR", "Failed to create cart", err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, created)
}

func (s *Server) handleCartRoutes(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/shopping-carts/")
	segments := strings.Split(path, "/")
	if len(segments) == 1 && r.Method == http.MethodGet {
		s.handleGetCart(w, r, segments[0])
		return
	}
	if len(segments) == 2 && segments[1] == "items" && r.Method == http.MethodPost {
		s.handleAddItem(w, r, segments[0])
		return
	}
	http.NotFound(w, r)
}

func (s *Server) handleGetCart(w http.ResponseWriter, r *http.Request, rawCartID string) {
	cartID, err := parsePositiveInt(rawCartID, "shoppingCartId")
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_INPUT", "Invalid shopping cart ID", err.Error())
		return
	}

	cart, err := s.store.GetCart(r.Context(), cartID)
	if err != nil {
		if errors.Is(err, ErrCartNotFound) {
			writeError(w, http.StatusNotFound, "CART_NOT_FOUND", "Shopping cart not found", "No shopping cart exists for the given ID")
			return
		}
		writeError(w, http.StatusInternalServerError, "STORE_ERROR", "Failed to retrieve cart", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, cart)
}

func (s *Server) handleAddItem(w http.ResponseWriter, r *http.Request, rawCartID string) {
	cartID, err := parsePositiveInt(rawCartID, "shoppingCartId")
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_INPUT", "Invalid shopping cart ID", err.Error())
		return
	}

	var req AddItemRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_INPUT", "Invalid input data", err.Error())
		return
	}
	if req.ProductID < 1 {
		writeError(w, http.StatusBadRequest, "INVALID_INPUT", "Invalid input data", "product_id must be a positive integer")
		return
	}
	if req.Quantity < 1 {
		writeError(w, http.StatusBadRequest, "INVALID_INPUT", "Invalid input data", "quantity must be a positive integer")
		return
	}

	if err := s.store.UpsertItem(r.Context(), cartID, CartItem{ProductID: req.ProductID, Quantity: req.Quantity}); err != nil {
		if errors.Is(err, ErrCartNotFound) {
			writeError(w, http.StatusNotFound, "CART_NOT_FOUND", "Shopping cart not found", "No shopping cart exists for the given ID")
			return
		}
		writeError(w, http.StatusInternalServerError, "STORE_ERROR", "Failed to update cart", err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func parsePositiveInt(raw, label string) (int64, error) {
	value, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil || value < 1 {
		return 0, errorsf("%s must be a positive integer", label)
	}
	return value, nil
}

func decodeJSONBody(r *http.Request, target any) error {
	defer r.Body.Close()

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return errors.New("request body must contain a single valid JSON object")
	}

	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return errors.New("request body must contain only one JSON object")
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, code, message, details string) {
	writeJSON(w, status, ErrorResponse{
		Error:   code,
		Message: message,
		Details: details,
	})
}

func newStoreFromConfig(ctx context.Context, cfg Config) (CartStore, func() error, error) {
	switch cfg.StoreBackend {
	case "memory":
		return NewMemoryStore(), func() error { return nil }, nil
	case "mysql":
		store, err := NewMySQLStore(ctx, cfg)
		if err != nil {
			return nil, nil, err
		}
		return store, store.Close, nil
	case "dynamodb":
		store, err := NewDynamoDBStore(ctx, cfg)
		if err != nil {
			return nil, nil, err
		}
		return store, func() error { return nil }, nil
	default:
		return nil, nil, errorsf("unsupported STORE_BACKEND %q", cfg.StoreBackend)
	}
}
