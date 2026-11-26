CREATE TABLE coupons (
 id uuid PRIMARY KEY, code text NOT NULL UNIQUE CHECK(code=upper(btrim(code)) AND code ~ '^[A-Z0-9][A-Z0-9_-]{0,63}$'),
 kind text NOT NULL CHECK(kind IN ('Percentage','Fixed')), value bigint NOT NULL CHECK(value BETWEEN 1 AND 9000000000000),
 minimum_minor bigint NOT NULL CHECK(minimum_minor BETWEEN 0 AND 9000000000000),
 expires_at timestamptz NOT NULL, usage_limit bigint NOT NULL CHECK(usage_limit BETWEEN 1 AND 9000000000000),
 used_count bigint NOT NULL DEFAULT 0 CHECK(used_count>=0 AND used_count<=usage_limit), enabled boolean NOT NULL,
 version bigint NOT NULL DEFAULT 1 CHECK(version>0), deleted_at timestamptz,
 created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
 CHECK(kind<>'Percentage' OR value<=100)
);
CREATE INDEX coupons_created_idx ON coupons(created_at,id) WHERE deleted_at IS NULL;
ALTER TABLE orders DROP CONSTRAINT orders_discount_minor_check;
ALTER TABLE orders ADD CONSTRAINT orders_discount_minor_check CHECK(discount_minor>=0 AND discount_minor<=subtotal_minor);
ALTER TABLE orders ADD COLUMN coupon_snapshot jsonb;
CREATE TABLE coupon_usages (
 id uuid PRIMARY KEY, coupon_id uuid NOT NULL REFERENCES coupons(id), user_id uuid NOT NULL REFERENCES users(id),
 order_id uuid NOT NULL UNIQUE REFERENCES orders(id), discount_minor bigint NOT NULL CHECK(discount_minor BETWEEN 0 AND 9000000000000),
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX coupon_usages_coupon_idx ON coupon_usages(coupon_id,created_at,id);
CREATE TABLE reviews (
 id uuid PRIMARY KEY, user_id uuid NOT NULL REFERENCES users(id), product_id uuid NOT NULL REFERENCES products(id),
 order_id uuid NOT NULL REFERENCES orders(id), rating integer NOT NULL CHECK(rating BETWEEN 1 AND 5),
 content text NOT NULL CHECK(char_length(btrim(content)) BETWEEN 1 AND 2000),
 visibility text NOT NULL DEFAULT 'Visible' CHECK(visibility IN ('Visible','Hidden')),
 moderation_reason text NOT NULL DEFAULT '' CHECK(char_length(moderation_reason)<=500), moderated_by uuid REFERENCES users(id), moderated_at timestamptz,
 deleted_at timestamptz, created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(user_id,product_id)
);
CREATE INDEX reviews_public_idx ON reviews(product_id,created_at,id) WHERE visibility='Visible' AND deleted_at IS NULL;
CREATE INDEX reviews_owner_idx ON reviews(user_id,created_at,id);
CREATE INDEX reviews_admin_idx ON reviews(created_at,id) WHERE deleted_at IS NULL;
CREATE INDEX orders_created_idx ON orders(created_at,id);
CREATE INDEX products_created_idx ON products(created_at,id) WHERE deleted_at IS NULL;
