package main

import (
	"net/http"

	"github.com/go-mego/cb"
	"github.com/go-mego/mego"
)

func main() {
	e := mego.Default()
	e.GET("/", cb.New(), func(c *mego.Context, cb *cb.Breaker) {
		c.String(http.StatusInternalServerError, "Internal server error.")
	})
	e.Run()
}
