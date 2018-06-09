package main

import (
	"net/http"

	"github.com/go-mego/cb"
	"github.com/go-mego/mego"
)

func main() {
	e := mego.Default()
	e.GET("/", cb.New(), func(c *mego.Context, cb *cb.Breaker) {
		cb.Fail()
		c.String(http.StatusOK, "Circuit breaker failed once.")
	})
	e.Run()
}
