package main

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"
)

// CheckoutSession records a pending order's financials and shipping details.
type CheckoutSession struct {
	SessionID       string      `json:"session_id"`
	Cart            *Cart       `json:"cart"`
	ShippingAddress string      `json:"shipping_address"`
	Subtotal        float64     `json:"subtotal"`
	Tax             float64     `json:"tax"`
	Shipping        float64     `json:"shipping"`
	Total           float64     `json:"total"`
	Status          string      `json:"status"` // "Pending", "Completed"
}

// Order represents a completed transaction in our systems.
type Order struct {
	OrderID         string           `json:"order_id"`
	SessionID       string           `json:"session_id"`
	Cart            *Cart            `json:"cart"`
	ShippingAddress string           `json:"shipping_address"`
	Total           float64          `json:"total"`
	Status          string           `json:"status"` // "Processing", "Shipped", "Delivered"
	CreatedAt       string           `json:"created_at"`
	EstimatedDays   int              `json:"estimated_days"`
}

var (
	checkoutSessions = make(map[string]*CheckoutSession)
	orders           = make(map[string]*Order)
	ordersMu         sync.RWMutex
)

// checkoutStartHandler implements the 'checkout_start' tool.
// Begins a checkout session, computing tax, shipping, and total.
func checkoutStartHandler(ctx context.Context, args map[string]any) (any, error) {
	sessionID := getSessionID(ctx)
	address, _ := args["shipping_address"].(string)
	if address == "" {
		return nil, fmt.Errorf("missing required parameter 'shipping_address'")
	}

	cart := getOrCreateCart(sessionID)

	cartsMu.RLock()
	defer cartsMu.RUnlock()

	if len(cart.Items) == 0 {
		return nil, fmt.Errorf("your cart is empty, cannot proceed to checkout")
	}

	var subtotal float64
	for _, item := range cart.Items {
		subtotal += item.Price * float64(item.Quantity)
	}

	tax := subtotal * 0.08 // 8% sales tax
	shipping := 5.99
	if subtotal >= 49.0 {
		shipping = 0.0 // Free shipping over $49
	}

	total := subtotal + tax + shipping

	ordersMu.Lock()
	defer ordersMu.Unlock()

	checkoutSessions[sessionID] = &CheckoutSession{
		SessionID:       sessionID,
		Cart:            cart,
		ShippingAddress: address,
		Subtotal:        subtotal,
		Tax:             tax,
		Shipping:        shipping,
		Total:           total,
		Status:          "Pending",
	}

	return map[string]any{
		"message": "Checkout session started successfully.",
		"details": checkoutSessions[sessionID],
	}, nil
}

// checkoutCompleteHandler implements the 'checkout_complete' tool.
// Finalizes payment, deducts catalog inventory stock, clears the cart, and generates an order.
func checkoutCompleteHandler(ctx context.Context, args map[string]any) (any, error) {
	sessionID := getSessionID(ctx)
	paymentMethod, _ := args["payment_method"].(string)
	if paymentMethod == "" {
		return nil, fmt.Errorf("missing required parameter 'payment_method'")
	}

	ordersMu.Lock()
	session, exists := checkoutSessions[sessionID]
	ordersMu.Unlock()

	if !exists || session.Status == "Completed" {
		return nil, fmt.Errorf("no active checkout session found for this session. Please call checkout_start first")
	}

	// Verify stock levels and deduct inventory
	cartsMu.Lock()
	for _, item := range session.Cart.Items {
		for i := range Products {
			if Products[i].ID == item.ProductID {
				if Products[i].Stock < item.Quantity {
					cartsMu.Unlock()
					return nil, fmt.Errorf("insufficient stock for product %q (available: %d, requested: %d)", Products[i].Name, Products[i].Stock, item.Quantity)
				}
			}
		}
	}

	// Stock deduction
	for _, item := range session.Cart.Items {
		for i := range Products {
			if Products[i].ID == item.ProductID {
				Products[i].Stock -= item.Quantity
			}
		}
	}
	cartsMu.Unlock()

	// Generate a unique Order ID
	rand.Seed(time.Now().UnixNano())
	orderID := fmt.Sprintf("ORD-%06d", rand.Intn(1000000))

	ordersMu.Lock()
	defer ordersMu.Unlock()

	order := &Order{
		OrderID:         orderID,
		SessionID:       sessionID,
		Cart:            session.Cart,
		ShippingAddress: session.ShippingAddress,
		Total:           session.Total,
		Status:          "Processing",
		CreatedAt:       time.Now().Format("2006-01-02 15:04:05"),
		EstimatedDays:   3 + rand.Intn(4),
	}

	orders[orderID] = order
	session.Status = "Completed"

	// Reset/Clear user's cart
	cartsMu.Lock()
	carts[sessionID] = &Cart{Items: make([]*CartItem, 0)}
	cartsMu.Unlock()

	return map[string]any{
		"success":  true,
		"message":  "Checkout completed! Thank you for your purchase.",
		"order_id": orderID,
		"order":    order,
	}, nil
}

// orderStatusHandler implements the 'order_status' tool.
// Returns the current status of an order using its order ID.
func orderStatusHandler(ctx context.Context, args map[string]any) (any, error) {
	orderID, _ := args["order_id"].(string)
	if orderID == "" {
		return nil, fmt.Errorf("missing required parameter 'order_id'")
	}

	ordersMu.RLock()
	order, exists := orders[orderID]
	ordersMu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("order with ID %q not found", orderID)
	}

	return map[string]any{
		"order": order,
	}, nil
}
