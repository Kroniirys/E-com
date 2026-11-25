CREATE TABLE store_settings (
    id boolean PRIMARY KEY DEFAULT true CHECK (id),
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 120),
    currency text NOT NULL CHECK (currency = 'VND'),
    shipping_fee_minor bigint NOT NULL CHECK (shipping_fee_minor BETWEEN 0 AND 9000000000000),
    updated_at timestamptz NOT NULL DEFAULT now()
);
