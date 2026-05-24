package main

import (
	"context"
	"fmt"
	"sync"
)

// CartItem represents an item inside a user's shopping cart.
type CartItem struct {
	ProductID   string  `json:"product_id"`
	ProductName string  `json:"product_name"`
	Price       float64 `json:"price"`
	Quantity    int     `json:"quantity"`
}

// Cart represents a user's shopping cart containing multiple items.
type Cart struct {
	Items []*CartItem `json:"items"`
}

var (
	carts   = make(map[string]*Cart)
	cartsMu sync.RWMutex
)

type sessionGetter interface {
	SessionID() string
}

// getSessionID extracts the active session ID from the tool context or fallback context.
func getSessionID(ctx context.Context) string {
	if getter, ok := ctx.(sessionGetter); ok {
		return getter.SessionID()
	}
	return "default-session"
}

// getOrCreateCart fetches or creates the active shopping cart for a given session.
func getOrCreateCart(sessionID string) *Cart {
	cartsMu.Lock()
	defer cartsMu.Unlock()
	if c, ok := carts[sessionID]; ok {
		return c
	}
	c := &Cart{Items: make([]*CartItem, 0)}
	carts[sessionID] = c
	return c
}

// cartAddHandler implements the 'cart_add' tool.
// Adds a product to the user's cart while checking stock limits.
func cartAddHandler(ctx context.Context, args map[string]any) (any, error) {
	sessionID := getSessionID(ctx)
	productID, _ := args["product_id"].(string)
	if productID == "" {
		return nil, fmt.Errorf("missing required parameter 'product_id'")
	}

	qtyVal, exists := args["quantity"]
	quantity := 1
	if exists {
		switch v := qtyVal.(type) {
		case int:
			quantity = v
		case float64:
			quantity = int(v)
		}
	}

	if quantity <= 0 {
		return nil, fmt.Errorf("quantity must be greater than zero")
	}

	// Find the product in our catalog to retrieve details and check stock
	var targetProduct *Product
	for i := range Products {
		if Products[i].ID == productID {
			targetProduct = &Products[i]
			break
		}
	}

	if targetProduct == nil {
		return nil, fmt.Errorf("product with ID %q not found", productID)
	}

	if targetProduct.Stock < quantity {
		return nil, fmt.Errorf("insufficient stock for product %q (requested: %d, available: %d)", targetProduct.Name, quantity, targetProduct.Stock)
	}

	cart := getOrCreateCart(sessionID)

	cartsMu.Lock()
	defer cartsMu.Unlock()

	// Check if the product is already in the cart
	var existingItem *CartItem
	for _, item := range cart.Items {
		if item.ProductID == productID {
			existingItem = item
			break
		}
	}

	if existingItem != nil {
		if targetProduct.Stock < existingItem.Quantity+quantity {
			return nil, fmt.Errorf("cannot add %d more units (requested total: %d, available stock: %d)", quantity, existingItem.Quantity+quantity, targetProduct.Stock)
		}
		existingItem.Quantity += quantity
	} else {
		cart.Items = append(cart.Items, &CartItem{
			ProductID:   productID,
			ProductName: targetProduct.Name,
			Price:       targetProduct.Price,
			Quantity:    quantity,
		})
	}

	return map[string]any{
		"success": true,
		"message": fmt.Sprintf("Added %d units of %q to your cart.", quantity, targetProduct.Name),
		"cart":    cart,
	}, nil
}

// cartViewHandler implements the 'cart_view' tool.
// Returns the items and the computed total for the user's cart.
func cartViewHandler(ctx context.Context, _ map[string]any) (any, error) {
	sessionID := getSessionID(ctx)
	cart := getOrCreateCart(sessionID)

	cartsMu.RLock()
	defer cartsMu.RUnlock()

	var total float64
	for _, item := range cart.Items {
		total += item.Price * float64(item.Quantity)
	}

	return map[string]any{
		"cart":  cart,
		"total": total,
	}, nil
}
