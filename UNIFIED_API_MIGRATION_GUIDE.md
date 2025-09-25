# 統一スクレイピングAPI移行ガイド

このガイドでは、既存のChrome専用またはHTTP専用コードを統一Action-baseインターフェースに移行する方法を説明します。

## 🎉 新しいAction-based設計

v2.0では、chromedp.Run()ライクなAction-based APIに刷新されました。chromedpの優れた可変引数スタイルを統一APIでも実現し、より簡潔で保守しやすいコードが書けるようになりました。

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

### パターン1: chromedp.Run()スタイルの移行

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

**移行後（統一Action-based API）:**

```go
func scrapeUnified(scraperInstance scraper.UnifiedScraper) error {
    // chromedp.Run()と同じスタイルで統一API使用
    err := scraperInstance.Run(
        scraper.Navigate("https://example.com"),
        scraper.WaitVisible("h1"),
        scraper.SavePage(),
    )
    if err != nil {
        return err
    }

    // データ抽出も統一
    var data struct {
        Title string `find:"h1"`
    }
    return scraperInstance.Run(
        scraper.ExtractData(&data, "body", scraper.UnmarshalOption{}),
    )
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

## Action対応表

### Chrome専用コード → 統一Action API

| 移行前 | 移行後 | 説明 |
|--------|--------|------|
| `chromedp.Navigate(url)` | `scraper.Navigate(url)` | ページ遷移 |
| `chromedp.WaitVisible(sel)` | `scraper.WaitVisible(sel)` | 要素の表示待ち |
| `chromedp.SendKeys(sel, val)` | `scraper.SendKeys(sel, val)` | フォーム入力 |
| `chromedp.Click(sel)` | `scraper.Click(sel)` | クリック操作 |
| `chromedp.Sleep(duration)` | `scraper.Sleep(duration)` | 待機（replay時自動スキップ） |
| `chromeSession.SaveHtml(nil)` | `scraper.SavePage()` | HTML保存 |
| `chromeSession.Unmarshal(&v, sel, opt)` | `scraper.ExtractData(&v, sel, opt)` | データ抽出 |

### 実行方法の比較

**Chrome専用 (移行前):**
```go
err = chromedp.Run(ctx,
    chromedp.Navigate(url),
    chromedp.WaitVisible(sel),
    chromedp.Click(sel),
)
```

**統一Action API (移行後):**
```go
err = scraper.Run(
    scraper.Navigate(url),
    scraper.WaitVisible(sel),
    scraper.Click(sel),
)
```

### HTTP専用コード → 統一Action API

HTTPスクレイピングでも同じActionを使用できます：

| 移行前 | 移行後 | 説明 |
|--------|--------|------|
| `session.GetPage(url)` + 状態管理 | `scraper.Navigate(url)` | ページ取得と状態更新 |
| `session.FormAction(page, sel, params)` | `scraper.SendKeys()` + `scraper.Click()` | フォーム操作を分割 |
| `session.FollowAnchorText(page, text)` | `scraper.Click()` with text selector | リンククリック |
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

**移行後の統一Action API版:**

```go
func getSbiSecurityUnified(param ParamRegistry, service StatementReceiver, scraperInstance scraper.UnifiedScraper) error {
    scraperInstance.SetDebugStep("SBI証券ログイン")
    defer scraperInstance.ClearDebugStep()

    // ログイン処理をAction-baseスタイルで実行
    err := scraperInstance.Run(
        scraper.Navigate(`https://www.sbisec.co.jp/`),
        scraper.WaitVisible(`form[name=form_login]`),
        scraper.SendKeys(`input[name=user_id]`, param.Param(ParamUser)),
        scraper.SendKeys(`input[name=user_password]`, param.Param(ParamPassword)),
        scraper.Click(`[name=ACT_login]`),
        scraper.SavePage(),
    )
    if err != nil {
        return err
    }

    // データ抽出処理
    type PositionData struct {
        Items []struct {
            Name  string  `find:".stock-name"`
            Price float64 `find:".price" re:"([0-9,]+)"`
        } `find:".position-row"`
    }

    var positions PositionData
    err = scraperInstance.Run(
        scraper.Click("a:contains('ポートフォリオ')"), // ポートフォリオページへ
        scraper.WaitVisible(".portfolio-table"),
        scraper.ExtractData(&positions, ".portfolio-table", scraper.UnmarshalOption{}),
    )
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

## 新機能とベストプラクティス

### 🎯 **Action-based APIの利点**

#### 1. **カスタムAction作成**

```go
// よく使う操作をActionとして部品化
func JCBLogin(userId, password string) scraper.UnifiedAction {
    return scraper.ActionFunc(func(s scraper.UnifiedScraper) error {
        return s.Run(
            scraper.Navigate("https://my.jcb.co.jp/Login"),
            scraper.WaitVisible("form[name='loginForm']"),
            scraper.SendKeys("#userId", userId),
            scraper.SendKeys("#password", password),
            scraper.Sleep(2*time.Second),
            scraper.Click("#loginButtonAD"),
        )
    })
}

// 使用例
err := scraper.Run(
    JCBLogin("myuser", "mypass"),
    scraper.SavePage(),
    // 続きの処理...
)
```

#### 2. **条件分岐処理**

```go
// Action内で条件分岐も可能
func ConditionalLogin(scraper scraper.UnifiedScraper) error {
    err := scraper.Run(
        scraper.Navigate("https://example.com/login"),
        scraper.SavePage(),
    )
    if err != nil {
        return err
    }

    // 現在のURLで処理を分岐
    currentURL, _ := scraper.GetCurrentURL()
    if strings.Contains(currentURL, "yahoo.co.jp") {
        return scraper.Run(
            scraper.WaitVisible(`input[name="handle"]`),
            scraper.SendKeys(`input[name="handle"]`, userId),
            scraper.Click(`button[class*="riff-bg-key"]`),
        )
    } else {
        return scraper.Run(
            scraper.WaitVisible("#loginForm"),
            scraper.SendKeys("#username", userId),
            scraper.Click("#submit"),
        )
    }
}
```

#### 3. **Replay Mode完全対応**

```go
// Sleep は replay mode で自動的にスキップされる
err := scraper.Run(
    scraper.Navigate("https://example.com"),
    scraper.Sleep(3*time.Second), // 記録時のみ実行、リプレイ時はスキップ
    scraper.Click("#button"),
)

// IsReplayMode()で状態確認も可能
if !scraper.IsReplayMode() {
    scraper.Printf("実際のネットワーク通信中...")
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

Action-based統一インターフェースへの移行により：

1. **🎯 chromedp.Run()スタイルで直感的な操作**
2. **🔄 Chrome版とHTTP版のコードが完全統一**
3. **⚡ Replay Mode で爆速開発・デバッグ**
4. **🧩 カスタムActionで高い再利用性**
5. **🛠️ 条件分岐とエラーハンドリングが簡潔**

## 移行のポイント

### ✅ **すぐに移行すべき理由**

- **開発効率の劇的向上**: Replay modeでスクレイピング開発が爆速化
- **コード保守性**: chromedp.Run()スタイルで可読性アップ
- **テスト容易性**: HTTP/Chrome両方で同じテストコード
- **条件分岐の簡潔性**: ActionFuncが不要になりGoの標準制御構文で記述

### 🚀 **推奨移行手順**

1. **新機能はAction-based APIで実装**
2. **既存の問題箇所から段階的に移行**
3. **Replay modeを活用して開発を高速化**
4. **カスタムActionで共通処理を部品化**

この新設計により、スクレイピングコードの開発・保守・テストが格段に効率化されます！
