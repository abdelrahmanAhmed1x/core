package httpx

import (
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBindingAndResponse(t *testing.T) {
	r := gin.New()
	r.POST("/", func(c *gin.Context) {
		_, ok := BindJSON[struct {
			N int `json:"n"`
		}](c)
		if ok {
			t.Error("bad body accepted")
		}
	})
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/", strings.NewReader(`{"n":"secret-value"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != 400 || strings.Contains(w.Body.String(), "secret-value") {
		t.Fatal(w.Code, w.Body.String())
	}
}
func TestNoDuplicateWrite(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	OK(c, 1)
	before := c.Writer.Size()
	AbortInternal(c, nil)
	if c.Writer.Size() != before {
		t.Fatal("duplicate response")
	}
}
