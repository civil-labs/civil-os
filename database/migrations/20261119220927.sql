-- Create extension "pg_trgm"
CREATE EXTENSION "pg_trgm" WITH SCHEMA "extensions" VERSION "1.6";
-- Create "search_normalize" function
CREATE FUNCTION "core"."search_normalize" ("input" text) RETURNS text LANGUAGE sql STRICT IMMUTABLE PARALLEL SAFE AS $$ SELECT pg_catalog.lower(pg_catalog.btrim(pg_catalog.regexp_replace(input, '[^[:alnum:]]+', ' ', 'g'))) $$;
