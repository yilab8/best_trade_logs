# 最佳交易日誌（Best Trade Logs）

最佳交易日誌是一個以 Go 實作的網頁應用程式，協助主觀交易員完整記錄、檢視並持續改善每一筆交易。系統提供一套結構化流程，用來整理交易計畫、實際執行、事後回顧，以及像是出場後第 7 天與第 30 天的市場表現等延伸觀察。

## 功能特色

- **完整的交易紀錄表單**：紀錄商品、方向、進出場資訊、停損、目標、手續費、風險規劃與質化備註。
- **交易回顧**：整理結果摘要、心理狀態、改進想法，並可替交易加上標籤以利後續篩選。
- **自動化指標計算**：自動計算損益、報酬率、R 倍數、總風險與目標 R 值。
- **後續追蹤**：記錄出場後數日（如 +7、+30）的價格觀察，評估錯過的延續走勢。
- **未實現績效追蹤**：對於尚未出場的部位，可填寫參考收盤價來估算當前績效。
- **瀏覽器介面**：提供響應式 HTML 介面，用於瀏覽清單、編輯紀錄與查看交易細節。

## 系統需求

- [Go](https://go.dev/dl/) 1.21 以上版本（建議 1.22+）。
- （選用）MongoDB 6.x 以上版本，若需要長期儲存資料。

## 快速開始

1. 下載並安裝 Go，確認 `go version` 顯示的版本 ≥ 1.21。
2. 取得專案程式碼並切換到專案根目錄：

   ```bash
   git clone <repo-url>
   cd best_trade_logs
   ```

3. 執行伺服器並載入示範資料：

   ```bash
   go run ./cmd/server --seed
   ```

   `--seed` 只會在資料庫為空時建立兩筆示範交易，方便直接瀏覽 UI。

4. 開啟瀏覽器並前往 <http://localhost:8080> 查看交易日誌。

5. 如需停止伺服器，可在終端機按下 `Ctrl + C`。

## 執行方式

### 快速體驗（記憶體儲存）

預設建置會使用記憶體資料庫，適合在本地快速試用或體驗介面。

```bash
go run ./cmd/server
```

開啟瀏覽器並造訪 http://localhost:8080 進入交易日誌。

若希望自動帶入示範資料，可加入 `--seed` 參數或設定環境變數 `SEED_SAMPLE_DATA=true`：

```bash
go run ./cmd/server --seed
# 或
SEED_SAMPLE_DATA=true go run ./cmd/server
```

### 常用指令（Makefile）

專案提供簡易的 Makefile，協助快速執行常見工作：

```bash
make run        # go run ./cmd/server
make run-seed   # go run ./cmd/server --seed
make test       # go test ./...
```


### 使用 MongoDB

若需要完整持久化，可在編譯時加入 `mongodb` build tag。請先準備可用的 MongoDB 服務，並安裝官方 Go Driver（在可連線的環境執行 `go get go.mongodb.org/mongo-driver/mongo`）。

1. 匯出必要環境變數：

```bash
export MONGO_URI="mongodb://localhost:27017"
export MONGO_DB="best_trade_logs"
# 可選，預設為 "trades"
export MONGO_COLLECTION="trades"
```

2. 以啟用 MongoDB 的方式建置與執行：

```bash
go build -tags mongodb ./cmd/server
go run -tags mongodb ./cmd/server --mongo-uri mongodb://localhost:27017 --mongo-db best_trade_logs
```

啟用 MongoDB 後，伺服器會在啟動時自動連線，並將交易資料存入指定的集合中。

### 設定參數

- `--port` / `PORT`：HTTP 埠號（預設 `8080`）。
- `--seed` / `SEED_SAMPLE_DATA`：啟動時若資料庫為空是否載入示範交易（預設 `false`）。
- `--mongo-uri` / `MONGO_URI`：MongoDB 連線字串（使用 `mongodb` build tag 時必填）。
- `--mongo-db` / `MONGO_DB`：MongoDB 資料庫名稱（必填）。
- `--mongo-collection` / `MONGO_COLLECTION`：MongoDB 集合名稱（預設 `trades`）。

指令旗標會覆寫同名環境變數；若習慣使用 `.env` 檔，可自行 `source` 或使用像是 [direnv](https://direnv.net/) 的工具載入設定。

## 測試

執行單元測試：

```bash
go test ./...
```

測試涵蓋領域計算、儲存庫行為、服務流程與關鍵 HTTP Handler 邏輯。

## 專案結構

- `cmd/server`：應用程式進入點與儲存庫初始化邏輯。
- `internal/domain/trade`：核心交易實體與指標計算。
- `internal/service/trade`：交易流程的協調邏輯。
- `internal/storage`：記憶體與 MongoDB 的儲存實作。
- `internal/web`：HTTP Handler 與檢視模型。
- `internal/web/templates`：嵌入程式的 HTML 樣板。

## 後續可延伸的方向

- 若需要多人使用，可加入認證與帳號管理。
- 擴充標籤、策略或結果的篩選與搜尋功能。
- 整合行情 API，自動填入出場後追蹤價或每日收盤價。
- 匯出分析結果為試算表或儀表板。
