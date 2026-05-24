package main

import (
	"context"
	"strings"
)

// Product represents a mock item in our botanical and gardening inventory.
type Product struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Category    string  `json:"category"`
	Price       float64 `json:"price"`
	Rating      float64 `json:"rating"`
	Stock       int     `json:"stock"`
	ImageURL    string  `json:"image_url"`
}

// Products is the mock botanical database of Cymbal Home & Garden.
var Products = []Product{
	{
		ID:          "p1",
		Name:        "Red Elegance Rose Bouquet",
		Description: "A gorgeous arrangement of deep red roses, handpicked and wrapped in premium eco-friendly paper. Perfect for romantic gestures.",
		Category:    "Flowers",
		Price:       29.99,
		Rating:      4.8,
		Stock:       25,
		ImageURL:    "/images/red_roses.jpg",
	},
	{
		ID:          "p2",
		Name:        "Golden Sunset Tulip Bunch",
		Description: "Vibrant yellow and orange tulips that bring warmth and brightness to any room. Sourced directly from local sustainable farms.",
		Category:    "Flowers",
		Price:       24.99,
		Rating:      4.6,
		Stock:       30,
		ImageURL:    "/images/yellow_tulips.jpg",
	},
	{
		ID:          "p3",
		Name:        "Midnight Orchid Orchidaceae",
		Description: "An exotic and rare deep purple phalaenopsis orchid in a premium handmade ceramic pot. Elevates any modern interior.",
		Category:    "Flowers",
		Price:       39.99,
		Rating:      4.9,
		Stock:       15,
		ImageURL:    "/images/midnight_orchid.jpg",
	},
	{
		ID:          "p4",
		Name:        "Sweet Lilac Fields Lavandula",
		Description: "A rustic bundle of fresh, fragrant English lavender. Fills your space with natural soothing aromatherapy.",
		Category:    "Flowers",
		Price:       19.99,
		Rating:      4.5,
		Stock:       40,
		ImageURL:    "/images/lavender.jpg",
	},
	{
		ID:          "p5",
		Name:        "Emerald Wave Fern Nephrolepis",
		Description: "A lush, feather-leafed Boston fern that thrives in indirect sunlight and purifies the air. Housed in a self-watering planter.",
		Category:    "Plants",
		Price:       34.99,
		Rating:      4.7,
		Stock:       20,
		ImageURL:    "/images/boston_fern.jpg",
	},
	{
		ID:          "p6",
		Name:        "Zen Bonsai Ficus Microcarpa",
		Description: "A meticulously pruned 5-year-old Ficus Bonsai with a winding thick trunk and miniature emerald foliage. Promotes peace and focus.",
		Category:    "Plants",
		Price:       59.99,
		Rating:      4.9,
		Stock:       8,
		ImageURL:    "/images/zen_bonsai.jpg",
	},
	{
		ID:          "p7",
		Name:        "Pebble Garden Succulent Mix",
		Description: "A premium collection of five colorful mini succulents nestled in a sleek geometric concrete trough. Extremely low maintenance.",
		Category:    "Plants",
		Price:       14.99,
		Rating:      4.4,
		Stock:       50,
		ImageURL:    "/images/succulents.jpg",
	},
	{
		ID:          "p8",
		Name:        "NutriGrow Organic Plant Food",
		Description: "Premium 100% organic liquid nutrient formula rich in seaweed and trace minerals. Stimulates root growth and vibrant blooms.",
		Category:    "Tools",
		Price:       12.49,
		Rating:      4.6,
		Stock:       100,
		ImageURL:    "/images/organic_fertilizer.jpg",
	},
	{
		ID:          "p9",
		Name:        "Heritage Brass Watering Can",
		Description: "A classic 1.5L watering can made of solid brass with a long, slender spout for drip-free watering of delicate foliage.",
		Category:    "Tools",
		Price:       44.99,
		Rating:      4.8,
		Stock:       12,
		ImageURL:    "/images/watering_can.jpg",
	},
	{
		ID:          "p10",
		Name:        "AeroSoil Premium Organic Mix",
		Description: "Highly aerated organic soil blend with perlite, coconut coir, and worm castings. Optimized for healthy root respiration.",
		Category:    "Soil",
		Price:       9.99,
		Rating:      4.7,
		Stock:       60,
		ImageURL:    "/images/premium_soil.jpg",
	},
}

// catalogSearchHandler implements the 'catalog_search' tool.
// It filters the mock database by keywords (name and description) and/or category.
func catalogSearchHandler(ctx context.Context, args map[string]any) (any, error) {
	query, _ := args["query"].(string)
	category, _ := args["category"].(string)

	query = strings.ToLower(strings.TrimSpace(query))
	category = strings.ToLower(strings.TrimSpace(category))

	var matches []Product
	for _, p := range Products {
		if category != "" && strings.ToLower(p.Category) != category {
			continue
		}
		if query != "" {
			nameMatch := strings.Contains(strings.ToLower(p.Name), query)
			descMatch := strings.Contains(strings.ToLower(p.Description), query)
			if !nameMatch && !descMatch {
				continue
			}
		}
		matches = append(matches, p)
	}

	return map[string]any{
		"products": matches,
		"count":    len(matches),
	}, nil
}
