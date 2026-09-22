-- Create orders table.
CREATE TABLE orders (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    order_number text UNIQUE NOT NULL,
    status text NOT NULL DEFAULT 'NEW',
    accrual int DEFAULT NULL,
    uploaded_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_orders_user_id_uploaded_at ON orders (user_id, uploaded_at DESC);
CREATE INDEX idx_orders_order_number ON orders (order_number);
