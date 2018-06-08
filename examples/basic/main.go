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

func main() {
	e := mego.Default()
	e.GET("/", binding.New(User{}), func(c *mego.Context, u *User) {
		c.String(http.StatusOK, "%+v", u)
	})
	e.Run()
}
