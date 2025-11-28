CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- UPSTREAM trigger for updating data version
CREATE OR REPLACE FUNCTION array_compare_as_set(arr1 anyarray, arr2 anyarray) RETURNS boolean AS $$
SELECT CASE
           WHEN array_dims(arr1) <> array_dims(arr2) THEN 'f'
           WHEN array_length(arr1,1) <> array_length(arr2,1) THEN 'f'
           ELSE NOT EXISTS (
               SELECT 1
               FROM unnest(arr1) a
                        FULL JOIN unnest(arr2) b ON (a=b)
               WHERE a IS NULL or b IS NULL
           )
           END
$$ LANGUAGE SQL IMMUTABLE;

CREATE OR REPLACE FUNCTION upstreams_update_trigger() RETURNS TRIGGER AS $$
DECLARE
    old_keys text[];
    new_keys text[];
BEGIN
    IF OLD.endpoints IS DISTINCT FROM NEW.endpoints THEN
        SELECT array_agg(key) INTO old_keys
        FROM (SELECT jsonb_object_keys(COALESCE(OLD.endpoints, '{}'::jsonb)) AS key) keys_old;

        SELECT array_agg(key) INTO new_keys
        FROM (SELECT jsonb_object_keys(COALESCE(NEW.endpoints, '{}'::jsonb)) AS key) keys_new;

        IF NOT array_compare_as_set(
                COALESCE(old_keys, '{}'::text[]),
                COALESCE(new_keys, '{}'::text[])
               ) THEN
            UPDATE state_versions
            SET version_id = uuid_generate_v4(), updated_at = NOW()
            WHERE type_id = 1;
        END IF;
    END IF;

    IF OLD.id IS DISTINCT FROM NEW.id THEN
        UPDATE state_versions
        SET version_id = uuid_generate_v4(), updated_at = NOW()
        WHERE type_id = 1;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS upstreams_update ON upstreams;

CREATE TRIGGER upstreams_update
    AFTER UPDATE ON upstreams
    FOR EACH ROW
EXECUTE FUNCTION upstreams_update_trigger();

-- UPSTREAM sql end

-- PORTS trigger for updating data version
CREATE OR REPLACE FUNCTION ports_insert_delete_trigger()
    RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        UPDATE state_versions set version_id=uuid_generate_v4(), updated_at=NOW() where type_id = 1;
        RETURN NEW;

    ELSIF TG_OP = 'DELETE' THEN
        UPDATE state_versions set version_id=uuid_generate_v4(), updated_at=NOW() where type_id = 1;
        RETURN OLD;
    END IF;

    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS insert_delete_ports ON ports;

CREATE TRIGGER insert_delete_ports
    AFTER INSERT OR DELETE ON ports
    FOR EACH ROW
EXECUTE FUNCTION ports_insert_delete_trigger();
-- PORTS sql ends

-- PROTECTIONS triggers
CREATE OR REPLACE FUNCTION insert_update_delete_protection() RETURNS TRIGGER AS $$
BEGIN
    CASE TG_OP
        WHEN 'INSERT' THEN
            UPDATE state_versions set version_id=uuid_generate_v4(), updated_at=NOW() where type_id = 1;
            RETURN NEW;
        WHEN 'UPDATE' THEN
            UPDATE state_versions set version_id=uuid_generate_v4(), updated_at=NOW() where type_id = 1;
            RETURN NEW;
        WHEN 'DELETE' THEN
            UPDATE state_versions set version_id=uuid_generate_v4(), updated_at=NOW() where type_id = 1;
            RETURN OLD;
        END CASE;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS insert_update_delete_protections ON protections;

CREATE TRIGGER insert_update_delete_protections
    AFTER INSERT OR UPDATE OR DELETE ON protections
    FOR EACH ROW
EXECUTE FUNCTION insert_update_delete_protection();
-- PROTECTION triggers end