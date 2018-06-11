# Circuit Breaker [![GoDoc](https://godoc.org/github.com/go-mego/cb?status.svg)](https://godoc.org/github.com/go-mego/cb) [![Coverage Status](https://coveralls.io/repos/github/go-mego/cb/badge.svg?branch=master)](https://coveralls.io/github/go-mego/cb?branch=master) [![Build Status](https://travis-ci.org/go-mego/cb.svg?branch=master)](https://travis-ci.org/go-mego/cb) [![Go Report Card](https://goreportcard.com/badge/github.com/go-mego/cb)](https://goreportcard.com/report/github.com/go-mego/cb)

Circuit Breaker 是基於斷路器理念用以防止錯誤不斷地發生的套件。當函式發生錯誤到一定的次數（或比例）就會直接斷路並回傳錯誤訊息，而不是嘗試執行可能發生錯誤的函式以防浪費資源。

過了一段時間後斷路器會呈現「半開放」的狀態並允許嘗試執行可能發生錯誤的函式，若此時錯誤再次發生則會轉換成「開啟」狀態繼續阻斷所有請求，相反地如果一段時間內沒有任何錯誤發生則會改為「關閉」狀態允許所有請求呼叫該函式。

# 索引

* [安裝方式](#安裝方式)
* [使用方式](#使用方式)
    * [保護與設置](#保護與設置)
	* [取得狀態](#取得狀態)
	* [手動操作](#手動操作)
	* [取得名稱](#取得名稱)

# 安裝方式

打開終端機並且透過 `go get` 安裝此套件即可。

```bash
$ go get github.com/go-mego/cb
```

# 使用方式

將 `cb.New` 傳入 Mego 引擎的 `Use` 就能夠作為全域中介軟體，這會讓所有路由共享同一個斷路器。

當任何一個路由持續出錯時，所有路由都會被斷路保護。

```go
package main

import (
	"github.com/go-mego/cb"
)

func main() {
	m := mego.New()
	// 將 Circuit Breaker 作為全域中介軟體即能在不同路由中使用同個斷路器。
	// 當任何一個路由持續出錯時，所有路由都會被斷路保護。
	m.Use(cb.New())
	m.Run()
}
```

Circuit Breaker 也可以獨立用於每個路由，這樣每個路由都有自己的斷路器保護機制。

```go
func main() {
	m := mego.New()
	// 也可以僅將 Circuit Breaker 傳入單個路由中使用。
	m.GET("/", cb.New(), func(c *cb.Crypt) {
		// ...
	})
	m.Run()
}
```

## 保護與設置

斷路器是主動式的。當請求回應錯誤狀態碼時，斷路器會將該請求視為伺服器錯誤而計次。當錯誤至少五次時，斷路器就會開啟並在接下來的 60 秒內拒絕請求，之後會呈現半開放的狀態讓客戶端可重新嘗試執行動作。

如果你希望自訂這背後的行為，在建立斷路器時可以傳入 `&cb.Options` 設置。

```go
func main() {
	m := mego.New()
	m.Use(cb.New(&cb.Options{
		// ...
	}))
	m.Run()
}
```

## 取得狀態

透過 `State` 可以取得斷路器目前的狀態。

```go
func main() {
	b := cb.New()
	if b.State() == cb.StateOpen {
		fmt.Println("斷路器處於開放狀態！請求都會被阻斷！")
	} else {
		fmt.Println("斷路器是關閉狀態。")
	}
}
```

## 手動操作

透過 `Open` 來開啟斷路器，拒絕所有的請求；而 `Close` 可以關閉斷路器讓請求回歸正常執行。

```go
func main() {
	b := cb.New()
	// 手動開啟斷路器，所有允許都會被拒絕。
	b.Open()
	fmt.Println(b.State() == cb.StateOpen) // 結果：true
	// 手動關閉斷路器，這將允許請求執行。
	b.Close()
	fmt.Println(b.State() == cb.StateClose) // 結果：true
}
```

## 取得名稱

以 `Name` 來取得斷路器的名稱。

```go
func main() {
	b := cb.New(&cb.Option{
		Name: "Database",
		// ...
	})
	fmt.Println(b.Name()) // 結果：Database
}
```