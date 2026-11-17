# Changelog

All notable changes to **NullWake** are documented in this file.

## [v1.1.0] - 2026-11-17

### Extensible Service Monitoring (可擴展自定義服務監控模組)
- **有序服務指標協定 (`[]ServiceMetric`)：** 將 `ServiceStatus.metrics` 改為有序陣列 `[{"label": "...", "value": "..."}]`（同時相容舊版 `script` JSON map 輸出），徹底解決 Go Map 亂序導致 Android 卡片欄位跳動的問題，並套用等寬數字（`tnum`）。
- **免腳本宣告式探測 (`tcp` / `http_json`) 與 Systemd 記憶體採集：**
  - 新增 `type = "tcp"`：透過標準 `net.Dialer` 檢查本地服務埠（預設 `500ms` 逾時）並回報 TCP 握手延遲。
  - 新增 `type = "http_json"`：針對 JSON 健康/狀態端點進行 `fields` dot-path 萃取（如 `players.online`）與 `summary_template` 模板格式化。
  - 增強 `type = "systemd"`：透過 DBus `MemoryCurrent` 與 `/proc/<MainPID>/status` `VmRSS` 自動附帶行程實體記憶體占用。
  - 所有探測類型統一彙整 `/run/nullwake.sock` 記憶體 Ring Buffer 歷史事件，且嚴格維持無訂閱零待機執行。
- **互動式探索指令 (`nullwake-agent discover`)：** 內建標準函式庫探索工具，掃描活躍 `systemd` 單元（過濾系統底層元件）與 `/proc/net/tcp{,6}` 監聽埠，互動選擇後預覽並附加 `[[services]]` TOML 設定；同步新增 `examples/services.example.toml` 範例模板。

### Android UI, Theme & Release Signing
- **主介面卡片標籤：** 繁體中文介面的 `CPU` 與 `RAM` 卡片標題統一維持英文顯示。
- **預設淺藍色主題與過渡動畫：** 將非動態配色的預設品牌主色調整為高對比淺藍色（Light Blue），並為淺色／深色／AMOLED 純黑切換加入平滑色彩漸變動畫，同步最佳化頁面切換與服務卡片尺寸變化動畫。
- **持久化 APK 簽章（v1/v2/v3）：** CI 改為使用固定儲存於 GitHub Secrets 的 PKCS12 金鑰簽署所有 Debug 與 Release APK，確保後續版本更新不再出現簽名不一致。

---

## [v1.0.1] - 2026-11-17

### Bug Fixes & Performance (連線與採集修正)
- **啟動與連線速度最佳化：**
  - `tsnet-bridge` 改為訂閱 `NotifyInitialState | NotifyInitialStatus | NotifyPeerChanges`，並重用單一 `http.Transport`，將節點發現輪詢間隔在初始等待期間縮短為 `500ms`，大幅降低 App 啟動等待時間。
  - 將冷啟動 WebSocket 握手逾時放寬至 `15s`，於 `NullWakeViewModel` 序列化 `subscribeJob` 防止重複連線競爭，並在首次連線逾時時自動重試一次（消除切換或喚醒時 `context deadline exceeded` 錯誤彈窗與節點狀態閃爍）。
- **Intel GPU 與系統感測器數據修正：**
  - 修正 Intel iGPU 在無 `CAP_PERFMON` 權限時誤讀 `gt_cur_freq_mhz / gt_max_freq_mhz`（例如待機顯示 88.3%）的問題，新增無特權 sysfs `rc6_residency_ms`（`1 - delta_rc6 / elapsed`）與 `rps_act_freq_mhz` 計算邏輯。
  - 修正 CPU 溫度優先權順序，確保 `coretemp` / `k10temp` / `zenpower` 優先於主機板 ACPI `acpitz`。
  - 修正網路流量採集排除 `tun*`、`tap*`、`veth*`、`docker*`、`br-*`、`tailscale*`、`wg*` 等虛擬介面，避免 VPN 或容器流量重複計算。
  - 於訂閱首筆快照加入短暫基線預熱（`150ms`），避免剛連線的第一幀出現 `0%` 假低點。
- **Linux Agent 安全與穩定性加固：**
  - Unix Push Socket (`/run/nullwake.sock`) 新增最大並發連線數限制（`32`）與 `30s` 讀取逾時，並限制記憶體 Ring Buffer 最大動態服務數（`64`），防止本地端連線耗盡資源。
  - WebSocket Hub 明確設定 `ReadLimit(16KB)` 防止異常封包。

### UI & Features (Android Material Design 3 與新功能)
- **上下行網路顏色區分：** 下行（`↓`）使用紫羅蘭色、上行（`↑`）使用玫瑰粉色，同步套用至箭頭、即時速度數值、圖例與雙線圖表。
- **Material Design 3 全面最佳化：** 導入 `ElevatedCard`、`OutlinedCard`、`Shapes` 與語意化色彩體系，統一元件圓角與層次。
- **主題與動態配色：**
  - 支援 **跟隨系統 / 淺色 / 深色** 三段式主題切換。
  - 於深色模式下提供 **AMOLED 純黑背景（`#000000`）** 開關。
  - 於「設定」頁面新增 **動態配色（Dynamic Color）** 開關（Android 12+ 支援）。
- **繁體中文支援：** 新增完整繁體中文（台灣）與英文語系資源，可於設定內即時切換或跟隨系統。
- **自訂設備名稱：** 支援於頂部節點切換選單或設定頁面修改各節點顯示名稱（僅儲存於本機）。

### Release Packaging
- 精簡 GitHub Release 資產為 3 個檔案：`nullwake-android-arm64-v8a-release.apk`、`nullwake-android-arm64-v8a-debug.apk`、`nullwake-agent-linux-amd64`。

---

## [v1.0.0] - 2026-11-17

### Initial Release
- Linux 無特權 / 零待機採集 Daemon（`nullwake-agent`），內嵌 Tailscale `tsnet` 與 `/run/nullwake.sock` 記憶體環形事件緩衝區。
- `tsnet-bridge`（`gomobile` AAR 封裝，支援 Android 11+ `netmon` 介面回退與沙盒 `TS_LOGS_DIR` 設定，無需 `VpnService`）。
- 原生 Kotlin + Jetpack Compose Android 監控檢視器（原生 `Canvas` 60 點折線圖、設備切換器與服務事件抽屜）。
