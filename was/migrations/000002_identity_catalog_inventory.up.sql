CREATE TABLE roles (
    id uuid PRIMARY KEY,
    code text NOT NULL UNIQUE CHECK (code IN ('Customer', 'Admin'))
);
INSERT INTO roles (id, code) VALUES
('00000000-0000-0000-0000-000000000001', 'Customer'),
('00000000-0000-0000-0000-000000000002', 'Admin');

CREATE TABLE users (
    id uuid PRIMARY KEY,
    email text NOT NULL UNIQUE CHECK (email = lower(btrim(email)) AND char_length(email) <= 254),
    name text NOT NULL CHECK (char_length(btrim(name)) BETWEEN 1 AND 120),
    password_hash text NOT NULL,
    role_id uuid NOT NULL REFERENCES roles(id),
    status text NOT NULL DEFAULT 'Active' CHECK (status IN ('Active', 'Disabled')),
    session_version bigint NOT NULL DEFAULT 1 CHECK (session_version > 0),
    deleted_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX users_created_idx ON users(created_at, id);

CREATE TABLE auth_sessions (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id),
    user_version bigint NOT NULL,
    expires_at timestamptz NOT NULL,
    revoked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX auth_sessions_user_idx ON auth_sessions(user_id);
CREATE TABLE refresh_tokens (
    id uuid PRIMARY KEY,
    family_id uuid NOT NULL REFERENCES auth_sessions(id),
    token_hash text NOT NULL UNIQUE,
    expires_at timestamptz NOT NULL,
    used_at timestamptz,
    replaced_by uuid REFERENCES refresh_tokens(id),
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX refresh_family_idx ON refresh_tokens(family_id);

CREATE TABLE user_addresses (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id),
    recipient text NOT NULL CHECK (char_length(btrim(recipient)) BETWEEN 1 AND 120),
    phone text NOT NULL CHECK (char_length(phone) BETWEEN 8 AND 20),
    address text NOT NULL CHECK (char_length(btrim(address)) BETWEEN 1 AND 500),
    is_default boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX user_addresses_owner_idx ON user_addresses(user_id);
CREATE UNIQUE INDEX address_one_default_idx ON user_addresses(user_id) WHERE is_default;

CREATE TABLE categories (
    id uuid PRIMARY KEY,
    name text NOT NULL CHECK (char_length(btrim(name)) BETWEEN 1 AND 120),
    slug text NOT NULL UNIQUE CHECK (slug ~ '^[a-z0-9]+(-[a-z0-9]+)*$'),
    deleted_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE products (
    id uuid PRIMARY KEY,
    sku text NOT NULL UNIQUE CHECK (sku = upper(btrim(sku)) AND char_length(sku) BETWEEN 1 AND 64),
    name text NOT NULL CHECK (char_length(btrim(name)) BETWEEN 1 AND 200),
    description text NOT NULL DEFAULT '' CHECK (char_length(description) <= 10000),
    price_minor bigint NOT NULL CHECK (price_minor BETWEEN 0 AND 9000000000000),
    original_price_minor bigint NOT NULL CHECK (original_price_minor BETWEEN 0 AND 9000000000000),
    status text NOT NULL CHECK (status IN ('Draft', 'Active', 'Inactive')),
    version bigint NOT NULL DEFAULT 1,
    deleted_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX products_public_idx ON products(status, created_at, id) WHERE deleted_at IS NULL;
CREATE TABLE product_images (
    id uuid PRIMARY KEY,
    product_id uuid NOT NULL REFERENCES products(id),
    url text NOT NULL,
    alt text NOT NULL DEFAULT '',
    sort_order integer NOT NULL CHECK (sort_order >= 0)
);
CREATE INDEX images_product_idx ON product_images(product_id, sort_order, id);
CREATE TABLE product_categories (
    product_id uuid NOT NULL REFERENCES products(id),
    category_id uuid NOT NULL REFERENCES categories(id),
    PRIMARY KEY (product_id, category_id)
);
CREATE INDEX product_categories_category_idx ON product_categories(category_id, product_id);

CREATE TABLE inventory (
    product_id uuid PRIMARY KEY REFERENCES products(id),
    quantity bigint NOT NULL DEFAULT 0 CHECK (quantity BETWEEN 0 AND 9000000000000),
    version bigint NOT NULL DEFAULT 1,
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE inventory_transactions (
    id uuid PRIMARY KEY,
    product_id uuid NOT NULL REFERENCES products(id),
    actor_id uuid REFERENCES users(id),
    delta bigint NOT NULL CHECK (delta <> 0),
    before_quantity bigint NOT NULL CHECK (before_quantity >= 0),
    after_quantity bigint NOT NULL CHECK (after_quantity >= 0),
    reason text NOT NULL CHECK (char_length(btrim(reason)) BETWEEN 1 AND 500),
    reference text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (product_id, reason, reference),
    CHECK (after_quantity = before_quantity + delta)
);
CREATE INDEX inventory_history_idx ON inventory_transactions(product_id, created_at, id);
CREATE TABLE idempotency_records (
    id uuid PRIMARY KEY,
    actor_id uuid NOT NULL REFERENCES users(id),
    operation text NOT NULL,
    key text NOT NULL CHECK (char_length(key) BETWEEN 16 AND 128),
    payload_hash text NOT NULL,
    response_status integer,
    response_body jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (actor_id, operation, key)
);
