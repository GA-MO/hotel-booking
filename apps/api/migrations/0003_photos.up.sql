-- Photos for hotel landing pages and room types.
-- storage_key is an opaque object key inside the S3/MinIO bucket; width/height
-- are populated asynchronously by imgproxy / the upload pipeline.
-- A partial unique index enforces at most one cover photo per parent row.

-- ========================================================================
-- hotel_photos: landing page hero + gallery
-- ========================================================================
CREATE TABLE hotel_photos (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  hotel_id        UUID NOT NULL REFERENCES hotels(id) ON DELETE CASCADE,
  storage_key     TEXT NOT NULL,
  caption         TEXT,
  alt_text        TEXT,
  width           INT,
  height          INT,
  display_order   INT NOT NULL DEFAULT 0,
  is_cover        BOOLEAN NOT NULL DEFAULT FALSE,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_hotel_photos_hotel_id ON hotel_photos(hotel_id);
CREATE UNIQUE INDEX idx_hotel_photos_one_cover
  ON hotel_photos(hotel_id) WHERE is_cover = TRUE;

-- ========================================================================
-- room_type_photos: per-room-type gallery
-- ========================================================================
CREATE TABLE room_type_photos (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  room_type_id    UUID NOT NULL REFERENCES room_types(id) ON DELETE CASCADE,
  storage_key     TEXT NOT NULL,
  caption         TEXT,
  alt_text        TEXT,
  width           INT,
  height          INT,
  display_order   INT NOT NULL DEFAULT 0,
  is_cover        BOOLEAN NOT NULL DEFAULT FALSE,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_room_type_photos_room_type_id ON room_type_photos(room_type_id);
CREATE UNIQUE INDEX idx_room_type_photos_one_cover
  ON room_type_photos(room_type_id) WHERE is_cover = TRUE;
