-- Landing pages: structured-template content per hotel per locale.
-- Layout/structure is fixed in code; only content + branding + sections + SEO
-- + tracking are stored. Sections are JSON-typed so we can evolve the catalog
-- without DB churn.

CREATE TABLE landing_pages (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  hotel_id    UUID NOT NULL REFERENCES hotels(id) ON DELETE CASCADE,
  locale      VARCHAR(10) NOT NULL,         -- "th", "en", ...
  status      VARCHAR(20) NOT NULL DEFAULT 'draft'
                CHECK (status IN ('draft','published')),
  version     INT NOT NULL DEFAULT 1,

  branding    JSONB NOT NULL DEFAULT '{}'::jsonb,
  sections    JSONB NOT NULL DEFAULT '[]'::jsonb,
  seo         JSONB NOT NULL DEFAULT '{}'::jsonb,
  tracking    JSONB NOT NULL DEFAULT '{}'::jsonb,

  published_at TIMESTAMPTZ,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),

  UNIQUE (hotel_id, locale)
);

-- Reuses set_updated_at() from 0001.
CREATE TRIGGER trg_landing_pages_updated_at
  BEFORE UPDATE ON landing_pages
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE INDEX idx_landing_pages_hotel ON landing_pages(hotel_id);
