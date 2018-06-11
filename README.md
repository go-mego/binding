# Binding [![GoDoc](https://godoc.org/github.com/go-mego/binding?status.svg)](https://godoc.org/github.com/go-mego/binding) [![Coverage Status](https://coveralls.io/repos/github/go-mego/binding/badge.svg?branch=master)](https://coveralls.io/github/go-mego/binding?branch=master) [![Build Status](https://travis-ci.org/go-mego/binding.svg?branch=master)](https://travis-ci.org/go-mego/binding) [![Go Report Card](https://goreportcard.com/badge/github.com/go-mego/binding)](https://goreportcard.com/report/github.com/go-mego/binding)

套件 Binding 能夠將接收到的資料映射到本地的某個建構體，並且自動驗證且傳入路由中供使用。

# 索引

* [安裝方式](#安裝方式)
* [使用方式](#使用方式)
	* [多重映射](#多重映射)
	* [必填欄位](#必填欄位)
	* [省略欄位](#省略欄位)
	* [命名欄位](#命名欄位)

# 安裝方式

打開終端機並且透過 `go get` 安裝此套件即可。

```bash
$ go get github.com/go-mego/binding
```

# 使用方式

將 `binding.New` 用於指定的路由中，並且帶上欲映射的建構體就可以在路由的處理函式中使用映射的資料。這支援映射下列請求內容。

* 網址參數表單：`application/x-www-form-urlencoded`
* 標準表單：`multipart/form-data`
* JSON 表單：`application/json`

```go
package main

import (
	"github.com/go-mego/binding"
	"github.com/go-mego/mego"
)

// 事先定義一個資料建構體供稍後映射。
type User struct {
	Username string
}

func main() {
	m := mego.New()
	// 將綁定中介軟體與欲映射的建構體傳入路由，便可在路由中使用。
	// 如果資料不正確或未通過驗證則會直接回傳 HTTP 錯誤。
	m.POST("/", binding.New(User{}), func(c *mego.Context, u *User) {
		c.String(http.StatusOK, "已接收到資料，Username：%s", u.Username)
	})
	m.Run()
}
```

## 多重映射

透過 `binding.New` 預設會映射請求中的一個來源（例如：表單）。如果沒有表單資料則以網址參數為主，倘若你希望能夠映射表單內容的同時也映射網址參數至另一個建構體，這個時候可以透過 `binding.NewQuery`。

```go
func main() {
	m := mego.Default()
	// 將網址參數映射到 `Query` 建構體，而表單內容則映射到 `User` 建構體。
	m.POST("/", binding.NewQuery(Query{}), binding.New(User{}), func(c *mego.Context, q *Query, u *User) {
		c.String(http.StatusOK, "Query: %+v\nUser: %+v", q, u)
	})
	m.Run()
}
```

## 必填欄位

在建構體欄位中以 `binding:"required"` 標籤標註，可以讓一個欄位成為必要欄位。當該欄位是零值時，Binding 模組會向上下文建構體中保存映射錯誤，如果這個錯誤不處理則會自動在最後向客戶端發送 HTTP 400 錯誤。

```go
// 讓欄位中的 `Username` 成為必填欄位，若該值為空則會回報錯誤。
type User struct {
	Username string `binding:"required"`
}
```

## 省略欄位

如果建構體中有個欄位是不希望被映射的，透過 `form:"-"` 或 `json:"-"`（如果請求是 JSON 格式的話）略過該欄位。

```go
// 建構體中的 `Username` 欄位不會被映射。
type User struct {
	Username string `form:"-"`
	Password string
}
```

## 命名欄位

Binding 模組會自動將請求資料的 `-`、`_`（分隔符號）移除，並且統一大小寫來自動對應 Golang 程式中的建構體欄位名稱。但是在請求欄位與本地建構體欄位名稱差異甚大時，則可以透過 `form:"myField"` 或 或 `json:"myField"`（如果請求是 JSON 格式的話）來重新對應

```go
// 建構體中的 `CreatedAt` 欄位實際上會對應請求中的 `registration_date` 欄位。
type User struct {
	CreatedAt time.Time `form:"registration_date"`
}
```