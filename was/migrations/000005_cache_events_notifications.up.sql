CREATE TABLE catalog_generation(id boolean PRIMARY KEY DEFAULT true CHECK(id), version bigint NOT NULL DEFAULT 1 CHECK(version>0));
INSERT INTO catalog_generation(id) VALUES(true);
CREATE FUNCTION bump_catalog_generation() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN UPDATE catalog_generation SET version=version+1 WHERE id=true; RETURN NULL; END $$;
CREATE CONSTRAINT TRIGGER products_generation AFTER INSERT OR UPDATE OR DELETE ON products DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION bump_catalog_generation();
CREATE CONSTRAINT TRIGGER categories_generation AFTER INSERT OR UPDATE OR DELETE ON categories DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION bump_catalog_generation();
CREATE CONSTRAINT TRIGGER images_generation AFTER INSERT OR UPDATE OR DELETE ON product_images DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION bump_catalog_generation();
CREATE CONSTRAINT TRIGGER links_generation AFTER INSERT OR UPDATE OR DELETE ON product_categories DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION bump_catalog_generation();
CREATE CONSTRAINT TRIGGER inventory_generation AFTER INSERT OR UPDATE OR DELETE ON inventory DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION bump_catalog_generation();
ALTER TABLE categories ADD COLUMN version bigint NOT NULL DEFAULT 1 CHECK(version>0);
ALTER TABLE outbox_events ADD COLUMN claim_token uuid;
ALTER TABLE outbox_events ADD COLUMN next_attempt_at timestamptz NOT NULL DEFAULT now();
ALTER TABLE outbox_events ADD COLUMN last_error text NOT NULL DEFAULT '';
CREATE TABLE processed_events(consumer text NOT NULL,event_id uuid NOT NULL,processed_at timestamptz NOT NULL DEFAULT now(),PRIMARY KEY(consumer,event_id));
CREATE TABLE notifications(
 id uuid PRIMARY KEY,user_id uuid NOT NULL REFERENCES users(id),event_id uuid NOT NULL,kind text NOT NULL,title text NOT NULL,body text NOT NULL,
 order_id uuid NOT NULL REFERENCES orders(id),occurred_at timestamptz NOT NULL,aggregate_version bigint NOT NULL CHECK(aggregate_version>0),
 read_at timestamptz,created_at timestamptz NOT NULL DEFAULT now(),UNIQUE(user_id,event_id,kind)
);
CREATE INDEX notifications_owner_idx ON notifications(user_id,occurred_at,id);
CREATE INDEX notifications_unread_idx ON notifications(user_id) WHERE read_at IS NULL;
