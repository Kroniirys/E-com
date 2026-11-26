DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM orders WHERE coupon_snapshot IS NOT NULL OR discount_minor<>0)
 OR EXISTS(SELECT 1 FROM reviews) OR EXISTS(SELECT 1 FROM coupon_usages)
 THEN RAISE EXCEPTION 'Refusing to discard coupon or review business data; use a forward migration'; END IF;
END $$;
DROP INDEX products_created_idx;
DROP INDEX orders_created_idx;
DROP TABLE reviews;
DROP TABLE coupon_usages;
ALTER TABLE orders DROP COLUMN coupon_snapshot;
ALTER TABLE orders DROP CONSTRAINT orders_discount_minor_check;
ALTER TABLE orders ADD CONSTRAINT orders_discount_minor_check CHECK(discount_minor=0);
DROP TABLE coupons;
