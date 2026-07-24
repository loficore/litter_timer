/**
 * 壁纸选择弹窗组件
 * 支持纯色与图片两种壁纸，提供 URL 输入、上传、删除本地图片等能力
 */

import { useState, useEffect } from "preact/hooks";
import type { FunctionalComponent } from "preact";
import { t } from "../utils/i18n";
import { getAPIClient } from "../utils/apiClientSingleton";
import { WALLPAPER_LOCAL_PREFIX, resolveWallpaperUrl } from "../utils/constants";

interface WallpaperModalProps {
  /** 是否显示弹窗 */
  isOpen: boolean;
  /** 当前壁纸值（颜色字符串或图片 URL） */
  value: string;
  /** 关闭弹窗回调 */
  onClose: () => void;
  /** 壁纸变更回调 */
  onChange: (wallpaper: string) => void;
}

/** 壁纸类型：纯色或图片 */
type WallpaperType = "solid" | "image";

/** 本地已上传图片 */
interface LocalImage {
  /** 图片文件名 */
  name: string;
}

export const WallpaperModal: FunctionalComponent<WallpaperModalProps> = ({
  isOpen,
  value,
  onClose,
  onChange,
}) => {
  const [wallpaperType, setWallpaperType] = useState<WallpaperType>("solid");
  const [colorValue, setColorValue] = useState("#121212");
  const [imageUrl, setImageUrl] = useState("");
  const [localImages, setLocalImages] = useState<LocalImage[]>([]);
  const [uploading, setUploading] = useState(false);
  const [uploadError, setUploadError] = useState<string | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<string | null>(null);
  const [previewLoading, setPreviewLoading] = useState(false);

  const api = getAPIClient();

  const isLocal = value.startsWith(WALLPAPER_LOCAL_PREFIX);
  const isColor = value.startsWith("#");
  const isImage = !isColor && value.length > 0;

  useEffect(() => {
    if (isColor) {
      setWallpaperType("solid");
      setColorValue(value);
    } else if (isImage) {
      setWallpaperType("image");
      if (isLocal) {
        setImageUrl(value.slice(WALLPAPER_LOCAL_PREFIX.length));
      } else {
        setImageUrl(value);
      }
    } else {
      setWallpaperType("solid");
    }
  }, [value, isColor, isImage, isLocal]);

  useEffect(() => {
    if (isOpen && wallpaperType === "image") {
      api.listWallpapers().then(setLocalImages).catch(() => setLocalImages([]));
    }
  }, [isOpen, wallpaperType]);

  if (!isOpen) {
    return null;
  }

  const handleColorChange = (color: string) => {
    setColorValue(color);
    onChange(color);
  };

  const handleImageUrlChange = (url: string) => {
    setImageUrl(url);
  };

  const handleSubmitUrl = async () => {
    const url = imageUrl.trim();
    if (!url) return;
    setPreviewLoading(true);
    setUploadError(null);
    try {
      const result = await api.fetchWallpaperByUrl(url);
      onChange(`${WALLPAPER_LOCAL_PREFIX}${result.filename}`);
      setLocalImages((prev) => [...prev, { name: result.filename }]);
    } catch {
      setUploadError(t("modal.upload_fail"));
    } finally {
      setPreviewLoading(false);
    }
  };

  const handleImageLoad = () => {
    setPreviewLoading(false);
  };

  const handleImageError = () => {
    setPreviewLoading(false);
  };

  const handleUpload = async (e: Event) => {
    const input = e.target as HTMLInputElement;
    const file = input.files?.[0];
    if (!file) return;

    setUploading(true);
    setUploadError(null);
    try {
      const result = await api.uploadWallpaper(file);
      const localValue = `${WALLPAPER_LOCAL_PREFIX}${result.filename}`;
      onChange(localValue);
      setImageUrl(result.filename);
      setLocalImages((prev) => [...prev, { name: result.filename }]);
    } catch {
      setUploadError(t("modal.upload_fail"));
    } finally {
      setUploading(false);
      input.value = "";
    }
  };

  const handleSelectLocal = (filename: string) => {
    setImageUrl(filename);
    onChange(`${WALLPAPER_LOCAL_PREFIX}${filename}`);
  };

  const handleDeleteClick = (filename: string) => {
    setDeleteTarget(filename);
  };

  const handleConfirmDelete = async () => {
    if (!deleteTarget) return;
    try {
      await api.deleteWallpaper(deleteTarget);
      setLocalImages((prev) => prev.filter((img) => img.name !== deleteTarget));
      if (isLocal && value.slice(WALLPAPER_LOCAL_PREFIX.length) === deleteTarget) {
        onChange("");
        setImageUrl("");
      }
    } catch {
      // ignore
    } finally {
      setDeleteTarget(null);
    }
  };

  const getPreviewUrl = (): string => {
    if (wallpaperType === "solid") {
      return "";
    }
    return resolveWallpaperUrl(value);
  };

  const showPreview = wallpaperType === "image" && (isLocal || isImage) && !previewLoading;

  const getBackdropImageUrl = (): string => {
    if (isColor) return "";
    if (isImage) return resolveWallpaperUrl(value);
    if (value.startsWith("linear")) return "";
    return "";
  };

  const backdropImageUrl = getBackdropImageUrl();
  const hasBackdropImage = backdropImageUrl.length > 0;
  const hasWallpaper = value.length > 0;

  const backdropStyle: Record<string, string> = {
    position: "fixed",
    top: "0",
    left: "0",
    right: "0",
    bottom: "0",
    zIndex: "40",
    overflow: "hidden",
  };

  const imgStyle: Record<string, string> = {
    width: "100%",
    height: "100%",
    objectFit: "cover",
    filter: "blur(34px) saturate(145%)",
    transform: "scale(1.1)",
    WebkitFilter: "blur(34px) saturate(145%)",
  };

  const overlayStyle: Record<string, string> = {
    position: "fixed",
    top: "0",
    left: "0",
    right: "0",
    bottom: "0",
    zIndex: "41",
    backgroundColor: "rgba(0,0,0,0.5)",
  };

  const backdropNode = hasBackdropImage ? (
    <div style={backdropStyle}>
      <img src={backdropImageUrl} alt="" style={imgStyle} />
    </div>
  ) : hasWallpaper ? (
    <div style={{ ...backdropStyle, ...(isColor ? { backgroundColor: value } : { background: value }) }} />
  ) : (
    <div style={backdropStyle} />
  );

  const containerStyle: Record<string, string> = {
    position: "fixed",
    top: "0",
    left: "0",
    right: "0",
    bottom: "0",
    zIndex: "100",
    display: "flex",
    alignItems: "center",
    justifyContent: "center",
    backgroundColor: "transparent",
  };

  const panelStyle: Record<string, string | number> = {
    border: "1px solid rgba(255,255,255,0.1)",
    borderRadius: "1rem",
    background: "linear-gradient(160deg, rgba(30,30,40,0.96) 0%, rgba(40,40,55,0.99) 100%)",
    boxShadow: "0 16px 32px rgba(0,0,0,0.18)",
    backdropFilter: "blur(36px) saturate(142%)",
    width: "100%",
    maxWidth: "32rem",
    marginLeft: "1rem",
    marginRight: "1rem",
    maxHeight: "80vh",
    overflow: "hidden",
    display: "flex",
    flexDirection: "column" as const,
  };

  const modalContent = (
    <div style={containerStyle}>
      <div style={panelStyle}>
        <div style={{ padding: "1rem", borderBottom: "1px solid rgba(255,255,255,0.1)", display: "flex", justifyContent: "space-between", alignItems: "center" }}>
          <h3 style={{ fontSize: "1.125rem", fontWeight: "bold" }}>{t("modal.select_wallpaper")}</h3>
          <button
            type="button"
            style={{ padding: "0.25rem", borderRadius: "9999px", background: "transparent", border: "none", cursor: "pointer" }}
            onClick={onClose}
          >
            <svg xmlns="http://www.w3.org/2000/svg" className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>

        <div className="tabs tabs-boxed m-3">
          <button
            type="button"
            className={`tab ${wallpaperType === "solid" ? "tab-active" : ""}`}
            onClick={() => setWallpaperType("solid")}
          >
            {t("modal.solid")}
          </button>
          <button
            type="button"
            className={`tab ${wallpaperType === "image" ? "tab-active" : ""}`}
            onClick={() => setWallpaperType("image")}
          >
            {t("modal.image")}
          </button>
        </div>

        <div className="flex-1 overflow-y-auto p-4">
          {wallpaperType === "solid" && (
            <div className="space-y-4">
              <div className="flex items-center gap-4">
                <div
                  className="w-16 h-16 rounded-lg border-2 border-[var(--my-outline)]"
                  style={{ backgroundColor: colorValue }}
                />
                <div>
                  <label className="text-sm text-[var(--my-on-surface-variant)] mb-1 block">
                    {t("modal.color_picker")}
                  </label>
                  <input
                    type="color"
                    className="cursor-pointer"
                    value={colorValue}
                    onChange={(e) => handleColorChange((e.target as HTMLInputElement).value)}
                  />
                </div>
              </div>
            </div>
          )}

          {wallpaperType === "image" && (
            <div className="space-y-4">
              {showPreview && (
                <div className="space-y-2">
                  <div className="text-sm text-[var(--my-on-surface-variant)]">
                    {t("modal.current_wallpaper")}
                  </div>
                  <div className="relative rounded-lg overflow-hidden bg-[var(--my-surface-strong)] border border-[var(--my-outline)]">
                    <img
                      src={getPreviewUrl()}
                      alt={t("modal.preview")}
                      className="w-full h-40 object-cover"
                      onLoad={handleImageLoad}
                      onError={handleImageError}
                    />
                    <div className="absolute bottom-0 left-0 right-0 bg-black/50 text-white text-xs p-1 truncate">
                      {isLocal ? imageUrl : imageUrl.split("/").pop() || imageUrl}
                    </div>
                  </div>
                </div>
              )}

              {previewLoading && (
                <div className="h-40 rounded-lg bg-[var(--my-surface-strong)] flex items-center justify-center">
                  <span className="loading loading-spinner loading-sm" />
                </div>
              )}

              <div>
                <label className="text-sm text-[var(--my-on-surface-variant)] mb-1 block">
                  {t("modal.image_url")}
                </label>
                <div className="flex items-center gap-2">
                  <input
                    type="text"
                    className="my-input flex-1 text-sm"
                    placeholder={t("modal.enter_url")}
                    value={isLocal ? "" : imageUrl}
                    onInput={(e) => handleImageUrlChange((e.target as HTMLInputElement).value)}
                    onKeyDown={(e) => {
                      if (e.key === "Enter") {
                        void handleSubmitUrl();
                      }
                    }}
                  />
                  <button
                    type="button"
                    className="btn btn-sm btn-outline"
                    onClick={() => { void handleSubmitUrl(); }}
                  >
                    {t("modal.add")}
                  </button>
                </div>
              </div>

              <div className="flex items-center gap-2">
                <label className="btn btn-sm btn-outline cursor-pointer">
                  {uploading ? t("modal.upload_progress") : t("modal.upload_image")}
                  <input
                    type="file"
                    accept="image/*"
                    className="hidden"
                    onChange={(e) => { void handleUpload(e); }}
                    disabled={uploading}
                  />
                </label>
                {uploadError && <span className="text-xs text-red-400">{uploadError}</span>}
              </div>

              {localImages.length > 0 && (
                <div>
                  <div className="text-sm text-[var(--my-on-surface-variant)] mb-2">
                    {t("modal.local_images")}
                  </div>
                  <div className="grid grid-cols-4 gap-2 max-h-48 overflow-y-auto">
                    {localImages.map((img) => (
                      <div
                        key={img.name}
                        className={`relative group rounded-lg overflow-hidden border-2 cursor-pointer ${
                          isLocal && value.slice(WALLPAPER_LOCAL_PREFIX.length) === img.name
                            ? "border-primary ring-2 ring-primary/30"
                            : "border-[var(--my-outline)] hover:border-primary/50"
                        }`}
                        onClick={() => handleSelectLocal(img.name)}
                      >
                        <img
                          src={`/api/wallpapers/${img.name}`}
                          alt={img.name}
                          className="w-full h-16 object-cover"
                        />
                        <button
                          type="button"
                          className="absolute top-0.5 right-0.5 w-5 h-5 flex items-center justify-center rounded-full bg-red-600/80 text-white text-xs opacity-0 group-hover:opacity-100 transition-opacity"
                          onClick={(e) => {
                            e.stopPropagation();
                            void handleDeleteClick(img.name);
                          }}
                          title={t("modal.delete_image")}
                        >
                          ×
                        </button>
                      </div>
                    ))}
                  </div>
                </div>
              )}

              {localImages.length === 0 && !showPreview && !previewLoading && (
                <div className="text-center py-6 text-[var(--my-on-surface-variant)] text-sm">
                  {t("modal.no_images")}
                </div>
              )}
            </div>
          )}
        </div>

        {deleteTarget && (
          <div className="fixed inset-0 z-60 flex items-center justify-center">
            <div className="absolute inset-0 bg-black/50" onClick={() => setDeleteTarget(null)} />
            <div className="relative my-surface-modal rounded-xl p-4 max-w-sm mx-4">
              <h4 className="font-bold mb-2">{t("modal.delete_confirm")}</h4>
              <p className="text-sm text-[var(--my-on-surface-variant)] mb-4">
                {t("modal.delete_confirm_desc")}
              </p>
              <div className="flex justify-end gap-2">
                <button
                  type="button"
                  className="btn btn-sm btn-ghost"
                  onClick={() => setDeleteTarget(null)}
                >
                  {t("modal.cancel")}
                </button>
                <button
                  type="button"
                  className="btn btn-sm btn-error"
                  onClick={() => void handleConfirmDelete()}
                >
                  {t("modal.confirm")}
                </button>
              </div>
            </div>
          </div>
        )}
      </div>
    </div>
  );

  return (
    <>
      {backdropNode}
      <div style={overlayStyle} />
      {modalContent}
    </>
  );
};