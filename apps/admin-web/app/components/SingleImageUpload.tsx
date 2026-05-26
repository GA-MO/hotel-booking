"use client";

import { useRef, useState } from "react";

import { t } from "@/app/i18n";
import { ApiClientError, Uploads } from "@/app/lib/api";
import type { UploadKind } from "@/app/lib/types";

import { Button, ErrorBanner } from "./ui";

const ACCEPT = "image/jpeg,image/png,image/webp";
const MAX_BYTES = 10 * 1024 * 1024;

type Props = {
  hotelID: string;
  kind: UploadKind;
  value?: string;
  onChange: (publicURL: string) => void;
  onRemove: () => void;
};

// Unlike PhotoUploader (multi-photo gallery persisted through /photos), this
// component just resolves a single object URL into a string field on the
// owning form — the form decides when/how to persist it.
export default function SingleImageUpload({
  hotelID,
  kind,
  value,
  onChange,
  onRemove,
}: Props) {
  const fileRef = useRef<HTMLInputElement>(null);
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  async function onFile(file: File | null) {
    if (!file) return;
    setErr(null);
    if (file.size > MAX_BYTES) {
      setErr(`${file.name}: ${t("upload_too_large")}`);
      return;
    }
    setBusy(true);
    try {
      const presigned = await Uploads.upload(file, kind, hotelID);
      onChange(presigned.public_url);
    } catch (e) {
      if (e instanceof ApiClientError) setErr(`${e.code}: ${e.message}`);
      else if (e instanceof Error) setErr(e.message);
      else setErr(t("error_generic"));
    } finally {
      setBusy(false);
      if (fileRef.current) fileRef.current.value = "";
    }
  }

  return (
    <div className="space-y-2">
      {value ? (
        <div className="flex items-start gap-3">
          {/* eslint-disable-next-line @next/next/no-img-element */}
          <img
            src={value}
            alt=""
            className="h-20 w-20 rounded border border-neutral-200 object-cover"
            loading="lazy"
          />
          <div className="flex flex-col gap-2">
            <Button
              type="button"
              variant="secondary"
              onClick={() => fileRef.current?.click()}
              disabled={busy}
            >
              {busy ? t("uploading") : t("replace_image")}
            </Button>
            <Button
              type="button"
              variant="ghost"
              onClick={onRemove}
              disabled={busy}
            >
              {t("remove")}
            </Button>
          </div>
        </div>
      ) : (
        <Button
          type="button"
          variant="secondary"
          onClick={() => fileRef.current?.click()}
          disabled={busy}
        >
          {busy ? t("uploading") : t("upload_photo")}
        </Button>
      )}

      <input
        ref={fileRef}
        type="file"
        accept={ACCEPT}
        className="hidden"
        onChange={(e) => onFile(e.target.files?.[0] || null)}
      />

      <p className="text-xs text-neutral-500">{t("upload_hint")}</p>

      <ErrorBanner message={err} />
    </div>
  );
}
