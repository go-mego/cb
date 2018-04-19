# Circuit Breaker [![GoDoc](https://godoc.org/github.com/go-mego/cb?status.svg)](https://godoc.org/github.com/go-mego/cb)

Circuit Breaker 是基於斷路器理念用以防止錯誤不斷地發生的套件。當函式發生錯誤到一定的次數（或比例）就會直接斷路並回傳錯誤訊息，而不是嘗試執行可能發生錯誤的函式以防浪費資源。

過了一段時間後斷路器會呈現「半開放」的狀態並允許嘗試執行可能發生錯誤的函式，若此時錯誤再次發生則會轉換成「開啟」狀態繼續阻斷所有請求，相反地如果一段時間內沒有任何錯誤發生則會改為「關閉」狀態允許所有請求呼叫該函式。

# 索引

* [安裝方式](#安裝方式)
* [使用方式](#使用方式)
    * [取得狀態](#取得狀態)
	* [取得名稱](#取得名稱)

# 安裝方式

打開終端機並且透過 `go get` 安裝此套件即可。

```bash
$ go get github.com/go-mego/cb
```

# 使用方式

要讓程式受到斷路器的保護很簡單，僅需將邏輯移至斷路器的 `Execute` 內執行並確保最終會回傳一個值和錯誤（若無則空）。

```go
package main

import (
	"io/ioutil"

	"github.com/go-mego/cb"
	"github.com/go-mego/mego"
)

func main() {
	m := mego.New()
	// 透過選項來初始化一個斷路器。
	b := cb.New(cb.Option{
		Name: "我的斷路器",
	})
	m.Get("/", func() string {
		// 可能會發生的錯誤請在 `Execute` 中執行。
		content, err := cb.Execute(func() (interface{}, error) {
			// 每當接收到錯誤，斷路器會增加一次錯誤紀錄，
			// 反之，若無錯誤則是增加一次成功紀錄。
			return ioutil.ReadFile("/tmp/dat")
		})
		// 當斷路器啟動時，`err` 會直接回傳 `cb.ErrOpenState` 且其中程式不會被執行。
		// 而這錯誤也有可能是程式內所回傳的錯誤資料。
		if err != nil {
			return err.Error()
		}
		// 如果無任何錯誤就可以正常繼續執行。
		return string(content.([]byte))
	})
	m.Run()
}
```

## 取得狀態

透過 `State` 可以取得斷路器目前的狀態。

```go
b := cb.New(cb.Option{
	// ...
})
if b.State() == cb.StateOpen {
	fmt.Println("斷路器處於開放狀態！請求都會被阻斷！")
}
```

## 取得名稱

以 `Name` 來取得斷路器的名稱。

```go
b := cb.New(cb.Option{
	Name: "Database",
	// ...
})
fmt.Println(b.Name()) // 結果：Database
```