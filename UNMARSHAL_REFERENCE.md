# Unmarshal / ChromeUnmarshal リファレンスドキュメント

## 概要

このライブラリは、HTMLページからGoの構造体にデータを抽出するための2つの関数を提供します：

- `Unmarshal` - goqueryを使用したDOM操作ベースの抽出
- `ChromeUnmarshal` - Chrome DevTools Protocolを使用したブラウザベースの抽出

## Unmarshal 関数

### 関数シグネチャ

```go
func Unmarshal(v interface{}, selection *goquery.Selection, opt UnmarshalOption) error
```

### パラメータ

- `v interface{}` - データを格納する構造体のポインタ
- `selection *goquery.Selection` - 抽出対象のHTML要素
- `opt UnmarshalOption` - 抽出オプション

### 使用例

```go
type Article struct {
    Title   string    `find:"h1"`
    Author  string    `find:".author" attr:"data-name"`
    Price   float64   `find:".price" re:"([0-9.]+)"`
    Date    time.Time `find:".date" time:"2006-01-02"`
    Content string    `find:".content" html:""`
}

var article Article
err := scraper.Unmarshal(&article, page.Find(".article"), scraper.UnmarshalOption{})
```

## ChromeUnmarshal 関数

### 関数シグネチャ

```go
func ChromeUnmarshal(ctx context.Context, v interface{}, cssSelector string, opt UnmarshalOption) error
```

### パラメータ

- `ctx context.Context` - Chrome操作のコンテキスト
- `v interface{}` - データを格納する構造体のポインタ（構造体のみサポート）
- `cssSelector string` - 抽出対象要素のCSSセレクタ
- `opt UnmarshalOption` - 抽出オプション

### 使用例

```go
type Product struct {
    Name  string  `find:"h2"`
    Price float64 `find:".price" re:"([0-9.]+)"`
    Stock int     `find:".stock"`
}

var products []Product
err := chromeSession.Unmarshal(&products, ".product-list .item", scraper.UnmarshalOption{})
```

## UnmarshalOption

抽出動作をカスタマイズするオプション構造体：

```go
type UnmarshalOption struct {
    Attr   string         // 要素のテキストの代わりに属性値を取得
    Re     string         // テキストに正規表現を適用（1つのキャプチャグループが必要）
    Time   string         // time.Time用の時刻フォーマット
    Loc    *time.Location // 時刻パースのタイムゾーン
    Html   bool           // Text()の代わりにHtml()を取得
    Ignore string         // この文字列と一致する場合、ゼロ値を設定
}
```

## 構造体フィールドタグ

### 基本タグ

#### `find`

子要素を指定するCSSセレクタ

```go
type Item struct {
    Title string `find:"h2"`           // <h2>要素のテキスト
    Link  string `find:"a" attr:"href"` // <a>要素のhref属性
}
```

#### `attr`

要素の属性値を取得

```go
type Link struct {
    Url   string `attr:"href"`
    Title string `attr:"title"`
}
```

#### `html`

要素の内部HTMLを取得（`attr`より優先）

```go
type Content struct {
    RawHtml string `html:""`
}
```

#### `re`

正規表現によるテキスト抽出（1つのキャプチャグループが必要）

```go
type Price struct {
    Amount float64 `find:".price" re:"￥([0-9,]+)"`
}
```

#### `time`

time.Time型のフィールド用時刻フォーマット

```go
type Event struct {
    Date time.Time `find:".date" time:"2006-01-02 15:04:05"`
}
```

#### `ignore`

指定した文字列と一致する場合、ゼロ値を設定

```go
type Product struct {
    Stock int `find:".stock" ignore:"在庫切れ"`
}
```

## サポートされるデータ型

### 基本型

- `string`
- `int`, `int8`, `int16`, `int32`, `int64`（カンマ区切り数値をサポート）
- `uint`, `uint8`, `uint16`, `uint32`, `uint64`（カンマ区切り数値をサポート）
- `float32`, `float64`（`ExtractNumber`関数による柔軟な数値抽出）
- `time.Time`（`time`タグが必要）

### 複合型

- `[]T` - スライス（各要素に対して抽出）
- `*T` - ポインタ（要素が見つからない場合はnil）
- `struct` - ネストした構造体

### カスタム型

`Unmarshaller`インターフェースを実装した型：

```go
type Unmarshaller interface {
    Unmarshal(s string) error
}
```

## 数値抽出

### ExtractNumber 関数

```go
func ExtractNumber(in string) (float64, error)
```

様々な形式の数値文字列から数値を抽出：

```go
ExtractNumber("￥1,234.56円")  // 1234.56
ExtractNumber("価格: 999")     // 999
ExtractNumber("$12.34 USD")   // 12.34
```

## エラー処理

### 主要なエラー型

#### `UnmarshalMustBePointerError`

引数がポインタでない場合

#### `UnmarshalUnexportedFieldError`

非公開フィールドを含む構造体の場合

#### `UnmarshalFieldError`

特定のフィールドでエラーが発生した場合（フィールド名を含む）

#### `UnmarshalParseNumberError`

数値のパースに失敗した場合

## Unmarshal vs ChromeUnmarshal の違い

| 機能 | Unmarshal | ChromeUnmarshal |
|------|-----------|-----------------|
| 実行環境 | goquery (静的HTML) | Chrome DevTools Protocol |
| パフォーマンス | 高速 | 低速（ブラウザ操作） |
| JavaScript | 非対応 | 対応 |
| 動的コンテンツ | 非対応 | 対応 |
| CSSセレクタ | goquery仕様 | Chrome準拠 |
| nth-of-type | 基本対応 | 拡張対応（odd/even/式） |
| 入力形式 | goquery.Selection | CSSセレクタ文字列 |
| 対象型 | 任意の型 | 構造体のみ |

## 高度な使用例

### ネストした構造体

```go
type Article struct {
    Title  string `find:"h1"`
    Meta   Meta   `find:".meta"`
    Tags   []Tag  `find:".tags .tag"`
}

type Meta struct {
    Author string `find:".author"`
    Date   time.Time `find:".date" time:"2006-01-02"`
}

type Tag struct {
    Name string `find:"span"`
    Url  string `find:"a" attr:"href"`
}
```

### ChromeUnmarshalでの動的コンテンツ

```go
// JavaScriptで生成された要素も取得可能
type DynamicContent struct {
    LoadedText string `find:".js-loaded"`
    AjaxData   string `find:"[data-ajax-content]" attr:"data-value"`
}

err := chromeSession.Unmarshal(&content, ".dynamic-section", scraper.UnmarshalOption{})
```

### カスタムUnmarshaller

```go
type Price struct {
    Amount   float64
    Currency string
}

func (p *Price) Unmarshal(s string) error {
    // "￥1,234" -> Amount: 1234, Currency: "JPY"
    re := regexp.MustCompile(`^([￥$€])([0-9,]+)`)
    matches := re.FindStringSubmatch(s)
    if len(matches) != 3 {
        return fmt.Errorf("invalid price format: %s", s)
    }
    
    switch matches[1] {
    case "￥":
        p.Currency = "JPY"
    case "$":
        p.Currency = "USD"
    case "€":
        p.Currency = "EUR"
    }
    
    amount, err := strconv.ParseFloat(strings.ReplaceAll(matches[2], ",", ""), 64)
    if err != nil {
        return err
    }
    p.Amount = amount
    return nil
}
```

## ベストプラクティス

1. **適切な関数選択**
   - 静的HTMLの場合は`Unmarshal`を使用
   - JavaScript必須の場合は`ChromeUnmarshal`を使用

2. **エラーハンドリング**
   - フィールド固有のエラーは`UnmarshalFieldError`で詳細確認
   - 正規表現は事前にテスト

3. **パフォーマンス**
   - 大量データ処理には`Unmarshal`を優先
   - `ChromeUnmarshal`は必要最小限に留める

4. **デバッグ**
   - 正規表現は単純に保つ
   - 複雑な抽出は段階的に構築
