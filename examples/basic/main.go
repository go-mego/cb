package main

import (
	"net/http"

	"github.com/go-mego/cb"
	"github.com/go-mego/mego"
)

func main() {
	e := mego.Default()
	b := cb.New()
	e.GET("/fail", b, func(c *mego.Context, cb *cb.Breaker) {
		c.String(http.StatusInternalServerError, "%+v, %+v", cb.Counts(), cb.State().String())
	})
	e.GET("/success", b, func(c *mego.Context, cb *cb.Breaker) {
		c.String(http.StatusOK, "%+v, %+v", cb.Counts(), cb.State().String())
	})
	e.Run()
}
