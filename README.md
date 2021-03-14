# scraper

## 基本的な使い方
```go
	// 初期化
	var logger scraper.ConsoleLogger
	session := scraper.NewSession("session-name", logger) // session-name はログフォルダ名になる

	// cookieを読む ( sessio-name/cookie というファイルを使う)
	err := session.LoadCookie()
	if err != nil {
		log.Fatal(err)
	}

	// ページを開く
	page, err := session.GetPage("https://example.com")
	if err != nil {
		log.Fatal(err)
	}

	// form 送信
	form, err := page.Form("form") // CSS selector でformを特定する
	if err != nil {
		log.Fatal(err)
	}
	_ = form.Set("id", id)
	_ = form.Set("password", password)
	resp, err := session.Submit(form) // レスポンスを得る
	if err != nil {
		log.Fatal(err)
	}
	page, err = resp.Page() // レスポンスからページにする
	if err != nil {
		log.Fatal(err)
	}

	// cookie を保存
	err := session.SaveCookie()
	if err != nil {
		log.Fatal(err)
	}

	// Pageから読み取る
	type Link struct {
		Href string `attr:"href"`
		Text string
	}
	var links []Link
	err := scraper.Unmarshal(&links, page.Find("div.items a"), scraper.UnmarshalOption{})
	if err != nil {
		log.Fatal(err)
	}
	// -> links に <div class="items"> 以下にある <a> タグの href とテキスト要素を収集する

```

## メモ
https://github.com/juju/persistent-cookiejar は max-age がないクッキーを永続化してくれないので
https://github.com/orirawlings/persistent-cookiejar を使ったらいけた。神

