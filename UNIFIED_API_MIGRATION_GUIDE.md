# 統一スクレイピングAPI移行ガイド

このガイドでは、既存のChrome専用またはHTTP専用コードを統一インターフェースに移行する方法を説明します。

## 目次

1. [移行の利点](#移行の利点)
2. [基本的な移行パターン](#基本的な移行パターン)
3. [メソッド対応表](#メソッド対応表)
4. [実践的な移行例](#実践的な移行例)
5. [注意点とベストプラクティス](#注意点とベストプラクティス)

## 移行の利点

### ✅ **コードの統一化**

- Chrome版とHTTP版で同じコードが使用可能
- 実行時にスクレイピング方法を切り替え可能
- テスト環境（HTTP）と本番環境（Chrome）の切り替えが容易

### ✅ **既存コードの保護**

- 既存のメソッドはすべて保持
- 段階的な移行が可能
- 後方互換性を完全に維持

## 基本的な移行パターン

### パターン1: 基本的な置き換え

**移行前（Chrome専用）:**

```go
func scrapeWithChrome(session *scraper.Session) error {
    chromeSession, cancel, err := session.NewChrome()
    if err != nil {
        return err
    }
    defer cancel()
    
    err = chromedp.Run(chromeSession.Ctx,
        chromedp.Navigate("https://example.com"),
        chromedp.WaitVisible("h1", chromedp.ByQuery),
        chromeSession.SaveHtml(nil),
    )
    if err != nil {
        return err
    }
    
    var data struct {
        Title string `find:"h1"`
    }
    return chromeSession.Unmarshal(&data, "body", scraper.UnmarshalOption{})
}
```

**移行後（統一インターフェース）:**

```go
func scrapeUnified(scraperInstance scraper.UnifiedScraper) error {
    err := scraperInstance.Navigate("https://example.com")
    if err != nil {
        return err
    }
    
    err = scraperInstance.WaitVisible("h1")
    if err != nil {
        return err
    }
    
    _, err = scraperInstance.SavePage()
    if err != nil {
        return err
    }
    
    var data struct {
        Title string `find:"h1"`
    }
    return scraperInstance.ExtractData(&data, "body", scraper.UnmarshalOption{})
}
```

### パターン2: 実行時切り替え対応

**移行前（固定方式）:**

```go
func main() {
    var logger scraper.ConsoleLogger
    session := scraper.NewSession("test", logger)
    
    // 常にChrome使用
    chromeSession, cancel, err := session.NewChrome()
    if err != nil {
        log.Fatal(err)
    }
    defer cancel()
    
    scrapeWithChrome(session)
}
```

**移行後（設定可能）:**

```go
func main() {
    useChrome := flag.Bool("chrome", false, "Use Chrome scraping")
    flag.Parse()
    
    var logger scraper.ConsoleLogger
    session := scraper.NewSession("test", logger)
    
    var scraperInstance scraper.UnifiedScraper
    var cancel context.CancelFunc
    
    if *useChrome {
        chromeSession, c, err := session.NewChrome()
        if err != nil {
            log.Fatal(err)
        }
        scraperInstance = chromeSession
        cancel = c
        defer cancel()
    } else {
        scraperInstance = session
    }
    
    scrapeUnified(scraperInstance)
}
```

## メソッド対応表

### Chrome専用コード → 統一インターフェース

| 移行前 | 移行後 | 説明 |
|--------|--------|------|
| `chromedp.Navigate(url)` | `scraper.Navigate(url)` | ページ遷移 |
| `chromedp.WaitVisible(sel)` | `scraper.WaitVisible(sel)` | 要素の表示待ち |
| `chromedp.SendKeys(sel, val)` | `scraper.SendKeys(sel, val)` | フォーム入力 |
| `chromedp.Click(sel)` | `scraper.Click(sel)` | クリック操作 |
| `chromeSession.SaveHtml(nil)` | `scraper.SavePage()` | HTML保存 |
| `chromeSession.Unmarshal(&v, sel, opt)` | `scraper.ExtractData(&v, sel, opt)` | データ抽出 |
| `chromeSession.DownloadFile(&f, opts, actions...)` | `scraper.DownloadResource(opts)` | ファイルダウンロード |

### HTTP専用コード → 統一インターフェース

| 移行前 | 移行後 | 説明 |
|--------|--------|------|
| `session.GetPage(url)` | `scraper.Navigate(url)` | ページ取得 |
| `session.FormAction(page, sel, params)` | `scraper.SubmitForm(sel, params)` | フォーム送信 |
| `session.FollowAnchorText(page, text)` | `scraper.FollowAnchor(text)` | リンク辿り |
| `scraper.Unmarshal(&v, selection, opt)` | `scraper.ExtractData(&v, sel, opt)` | データ抽出 |

## 実践的な移行例

### 例1: SBI証券スクレイピングの移行

**移行前のChrome専用コード:**

```go
func getSbiSecurityChrome(param ParamRegistry, service StatementReceiver, session *scraper.Session) error {
    chromeSession, cancel, err := session.NewChrome()
    if err != nil {
        return err
    }
    defer cancel()
    
    err = chromedp.Run(chromeSession.Ctx,
        chromedp.Navigate(`https://www.sbisec.co.jp/`),
        chromedp.WaitVisible(`form[name=form_login]`, chromedp.ByQuery),
        chromedp.SendKeys(`input[name=user_id]`, param.Param(ParamUser), chromedp.ByQuery),
        chromedp.SendKeys(`input[name=user_password]`, param.Param(ParamPassword), chromedp.ByQuery),
        chromedp.Click(`[name=ACT_login]`, chromedp.ByQuery),
    )
    if err != nil {
        return err
    }
    
    // CSVダウンロード処理...
    return nil
}
```

**移行後の統一インターフェース版:**

```go
func getSbiSecurityUnified(param ParamRegistry, service StatementReceiver, scraperInstance scraper.UnifiedScraper) error {
    scraperInstance.SetDebugStep("SBI証券ログイン")
    defer scraperInstance.ClearDebugStep()
    
    err := scraperInstance.Navigate(`https://www.sbisec.co.jp/`)
    if err != nil {
        return err
    }
    
    err = scraperInstance.WaitVisible(`form[name=form_login]`)
    if err != nil {
        return err
    }
    
    err = scraperInstance.SendKeys(`input[name=user_id]`, param.Param(ParamUser))
    if err != nil {
        return err
    }
    
    err = scraperInstance.SendKeys(`input[name=user_password]`, param.Param(ParamPassword))
    if err != nil {
        return err
    }
    
    err = scraperInstance.Click(`[name=ACT_login]`)
    if err != nil {
        return err
    }
    
    // ポートフォリオページへ遷移
    err = scraperInstance.FollowAnchor("ポートフォリオ")
    if err != nil {
        return err
    }
    
    // データ抽出
    type PositionData struct {
        Items []struct {
            Name  string  `find:".stock-name"`
            Price float64 `find:".price" re:"([0-9,]+)"`
        } `find:".position-row"`
    }
    
    var positions PositionData
    err = scraperInstance.ExtractData(&positions, ".portfolio-table", scraper.UnmarshalOption{})
    if err != nil {
        return err
    }
    
    scraperInstance.Printf("取得したポジション数: %d", len(positions.Items))
    return nil
}
```

### 例2: 呼び出し側のファクトリーパターン

```go
type ScraperConfig struct {
    UseChrome bool
    Headless  bool
    Timeout   time.Duration
}

func createScraper(config ScraperConfig, session *scraper.Session) (scraper.UnifiedScraper, context.CancelFunc, error) {
    if config.UseChrome {
        options := scraper.NewChromeOptions{
            Headless: config.Headless,
            Timeout:  config.Timeout,
        }
        chromeSession, cancel, err := session.NewChromeOpt(options)
        if err != nil {
            return nil, nil, err
        }
        return chromeSession, cancel, nil
    } else {
        // HTTP版はキャンセル不要
        return session, func() {}, nil
    }
}

func main() {
    config := ScraperConfig{
        UseChrome: os.Getenv("USE_CHROME") == "true",
        Headless:  true,
        Timeout:   30 * time.Second,
    }
    
    var logger scraper.ConsoleLogger
    session := scraper.NewSession("unified-example", logger)
    
    scraperInstance, cancel, err := createScraper(config, session)
    if err != nil {
        log.Fatal(err)
    }
    defer cancel()
    
    // 統一インターフェースで処理
    err = getSbiSecurityUnified(param, service, scraperInstance)
    if err != nil {
        log.Fatal(err)
    }
}
```

## 注意点とベストプラクティス

### ⚠️ **移行時の注意点**

#### 1. **ダウンロード機能の違い**

```go
// Chrome版: 高機能だが複雑
chromeSession.DownloadFile(&filename, options, 
    chromedp.Click(".download-button"),
)

// 統一版: シンプルだが機能制限
filename, err := scraper.DownloadResource(options)
// → 複雑なダウンロードは既存メソッドを併用
```

#### 2. **コンテキスト処理**

```go
// Chrome版では明示的なコンテキスト管理が必要
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

// 統一版ではタイムアウトは内部処理
scraper.WaitVisible("selector") // タイムアウトは実装依存
```

#### 3. **エラーハンドリング**

```go
// 統一インターフェースでは具体的なエラー情報が制限される場合がある
err := scraper.Click("selector")
if err != nil {
    // Chrome固有のエラー情報が必要な場合は型アサーションを使用
    if chromeSession, ok := scraper.(*scraper.ChromeSession); ok {
        // Chrome固有の処理
    }
}
```

### ✅ **ベストプラクティス**

#### 1. **段階的移行**

```go
// Phase 1: 統一インターフェースを受け取れるように変更
func processData(scraperInstance scraper.UnifiedScraper) error {
    // 統一インターフェースを使用した処理
}

// Phase 2: 既存コードから徐々に移行
func oldFunction(session *scraper.Session) error {
    return processData(session) // 既存のSessionも統一インターフェースとして渡せる
}
```

#### 2. **設定駆動の切り替え**

```go
type Config struct {
    ScrapingMode string `json:"scraping_mode"` // "http" or "chrome"
    Chrome       struct {
        Headless bool          `json:"headless"`
        Timeout  time.Duration `json:"timeout"`
    } `json:"chrome"`
}

func createScraperFromConfig(config Config, session *scraper.Session) scraper.UnifiedScraper {
    switch config.ScrapingMode {
    case "chrome":
        chromeSession, _, _ := session.NewChromeOpt(scraper.NewChromeOptions{
            Headless: config.Chrome.Headless,
            Timeout:  config.Chrome.Timeout,
        })
        return chromeSession
    default:
        return session
    }
}
```

#### 3. **テスト戦略**

```go
func TestUnifiedScraping(t *testing.T) {
    testCases := []struct {
        name    string
        scraper scraper.UnifiedScraper
    }{
        {
            name:    "HTTP Scraper",
            scraper: scraper.NewSession("test-http", logger),
        },
        {
            name:    "Chrome Scraper", 
            scraper: setupChromeSession(t),
        },
    }
    
    for _, tc := range testCases {
        t.Run(tc.name, func(t *testing.T) {
            err := processData(tc.scraper)
            assert.NoError(t, err)
        })
    }
}
```

#### 4. **既存機能の活用**

```go
func advancedDownload(scraperInstance scraper.UnifiedScraper) error {
    // 統一インターフェースでシンプルなケースを処理
    filename, err := scraperInstance.DownloadResource(options)
    if err != nil {
        // 失敗した場合は型固有の機能を使用
        if chromeSession, ok := scraperInstance.(*scraper.ChromeSession); ok {
            return chromeSession.DownloadFile(&filename, chromeOptions, 
                chromedp.Click(".download-button"),
            )
        }
        return err
    }
    return nil
}
```

## まとめ

統一インターフェースへの移行により：

1. **Chrome版とHTTP版のコードが統一される**
2. **実行時にスクレイピング方法を切り替え可能**  
3. **既存コードは完全に保持される**
4. **段階的な移行が可能**

移行は急ぐ必要はありません。新しい機能から統一インターフェースを使用し、既存コードは必要に応じて徐々に移行してください。
