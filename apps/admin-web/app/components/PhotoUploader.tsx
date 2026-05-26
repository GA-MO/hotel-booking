"use client";

import { useRef, useState } from "react";

import { t } from "@/app/i18n";
import { ApiClientError, Photos, Uploads } from "@/app/lib/api";
import type { Photo, UploadKind } from "@/app/lib/types";

import { Button, ErrorBanner } from "./ui";

const ACCEPT = "image/jpeg,image/png,image/webp";
const MAX_BYTES = 10 * 1024 * 1024;

type PendingUpload = { name: string; progress: "uploading" | "saving" };

type Props = {
  hotelID: string;
  // `room_type_photo` requires roomTypeID; `hotel_photo` ignores it.
  kind: UploadKind;
  roomTypeID?: string;
  photos: Photo[];
  onChange: (photos: Photo[]) => void;
};

export default function PhotoUploader({
  hotelID,
  kind,
  roomTypeID,
  photos,
  onChange,
}: Props) {
  const fileRef = useRef<HTMLInputElement>(null);
  const [pending, setPending] = useState<PendingUpload[]>([]);
  const [err, setErr] = useState<string | null>(null);

  async function uploadOne(file: File): Promise<Photo | null> {
    if (file.size > MAX_BYTES) {
      throw new Error(`${file.name}: ${t("upload_too_large")}`);
    }
    const presigned = await Uploads.upload(file, kind, hotelID);
    const req = {
      storage_key: presigned.object_key,
      caption: file.name,
      display_order: photos.length,
      is_cover: photos.length === 0,
    };
    if (kind === "room_type_photo") {
      if (!roomTypeID) return null;
      return await Photos.createRoomType(hotelID, roomTypeID, req);
    }
    return await Photos.createHotel(hotelID, req);
  }

  async function onFiles(files: FileList | null) {
    if (!files || files.length === 0) return;
    setErr(null);
    const list = Array.from(files);
    setPending(list.map((f) => ({ name: f.name, progress: "uploading" })));
    const next: Photo[] = [...photos];
    try {
      for (let i = 0; i < list.length; i++) {
        setPending((p) =>
          p.map((x, idx) => (idx === i ? { ...x, progress: "uploading" } : x)),
        );
        const created = await uploadOne(list[i]);
        if (created) {
          setPending((p) =>
            p.map((x, idx) => (idx === i ? { ...x, progress: "saving" } : x)),
          );
          next.push(created);
          onChange([...next]);
        }
      }
    } catch (e) {
      if (e instanceof ApiClientError) {
        setErr(`${e.code}: ${e.message}`);
      } else if (e instanceof Error) {
        setErr(e.message);
      } else {
        setErr(t("error_generic"));
      }
    } finally {
      setPending([]);
      if (fileRef.current) fileRef.current.value = "";
    }
  }

  async function remove(p: Photo) {
    if (!window.confirm(t("delete") + "?")) return;
    try {
      if (kind === "room_type_photo" && roomTypeID) {
        await Photos.removeRoomType(hotelID, roomTypeID, p.id);
      } else if (kind === "hotel_photo") {
        await Photos.removeHotel(hotelID, p.id);
      }
      onChange(photos.filter((x) => x.id !== p.id));
    } catch (e) {
      setErr(e instanceof Error ? e.message : t("error_generic"));
    }
  }

  return (
    <div className="space-y-2">
      <div className="flex flex-wrap items-center gap-2">
        <Button
          type="button"
          variant="secondary"
          onClick={() => fileRef.current?.click()}
          disabled={pending.length > 0}
        >
          {pending.length > 0 ? t("uploading") : t("upload_photo")}
        </Button>
        <input
          ref={fileRef}
          type="file"
          accept={ACCEPT}
          multiple
          className="hidden"
          onChange={(e) => onFiles(e.target.files)}
        />
        <span className="text-xs text-neutral-500">{t("upload_hint")}</span>
      </div>

      <ErrorBanner message={err} />

      {pending.length > 0 && (
        <ul className="space-y-1 text-xs text-neutral-600">
          {pending.map((p, i) => (
            <li key={i}>
              {p.name} — {p.progress === "uploading" ? t("uploading") : t("saving")}
            </li>
          ))}
        </ul>
      )}

      {photos.length > 0 && (
        <ul className="grid grid-cols-2 gap-2 sm:grid-cols-3">
          {photos.map((p) => (
            <li key={p.id} className="group relative overflow-hidden rounded border border-neutral-200">
              {/* eslint-disable-next-line @next/next/no-img-element */}
              <img
                src={p.storage_key}
                alt={p.alt_text || p.caption || ""}
                className="aspect-square w-full object-cover"
                loading="lazy"
              />
              <div className="absolute inset-x-0 bottom-0 flex items-center justify-between bg-black/55 px-2 py-1 text-xs text-white opacity-0 transition group-hover:opacity-100">
                <span className="truncate" title={p.caption}>
                  {p.is_cover ? "★ " : ""}
                  {p.caption || ""}
                </span>
                <button
                  type="button"
                  onClick={() => remove(p)}
                  className="text-xs underline-offset-2 hover:underline"
                >
                  {t("delete")}
                </button>
              </div>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
