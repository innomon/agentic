# Cymbal Retail Agent (Botanical Floral Commerce Demo)

This is a premium, self-contained implementation of the **Cymbal Retail Agent** (Cymbal Home & Garden floral shop) built entirely within this directory as a standalone Go executable using Google's official **ADK-Go** framework and standard A2A launchers.

This example converts the core concepts of the **Universal Commerce Protocol (UCP) A2A** samples into our Go agent framework, showcasing how conversational agents discover catalogs, manage thread-safe shopping carts, compute taxes/shipping, and process checkouts.

---

## Capabilities & Tools

The agent is equipped with **6 specialized commerce tools** registered dynamically at startup:

1. `catalog_search`: Filters and searches the botanical database by query keywords or product category.
2. `cart_add`: Adds a selected product (with desired quantity) to the user's shopping cart while checking real-time stock levels.
3. `cart_view`: Reviews items currently in the cart and computes the running subtotal.
4. `checkout_start`: Initiates the checkout session for the shipping address, calculating an **8% sales tax** and **shipping fees** ($5.99 flat, free for orders $49+).
5. `checkout_complete`: Processes payment, deducts stock from inventory, generates a unique Order ID (`ORD-XXXXXX`), and clears the cart.
6. `order_status`: Tracks an order and returns its status (`Processing`, `Shipped`, `Delivered`).

---

## Botanical Inventory

Our boutique "Cymbal Home & Garden" features the following premium catalog:

| ID | Product Name | Category | Price | Stock | Description |
|---|---|---|---|---|---|
| `p1` | Red Elegance Rose Bouquet | Flowers | $29.99 | 25 | Handpicked deep red roses, eco-friendly wrap. |
| `p2` | Golden Sunset Tulip Bunch | Flowers | $24.99 | 30 | Direct from local sustainable farms. |
| `p3` | Midnight Orchid Orchidaceae | Flowers | $39.99 | 15 | Rare phalaenopsis in handmade ceramic pot. |
| `p4` | Sweet Lilac Fields Lavandula | Flowers | $19.99 | 40 | Fragrant English lavender aromatherapy. |
| `p5` | Emerald Wave Fern Nephrolepis | Plants | $34.99 | 20 | Air-purifying Boston fern in self-watering pot. |
| `p6` | Zen Bonsai Ficus Microcarpa | Plants | $59.99 | 8 | 5-year-old Bonsai with miniature emerald foliage. |
| `p7` | Pebble Garden Succulent Mix | Plants | $14.99 | 50 | Colorful succulents in geometric concrete trough. |
| `p8` | NutriGrow Organic Plant Food | Tools | $12.49 | 100 | 100% organic liquid nutrient formula. |
| `p9` | Heritage Brass Watering Can | Tools | $44.99 | 12 | Classic 1.5L solid brass watering can. |
| `p10` | AeroSoil Premium Organic Mix | Soil | $9.99 | 60 | Perlite, coco coir, and worm castings mix. |

---

## Build & Run

Since the example is completely self-contained in this folder, you can build and compile the standalone launcher using standard Go tooling:

### 1. Compile the Standalone Binary
```bash
cd examples/cymbal-retail-agent
go build -o cymbal-agent .
```

### 2. Run in Console Mode
Launch the interactive command-line interface:
```bash
./cymbal-agent console
```
You can converse with the botanical expert and test tool calling:
```
User -> Do you have any orchids or bonsai trees?
User -> Add 1 Midnight Orchid and 1 Zen Bonsai to my cart please
User -> Show me my cart
User -> Start checkout and ship to 456 Greenhouse Rd
User -> Complete checkout with Mock Card
User -> Track order ORD-XXXXXX
```

### 3. Run in Web UI Mode
Start the browser-based chat interface:
```bash
./cymbal-agent -webui web
```
Then navigate your browser to `http://localhost:8080/ui` to talk to the retail agent inside the visual interface.

### 4. Run in A2A Mode
Start the A2A server and expose the agent to the standard Agent-to-Agent protocol on port `10999` (compatible with the standard UCP chat-client):
```bash
./cymbal-agent -a2a -port 10999 web
```
The server will start at `http://localhost:10999/` and publish its standardized Agent Card containing UCP capability metadata for tool mapping and capability discovery!

### 5. Running with UCP Chat Client
You can run the official React-based UCP Chat Client from [Universal-Commerce-Protocol/samples](https://github.com/Universal-Commerce-Protocol/samples/tree/main/a2a/chat-client) to interact with our Go-based A2A retail agent visually:

1. **Start the Go A2A Agent on Port 10999:**
   The chat client is pre-configured to look for the backend agent on port `10999` by default. Launch our compiled Go agent in A2A mode:
   ```bash
   ./cymbal-agent -a2a -port 10999 web
   ```

2. **Clone and Setup the UCP Chat Client:**
   Open a new terminal window, clone the samples repository, and navigate to the chat client directory:
   ```bash
   git clone https://github.com/Universal-Commerce-Protocol/samples.git
   cd samples/a2a/chat-client
   ```

3. **Install Dependencies & Launch the Client:**
   Use Node.js package manager to start the Vite-powered React client:
   ```bash
   npm install
   npm run dev
   ```

4. **Interact in the Browser:**
   Open your browser to `http://localhost:3000/` (or the address output by Vite). The client will perform the standard A2A handshake with our Go-based `cymbal-agent` to discover capabilities and start a session. You can now chat, search the catalog, add products to the cart, and checkout in a highly interactive visual frontend!

