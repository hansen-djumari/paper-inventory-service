CREATE TYPE enum_type AS ENUM ('input', 'output');
CREATE TABLE "invoices" (
    id SERIAL PRIMARY KEY,
    created_at TIMESTAMP NOT NULL,
    types enum_type NOT NULL,
    location_id VARCHAR(255) NOT NULL,
    qty INTEGER NOT NULL,
    stock_document_type VARCHAR(255) NOT NULL,
    price NUMERIC,
    cogs NUMERIC,
    remaining_qty INTEGER,
    fifo_input_stock_movement_id INTEGER,
    fifo_input_pre_adjustment_remaining_qty INTEGER,
    sales_return_id INTEGER,
    purchase_return_id INTEGER,
    accumulated_qty INTEGER,
    accumulated_inventory_value INTEGER
);
