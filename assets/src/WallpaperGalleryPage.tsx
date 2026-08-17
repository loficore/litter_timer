/**
 * 壁纸图库管理页面
 * 独立的图库管理：列表 / 缩略图 / 上传 / 删除确认（带引用数提示）
 * 图片已由后端压缩（≤2560px），直接使用原图预览，不做缩略图管线
 */

import { useState, useEffect } from "preact/hooks";
import type { FunctionalComponent } from "preact";
import { Header } from "./components/Header";
import { t } from "./utils/i18n";
import { getAPIClient } from "./utils/apiClientSingleton";
import type { WallpaperListResult } from "./types/api";

export const WallpaperGalleryPage: FunctionalComponent = () => {
  const [wallpapers, setWallpapers] = useState<WallpaperListResult[]>([]);
  const [loading, setLoading] = useState(true);
  const [uploading, setUploading] = useState(false);
  const [uploadError, setUploadError] = useState<string | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<WallpaperListResult | null>(null);
  const [deleteError, setDeleteError] = useState<string | null>(null);

  const api = getAPIClient();

  const loadWallpapers = async () => {
    try {
      const list = await api.listWallpapers();
      setWallpapers(Array.isArray(list) ? list : []);
    } catch {
      setWallpapers([]);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void loadWallpapers();
  }, []);

  const handleUpload = async (e: Event) => {
    const input = e.target as HTMLInputElement;
    const file = input.files?.[0];
    if (!file) return;

    setUploading(true);
    setUploadError(null);
    try {
      await api.uploadWallpaper(file);
      await loadWallpapers();
    } catch {
      setUploadError(t("modal.upload_fail"));
    } finally {
      setUploading(false);
      input.value = "";
    }
  };

  const handleConfirmDelete = async () => {
    if (!deleteTarget) return;
    setDeleteError(null);
    try {
      await api.deleteWallpaper(deleteTarget.name);
      setDeleteTarget(null);
      await loadWallpapers();
    } catch {
      setDeleteTarget(null);
      setDeleteError(t("errors.operation_failed"));
    }
  };

  const formatSize = (bytes: number): string => {
    return `${(bytes / 1024).toFixed(0)} KB`;
  };

  return (
    <div className="flex flex-col flex-1 bg-transparent overflow-hidden">
      <Header title={t("gallery.title")} showSettings={false} showStats={false} />

      <div className="flex-1 overflow-y-auto p-4">
        <div className="flex justify-between items-center mb-4">
          <h2 className="text-xl font-bold">{t("gallery.title")}</h2>
          <label className="btn btn-primary btn-sm cursor-pointer" data-testid="gallery-upload-label">
            {uploading ? (
              <span className="loading loading-spinner loading-xs" />
            ) : (
              <>
                <span>+ {t("gallery.upload")}</span>
                <input
                  type="file"
                  accept="image/*"
                  className="hidden"
                  data-testid="gallery-upload-input"
                  onChange={(e) => { void handleUpload(e); }}
                  disabled={uploading}
                />
              </>
            )}
          </label>
        </div>

        {uploadError && (
          <div className="alert alert-error mb-4 py-2 text-sm" role="alert">
            {uploadError}
          </div>
        )}

        {deleteError && (
          <div className="alert alert-error mb-4 py-2 text-sm" role="alert">
            {deleteError}
          </div>
        )}

        {loading && (
          <div className="flex items-center justify-center py-16" data-testid="gallery-loading">
            <span className="loading loading-spinner loading-lg" />
          </div>
        )}

        {!loading && wallpapers.length === 0 && (
          <div className="text-center py-16 text-[var(--my-on-surface-variant)] text-sm" data-testid="gallery-empty">
            {t("gallery.empty")}
          </div>
        )}

        {!loading && wallpapers.length > 0 && (
          <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 gap-3">
            {wallpapers.map((item) => (
              <div
                key={item.name}
                className="rounded-lg overflow-hidden border border-[var(--my-outline)] bg-[var(--my-surface-strong)]"
                data-testid={`gallery-item-${item.name}`}
              >
                <img
                  src={`/api/wallpapers/${item.name}`}
                  alt={item.name}
                  className="w-full h-24 object-cover"
                />
                <div className="p-2 space-y-1">
                  <div className="text-xs truncate" title={item.name}>
                    {item.name}
                  </div>
                  <div className="flex items-center justify-between gap-1">
                    <span className="text-xs text-[var(--my-on-surface-variant)]">
                      {t("gallery.size")}: {formatSize(item.size)}
                    </span>
                    <button
                      type="button"
                      className="btn btn-xs btn-ghost text-red-400"
                      data-testid={`gallery-delete-${item.name}`}
                      onClick={() => setDeleteTarget(item)}
                      title={t("gallery.delete_confirm")}
                    >
                      {t("button.delete")}
                    </button>
                  </div>
                  {item.refs > 0 && (
                    <div className="badge badge-warning badge-sm" data-testid={`gallery-refs-${item.name}`}>
                      {t("gallery.referenced_by", { count: item.refs })}
                    </div>
                  )}
                </div>
              </div>
            ))}
          </div>
        )}

        {deleteTarget && (
          <div className="fixed inset-0 z-60 flex items-center justify-center">
            <div
              className="absolute inset-0 bg-black/50"
              onClick={() => setDeleteTarget(null)}
            />
            <div className="relative my-surface-modal rounded-xl p-4 max-w-sm mx-4">
              <h4 className="font-bold mb-2">{t("gallery.delete_confirm")}</h4>
              <p className="text-sm text-[var(--my-on-surface-variant)] mb-2">
                {t("gallery.delete_confirm_desc")}
              </p>
              {deleteTarget.refs > 0 && (
                <div className="space-y-1 mb-2">
                  <div className="badge badge-warning badge-sm">
                    {t("gallery.referenced_by", { count: deleteTarget.refs })}
                  </div>
                  <p className="text-sm text-[var(--my-on-surface-variant)]">
                    {t("gallery.unbound_note")}
                  </p>
                </div>
              )}
              <div className="flex justify-end gap-2">
                <button
                  type="button"
                  className="btn btn-sm btn-ghost"
                  data-testid="gallery-cancel-delete"
                  onClick={() => setDeleteTarget(null)}
                >
                  {t("modal.cancel")}
                </button>
                <button
                  type="button"
                  className="btn btn-sm btn-error"
                  data-testid="gallery-confirm-delete"
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
};

export default WallpaperGalleryPage;
