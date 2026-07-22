/**
 * 主密码弹窗组件
 * 用于首次设置主密码或解锁已加密的凭据存储
 */

import { useState } from "preact/hooks";
import type { FunctionalComponent } from "preact";
import { t } from "../utils/i18n";
import { getAPIClient } from "../utils/apiClientSingleton";

interface MasterPasswordModalProps {
  /** 是否显示弹窗 */
  isOpen: boolean;
  /** 模式："setup" 设置密码，"unlock" 解锁凭据 */
  mode: "setup" | "unlock";
  /** 操作成功回调 */
  onSuccess: () => void;
  /** 关闭弹窗回调 */
  onClose: () => void;
}

export const MasterPasswordModal: FunctionalComponent<MasterPasswordModalProps> = ({
  isOpen,
  mode,
  onSuccess,
  onClose,
}) => {
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const api = getAPIClient();

  const handleSubmit = async () => {
    if (loading) return;
    setError(null);

    if (password.length < 4) {
      setError(t("master_password.min_length"));
      return;
    }

    if (mode === "setup" && password !== confirmPassword) {
      setError(t("master_password.not_match"));
      return;
    }

    setLoading(true);
    try {
      let result;
      if (mode === "setup") {
        result = await api.setMasterPassword(password);
      } else {
        result = await api.unlockCredentials(password);
      }

      if (result.success) {
        onSuccess();
        setPassword("");
        setConfirmPassword("");
      } else {
        setError(result.error || t("master_password.invalid_password"));
      }
    } catch (e) {
      setError(String(e));
    } finally {
      setLoading(false);
    }
  };

  if (!isOpen) return null;

  const title = mode === "setup" ? t("master_password.setup_title") : t("master_password.unlock_title");
  const description = mode === "setup" ? t("master_password.setup_description") : t("master_password.unlock_description");

  return (
    <div
      style={{
        position: "fixed",
        top: 0,
        left: 0,
        right: 0,
        bottom: 0,
        backgroundColor: "rgba(0,0,0,0.5)",
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        zIndex: 1000,
      }}
      onClick={onClose}
    >
      <div
        style={{
          backgroundColor: "var(--bg-primary)",
          borderRadius: "12px",
          padding: "24px",
          width: "90%",
          maxWidth: "400px",
          boxShadow: "0 4px 24px rgba(0,0,0,0.3)",
        }}
        onClick={(e) => e.stopPropagation()}
      >
        <h2 style={{ margin: "0 0 8px 0", fontSize: "20px", fontWeight: 600 }}>{title}</h2>
        <p style={{ margin: "0 0 20px 0", color: "var(--text-secondary)", fontSize: "14px" }}>{description}</p>

        <div style={{ marginBottom: "16px" }}>
          <input
            type="password"
            value={password}
            onInput={(e) => setPassword((e.target as HTMLInputElement).value)}
            placeholder={t("master_password.password_placeholder")}
            style={{
              width: "100%",
              padding: "12px",
              borderRadius: "8px",
              border: "1px solid var(--border-color)",
              backgroundColor: "var(--bg-secondary)",
              color: "var(--text-primary)",
              fontSize: "14px",
              boxSizing: "border-box",
            }}
          />
        </div>

        {mode === "setup" && (
          <div style={{ marginBottom: "20px" }}>
            <input
              type="password"
              value={confirmPassword}
              onInput={(e) => setConfirmPassword((e.target as HTMLInputElement).value)}
              placeholder={t("master_password.confirm_placeholder")}
              style={{
                width: "100%",
                padding: "12px",
                borderRadius: "8px",
                border: "1px solid var(--border-color)",
                backgroundColor: "var(--bg-secondary)",
                color: "var(--text-primary)",
                fontSize: "14px",
                boxSizing: "border-box",
              }}
            />
          </div>
        )}

        {error && (
          <div
            style={{
              color: "#ef4444",
              fontSize: "14px",
              marginBottom: "16px",
              padding: "8px 12px",
              backgroundColor: "rgba(239,68,68,0.1)",
              borderRadius: "6px",
            }}
          >
            {error}
          </div>
        )}

        <div style={{ display: "flex", gap: "12px", justifyContent: "flex-end" }}>
          <button
            onClick={onClose}
            style={{
              padding: "10px 16px",
              borderRadius: "8px",
              border: "1px solid var(--border-color)",
              backgroundColor: "transparent",
              color: "var(--text-primary)",
              cursor: "pointer",
              fontSize: "14px",
            }}
          >
            {t("master_password.cancel")}
          </button>
          <button
            onClick={() => { void handleSubmit(); }}
            disabled={loading}
            style={{
              padding: "10px 16px",
              borderRadius: "8px",
              border: "none",
              backgroundColor: "var(--accent-color, #3b82f6)",
              color: "white",
              cursor: loading ? "not-allowed" : "pointer",
              opacity: loading ? 0.7 : 1,
              fontSize: "14px",
            }}
          >
            {loading ? "..." : t("master_password.submit")}
          </button>
        </div>
      </div>
    </div>
  );
};
