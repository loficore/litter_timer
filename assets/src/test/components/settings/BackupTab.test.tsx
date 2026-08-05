import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/preact";
import { BackupTab } from "../../../components/settings/BackupTab";
import { t } from "../../../utils/i18n";

vi.mock("../../../utils/apiClientSingleton", () => ({
  getAPIClient: vi.fn(),
}));

vi.mock("../../../components/MasterPasswordModal", () => ({
  MasterPasswordModal: ({ isOpen, mode, onSuccess, onClose }: any) => {
    if (!isOpen) return null;
    return (
      <div data-testid="master-password-modal">
        <div data-testid="modal-mode">{mode}</div>
        <button data-testid="modal-close" onClick={onClose}>关闭</button>
        <button data-testid="modal-success" onClick={onSuccess}>成功</button>
      </div>
    );
  },
}));

import { getAPIClient } from "../../../utils/apiClientSingleton";

const mockApiClient = {
  listBackups: vi.fn(),
  createBackup: vi.fn(),
  restoreBackup: vi.fn(),
  deleteBackup: vi.fn(),
  verifyBackup: vi.fn(),
  getMasterPasswordStatus: vi.fn(),
  getBackupConfig: vi.fn(),
  updateBackupConfig: vi.fn(),
};

describe("BackupTab", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.useRealTimers();
    (getAPIClient as ReturnType<typeof vi.fn>).mockReturnValue(mockApiClient);
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("renders backup tab heading", async () => {
    mockApiClient.listBackups.mockResolvedValue({ success: true, backups: [] });

    render(<BackupTab config={{}} onChange={vi.fn()} />);

    await waitFor(() => {
      expect(screen.getByText("备份")).toBeTruthy();
    });
  });

  it("shows empty backups message when no backups exist", async () => {
    mockApiClient.listBackups.mockResolvedValue({ success: true, backups: [] });

    render(<BackupTab config={{}} onChange={vi.fn()} />);

    await waitFor(() => {
      expect(screen.getByText(t("backup.no_backups"))).toBeTruthy();
    });
  });

  it("displays backups list when backups exist", async () => {
    const mockBackups = [
      { name: "backup_2024_01.db", timestamp: 1704067200, size_bytes: 1024 * 1024 },
      { name: "backup_2024_02.db", timestamp: 1704153600, size_bytes: 2 * 1024 * 1024 },
    ];
    mockApiClient.listBackups.mockResolvedValue({ success: true, backups: mockBackups });

    render(<BackupTab config={{}} onChange={vi.fn()} />);

    await waitFor(() => {
      expect(screen.getByText("backup_2024_01.db")).toBeTruthy();
      expect(screen.getByText("backup_2024_02.db")).toBeTruthy();
    });
  });

  it("shows error message when loadBackups fails", async () => {
    mockApiClient.listBackups.mockResolvedValue({ success: false, error: "network error: GET https://example.com -> timeout" });

    render(<BackupTab config={{}} onChange={vi.fn()} />);

    await waitFor(() => {
      expect(screen.getByText("网络连接失败，请检查网络后重试")).toBeTruthy();
    });
  });

  it("calls onChange when target type changes to webdav", async () => {
    mockApiClient.listBackups.mockResolvedValue({ success: true, backups: [] });
    mockApiClient.getMasterPasswordStatus.mockResolvedValue({ has_password: false, unlocked: false });
    const onChange = vi.fn();

    render(<BackupTab config={{}} onChange={onChange} />);

    await waitFor(() => {
      expect(screen.getByText("备份")).toBeTruthy();
    });

    const select = screen.getByRole("combobox") as HTMLSelectElement;
    fireEvent.change(select, { target: { value: "webdav" } });

    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({ backup_target_type: "webdav" })
    );
  });

  it("shows webdav fields when target_type is webdav", async () => {
    mockApiClient.listBackups.mockResolvedValue({ success: true, backups: [] });
    mockApiClient.getMasterPasswordStatus.mockResolvedValue({ has_password: false, unlocked: false });

    render(<BackupTab config={{}} onChange={vi.fn()} />);

    await waitFor(() => {
      expect(screen.getByText("备份")).toBeTruthy();
    });

    const select = screen.getByRole("combobox") as HTMLSelectElement;
    fireEvent.change(select, { target: { value: "webdav" } });

    await waitFor(() => {
      expect(screen.getByText(t("backup.webdav_url"))).toBeTruthy();
      expect(screen.getByText(t("backup.username"))).toBeTruthy();
      expect(screen.getByText(t("backup.password"))).toBeTruthy();
    });
  });

  it("shows s3 fields when target_type is s3", async () => {
    mockApiClient.listBackups.mockResolvedValue({ success: true, backups: [] });
    mockApiClient.getMasterPasswordStatus.mockResolvedValue({ has_password: false, unlocked: false });

    render(<BackupTab config={{}} onChange={vi.fn()} />);

    await waitFor(() => {
      expect(screen.getByText("备份")).toBeTruthy();
    });

    const select = screen.getByRole("combobox") as HTMLSelectElement;
    fireEvent.change(select, { target: { value: "s3" } });

    await waitFor(() => {
      expect(screen.getByText(t("backup.s3_endpoint"))).toBeTruthy();
      expect(screen.getByText(t("backup.s3_bucket"))).toBeTruthy();
      expect(screen.getByText(t("backup.s3_region"))).toBeTruthy();
      expect(screen.getByText(t("backup.s3_access_key"))).toBeTruthy();
      expect(screen.getByText(t("backup.s3_secret_key"))).toBeTruthy();
    });
  });

  it("shows success message when createBackup succeeds", async () => {
    mockApiClient.listBackups.mockResolvedValue({ success: true, backups: [] });
    mockApiClient.createBackup.mockResolvedValue({
      success: true,
      backup_path: "/tmp/backup.db",
    });

    render(<BackupTab config={{}} onChange={vi.fn()} />);

    await waitFor(() => {
      expect(screen.getByText("备份")).toBeTruthy();
    });

    const createButton = screen.getByText(t("backup.create_now"));
    fireEvent.click(createButton);

    await waitFor(() => {
      expect(screen.getByText(t("backup.created_at", { path: "/tmp/backup.db" }))).toBeTruthy();
    });
  });

  it("shows error message when createBackup fails", async () => {
    mockApiClient.listBackups.mockResolvedValue({ success: true, backups: [] });
    mockApiClient.createBackup.mockResolvedValue({ success: false, error: "Failed to create backup" });

    render(<BackupTab config={{}} onChange={vi.fn()} />);

    await waitFor(() => {
      expect(screen.getByText("备份")).toBeTruthy();
    });

    const createButton = screen.getByText(t("backup.create_now"));
    fireEvent.click(createButton);

    await waitFor(() => {
      expect(screen.getByText("Failed to create backup")).toBeTruthy();
    });
  });

  it("does not call restoreBackup when user cancels confirmation", async () => {
    mockApiClient.listBackups.mockResolvedValue({
      success: true,
      backups: [{ name: "backup.db", timestamp: 1704067200, size_bytes: 1024 }],
    });
    mockApiClient.restoreBackup.mockResolvedValue({ success: true });

    const confirmMock = vi.fn().mockReturnValue(false);
    vi.stubGlobal("confirm", confirmMock);

    render(<BackupTab config={{}} onChange={vi.fn()} />);

    await waitFor(() => {
      expect(screen.getByText("backup.db")).toBeTruthy();
    });

    const restoreButton = screen.getByText(t("backup.restore"));
    fireEvent.click(restoreButton);

    expect(confirmMock).toHaveBeenCalledWith(t("backup.restore_confirm", { name: "backup.db" }));
    expect(mockApiClient.restoreBackup).not.toHaveBeenCalled();
  });

  it("calls restoreBackup when user confirms", async () => {
    mockApiClient.listBackups.mockResolvedValue({
      success: true,
      backups: [{ name: "backup.db", timestamp: 1704067200, size_bytes: 1024 }],
    });
    mockApiClient.restoreBackup.mockResolvedValue({ success: true });

    const confirmMock = vi.fn().mockReturnValue(true);
    vi.stubGlobal("confirm", confirmMock);

    render(<BackupTab config={{}} onChange={vi.fn()} />);

    await waitFor(() => {
      expect(screen.getByText("backup.db")).toBeTruthy();
    });

    const restoreButton = screen.getByText(t("backup.restore"));
    fireEvent.click(restoreButton);

    expect(mockApiClient.restoreBackup).toHaveBeenCalledWith("backup.db");
  });

  it("does not call deleteBackup when user cancels confirmation", async () => {
    mockApiClient.listBackups.mockResolvedValue({
      success: true,
      backups: [{ name: "backup.db", timestamp: 1704067200, size_bytes: 1024 }],
    });
    mockApiClient.deleteBackup.mockResolvedValue({ success: true });

    const confirmMock = vi.fn().mockReturnValue(false);
    vi.stubGlobal("confirm", confirmMock);

    render(<BackupTab config={{}} onChange={vi.fn()} />);

    await waitFor(() => {
      expect(screen.getByText("backup.db")).toBeTruthy();
    });

    const deleteButton = screen.getByText(t("backup.delete"));
    fireEvent.click(deleteButton);

    expect(confirmMock).toHaveBeenCalledWith(t("backup.delete_confirm", { name: "backup.db" }));
    expect(mockApiClient.deleteBackup).not.toHaveBeenCalled();
  });

  it("calls deleteBackup when user confirms", async () => {
    mockApiClient.listBackups.mockResolvedValue({
      success: true,
      backups: [{ name: "backup.db", timestamp: 1704067200, size_bytes: 1024 }],
    });
    mockApiClient.deleteBackup.mockResolvedValue({ success: true });

    const confirmMock = vi.fn().mockReturnValue(true);
    vi.stubGlobal("confirm", confirmMock);

    render(<BackupTab config={{}} onChange={vi.fn()} />);

    await waitFor(() => {
      expect(screen.getByText("backup.db")).toBeTruthy();
    });

    const deleteButton = screen.getByText(t("backup.delete"));
    fireEvent.click(deleteButton);

    expect(mockApiClient.deleteBackup).toHaveBeenCalledWith("backup.db");
  });

  it("formats size correctly for bytes, KB, MB", async () => {
    mockApiClient.listBackups.mockResolvedValue({
      success: true,
      backups: [
        { name: "small.db", timestamp: 1704067200, size_bytes: 512 },
        { name: "medium.db", timestamp: 1704067200, size_bytes: 1024 * 50 },
        { name: "large.db", timestamp: 1704067200, size_bytes: 1024 * 1024 * 5 },
      ],
    });

    render(<BackupTab config={{}} onChange={vi.fn()} />);

    await waitFor(() => {
      expect(screen.getByText(/512 B/)).toBeTruthy();
      expect(screen.getByText(/50\.0 KB/)).toBeTruthy();
      expect(screen.getByText(/5\.0 MB/)).toBeTruthy();
    });
  });

  it("disables create button during create operation", async () => {
    mockApiClient.listBackups.mockResolvedValue({ success: true, backups: [] });
    mockApiClient.createBackup.mockImplementation(
      () => new Promise((resolve) => setTimeout(() => resolve({ success: true }), 100))
    );

    render(<BackupTab config={{}} onChange={vi.fn()} />);

    await waitFor(() => {
      expect(screen.getByText("备份")).toBeTruthy();
    });

    const createButton = screen.getByText(t("backup.create_now"));
    fireEvent.click(createButton);

    await waitFor(() => {
      expect((createButton as HTMLButtonElement).disabled).toBe(true);
    });
  });

  it("shows verify success message", async () => {
    mockApiClient.listBackups.mockResolvedValue({ success: true, backups: [] });
    mockApiClient.verifyBackup.mockResolvedValue({ success: true });

    render(<BackupTab config={{}} onChange={vi.fn()} />);

    await waitFor(() => {
      expect(screen.getByText("备份")).toBeTruthy();
    });

    const verifyButton = screen.getByText(t("backup.verify_connection"));
    fireEvent.click(verifyButton);

    await waitFor(() => {
      expect(screen.getByText(t("backup.verify_success"))).toBeTruthy();
    });
  });

  it("opens master password modal when createBackup returns action with setup mode", async () => {
    mockApiClient.listBackups.mockResolvedValue({ success: true, backups: [] });
    mockApiClient.createBackup.mockResolvedValue({
      success: false,
      error: "credentials_not_available",
      action: { type: "show_modal", target: "master_password", params: { mode: "setup" } }
    });

    render(<BackupTab config={{}} onChange={vi.fn()} />);

    await waitFor(() => {
      expect(screen.getByText("备份")).toBeTruthy();
    });

    const createButton = screen.getByText(t("backup.create_now"));
    fireEvent.click(createButton);

    await waitFor(() => {
      expect(screen.getByTestId("modal-mode")).toBeTruthy();
      expect(screen.getByTestId("modal-mode").textContent).toBe("setup");
    });
  });

  it("opens master password modal when createBackup returns action with unlock mode", async () => {
    mockApiClient.listBackups.mockResolvedValue({ success: true, backups: [] });
    mockApiClient.createBackup.mockResolvedValue({
      success: false,
      error: "master_password_not_unlocked",
      action: { type: "show_modal", target: "master_password", params: { mode: "unlock" } }
    });

    render(<BackupTab config={{}} onChange={vi.fn()} />);

    await waitFor(() => {
      expect(screen.getByText("备份")).toBeTruthy();
    });

    const createButton = screen.getByText(t("backup.create_now"));
    fireEvent.click(createButton);

    await waitFor(() => {
      expect(screen.getByTestId("modal-mode")).toBeTruthy();
      expect(screen.getByTestId("modal-mode").textContent).toBe("unlock");
    });
  });

  it("shows regular error when no action in response", async () => {
    mockApiClient.listBackups.mockResolvedValue({ success: true, backups: [] });
    mockApiClient.createBackup.mockResolvedValue({
      success: false,
      error: "Network error"
    });

    render(<BackupTab config={{}} onChange={vi.fn()} />);

    await waitFor(() => {
      expect(screen.getByText("备份")).toBeTruthy();
    });

    const createButton = screen.getByText(t("backup.create_now"));
    fireEvent.click(createButton);

    await waitFor(() => {
      expect(screen.getByText("Network error")).toBeTruthy();
    });
  });
});