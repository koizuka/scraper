# Scraper Library - Unified Interface Enhancement TODO

## 必要な機能拡張

統一Scraperインターフェースに以下の機能を追加することで、複雑なスクレイピング処理をよりスムーズに移行できます。

### 1. 条件付きアクション機能

現在、Chrome専用コードでは `IfExistsAction` ヘルパーを使用していますが、統一インターフェースには同等の機能がありません。

```go
type UnifiedScraper interface {
    // 既存のメソッド...
    
    // 要素が存在する場合のみアクションを実行
    IfExists(selector string, action func() error) error
    
    // 要素の存在チェック
    ElementExists(selector string) (bool, error)
}
```

**用途：**
- ログイン状態の判定
- オプショナルなフォーム要素の処理
- 条件付きボタンクリック

### 2. Sleep/待機機能

ページ遷移後の安定化や、動的コンテンツの読み込み待ちに必要。

```go
type UnifiedScraper interface {
    // 既存のメソッド...
    
    // 指定時間待機
    Sleep(duration time.Duration) error
}
```

**用途：**
- ページ遷移後の安定化待ち
- Ajax処理の完了待ち
- レート制限対応

### 3. ダウンロードトリガー機能

現在のダウンロード機能は基本的ですが、クリックアクションと組み合わせたダウンロード処理が必要。

```go
type UnifiedScraper interface {
    // 既存のメソッド...
    
    // アクション実行後のファイルダウンロード
    DownloadWithAction(options UnifiedDownloadOptions, triggerAction func() error) (string, error)
}
```

**用途：**
- ボタンクリック→CSVダウンロード
- フォーム送信→ファイル生成→ダウンロード

## 実装優先度

### 高優先度（必須）
- [ ] `IfExists` / `ElementExists` - 条件付き処理に不可欠
- [ ] `Sleep` - ページ安定化に必要

### 中優先度（推奨）
- [ ] `DownloadWithAction` - ダウンロード処理の改善

### 低優先度（将来的）
- [ ] JavaScript実行機能（デバイス認証の判定など）

## 実装上の注意

### Chrome版実装
- 既存の `ChromeSession` メソッドを活用
- `chromedp` パッケージの機能を直接使用

### HTTP版実装
- 可能な範囲で同等の動作を提供
- 制限がある場合は適切にエラーまたは警告

### 後方互換性
- 既存のインターフェースは変更しない
- 新機能のみ追加

## 移行予定のコード

以下のコードが統一インターフェースへの移行を待機中：

### sbiSecurity.go
- `IfExistsAction` 使用箇所の移行
- `chromedp.Sleep` の代替
- ダウンロード処理の改善

## テスト要件

各機能の実装時には以下をテスト：
- Chrome版とHTTP版の動作一貫性
- エラー処理の適切性
- パフォーマンスへの影響