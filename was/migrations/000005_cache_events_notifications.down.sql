DO $$ BEGIN IF EXISTS(SELECT 1 FROM processed_events) OR EXISTS(SELECT 1 FROM notifications) THEN RAISE EXCEPTION 'Refusing to discard inbox/notification history';END IF; END $$;
DROP TABLE notifications;DROP TABLE processed_events;
ALTER TABLE outbox_events DROP COLUMN last_error,DROP COLUMN next_attempt_at,DROP COLUMN claim_token;
ALTER TABLE categories DROP COLUMN version;
DROP TRIGGER inventory_generation ON inventory;DROP TRIGGER links_generation ON product_categories;DROP TRIGGER images_generation ON product_images;DROP TRIGGER categories_generation ON categories;DROP TRIGGER products_generation ON products;
DROP FUNCTION bump_catalog_generation();DROP TABLE catalog_generation;
