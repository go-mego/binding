package main

import (
	"net/http"

	"github.com/go-mego/binding"
	"github.com/go-mego/mego"
)

type User struct {
	Username string
	Password string
}

type Query struct {
	Page    int
	OrderBy string
}

func main() {
	e := mego.Default()
	e.POST("/", binding.NewQuery(Query{}), binding.New(User{}), func(c *mego.Context, q *Query, u *User) {
		c.String(http.StatusOK, "Query: %+v\nUser: %+v", q, u)
	})
	e.Run()
}
