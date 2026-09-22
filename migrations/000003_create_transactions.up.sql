-- Create transactions table.
CREATE TABLE transactions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    order_id uuid REFERENCES orders(id) ON DELETE SET NULL,
    type text NOT NULL,
    points int NOT NULL,
    order_discount decimal DEFAULT NULL,
    description text,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_transactions_user_id_created_at ON transactions (user_id, created_at DESC);
