CREATE TABLE carts (
 id uuid PRIMARY KEY, user_id uuid NOT NULL UNIQUE REFERENCES users(id),
 version bigint NOT NULL DEFAULT 1 CHECK (version > 0)
);
CREATE TABLE cart_items (
 id uuid PRIMARY KEY, cart_id uuid NOT NULL REFERENCES carts(id), product_id uuid NOT NULL REFERENCES products(id),
 quantity bigint NOT NULL CHECK (quantity BETWEEN 1 AND 1000), UNIQUE (cart_id,product_id)
);
CREATE TABLE orders (
 id uuid PRIMARY KEY, user_id uuid NOT NULL REFERENCES users(id),
 status text NOT NULL CHECK (status IN ('Pending','Confirmed','Processing','Shipped','Delivered','Cancelled','Failed')),
 currency text NOT NULL CHECK (currency='VND'),
 subtotal_minor bigint NOT NULL CHECK (subtotal_minor BETWEEN 0 AND 9000000000000),
 discount_minor bigint NOT NULL DEFAULT 0 CHECK (discount_minor=0),
 shipping_minor bigint NOT NULL CHECK (shipping_minor BETWEEN 0 AND 9000000000000),
 total_minor bigint NOT NULL CHECK (total_minor BETWEEN 0 AND 9000000000000 AND total_minor=subtotal_minor-discount_minor+shipping_minor),
 shipping_address jsonb NOT NULL, version bigint NOT NULL DEFAULT 1 CHECK(version>0),
 created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX orders_owner_idx ON orders(user_id,created_at,id);
CREATE INDEX orders_status_idx ON orders(status,created_at,id);
CREATE TABLE order_items (
 id uuid PRIMARY KEY, order_id uuid NOT NULL REFERENCES orders(id), product_id uuid NOT NULL REFERENCES products(id),
 sku text NOT NULL, name text NOT NULL, image_url text NOT NULL,
 quantity bigint NOT NULL CHECK(quantity BETWEEN 1 AND 1000),
 unit_price_minor bigint NOT NULL CHECK(unit_price_minor BETWEEN 0 AND 9000000000000),
 line_total_minor bigint NOT NULL CHECK(line_total_minor BETWEEN 0 AND 9000000000000 AND line_total_minor=quantity*unit_price_minor),
 UNIQUE(order_id,product_id)
);
CREATE TABLE order_status_history (
 id uuid PRIMARY KEY, order_id uuid NOT NULL REFERENCES orders(id), actor_id uuid NOT NULL REFERENCES users(id),
 from_status text NOT NULL, to_status text NOT NULL, reason text NOT NULL CHECK(char_length(reason)<=500),
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX order_history_idx ON order_status_history(order_id,created_at,id);
CREATE TABLE payments (
 id uuid PRIMARY KEY, order_id uuid NOT NULL UNIQUE REFERENCES orders(id),
 method text NOT NULL CHECK(method IN ('COD','Mock')), status text NOT NULL CHECK(status IN ('Pending','Paid','Cancelled')),
 amount_minor bigint NOT NULL CHECK(amount_minor BETWEEN 0 AND 9000000000000), paid_at timestamptz
);
CREATE TABLE payment_attempts (
 id uuid PRIMARY KEY, payment_id uuid NOT NULL REFERENCES payments(id),
 status text NOT NULL CHECK(status IN ('Pending','Paid','Failed','Cancelled')),
 failure_code text NOT NULL DEFAULT '' CHECK(char_length(failure_code)<=100),
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX payment_one_pending_idx ON payment_attempts(payment_id) WHERE status='Pending';
CREATE UNIQUE INDEX payment_one_paid_idx ON payment_attempts(payment_id) WHERE status='Paid';
CREATE INDEX payment_attempts_payment_idx ON payment_attempts(payment_id,created_at,id);
CREATE TABLE outbox_events (
 id uuid PRIMARY KEY, aggregate_id uuid NOT NULL, aggregate_type text NOT NULL, type text NOT NULL,
 version integer NOT NULL CHECK(version=1), aggregate_version bigint NOT NULL CHECK(aggregate_version>0),
 correlation_id text NOT NULL, payload jsonb NOT NULL,
 occurred_at timestamptz NOT NULL DEFAULT now(), published_at timestamptz, lease_until timestamptz,
 attempts integer NOT NULL DEFAULT 0 CHECK(attempts>=0)
);
CREATE INDEX outbox_pending_idx ON outbox_events(published_at,lease_until,occurred_at);
