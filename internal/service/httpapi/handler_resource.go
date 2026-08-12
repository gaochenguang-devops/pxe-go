package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"pxe-server/internal/db"
)

// handleListResources 资源列表（供系统镜像的 vmlinuz/initrd 下拉框及 iPXE 渲染使用）。
func (s *Server) handleListResources(c *gin.Context) {
	list, err := db.ListResources()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": list})
}

func parseInt64(s string) int64 {
	var n int64
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0
		}
		n = n*10 + int64(r-'0')
	}
	return n
}

func int64Str(i int64) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var d []byte
	for i > 0 {
		d = append([]byte{byte('0' + i%10)}, d...)
		i /= 10
	}
	if neg {
		return "-" + string(d)
	}
	return string(d)
}
