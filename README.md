# Binding [![GoDoc](https://godoc.org/github.com/go-mego/binding?status.svg)](https://godoc.org/github.com/go-mego/binding)

套件 Binding 能夠將接收到的資料映射到本地的某個建構體，並且自動驗證且傳入路由中供使用。

# 索引

* [安裝方式](#安裝方式)
* [使用方式](#使用方式)


# 安裝方式

打開終端機並且透過 `go get` 安裝此套件即可。

```bash
$ go get github.com/go-mego/binding
```

# 使用方式

將 `binding.New` 用於指定的路由中，並且帶上欲映射的建構體就可以在路由的處理函式中使用映射的資料。如果欲映射的資料沒有通過驗證或是格式不正確，則會直接回傳 `HTTP 400` 錯誤。

```go
package main

import (
	"github.com/go-mego/binding"
	"github.com/go-mego/mego"
)

// 事先定義一個資料建構體供稍後映射。
type User struct {
	Username string `form:"username" binding:"required"`
}

func main() {
	m := mego.New()
	// 將綁定中介軟體與欲映射的建構體傳入路由，便可在路由中使用。
	// 如果資料不正確或未通過驗證則會直接回傳 HTTP 錯誤。
	m.POST("/", binding.New(User{}), func(u User) string {
		return "已接收到資料，Username：" + u.Username
	})
	m.Run()
}
```