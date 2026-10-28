package bingfa

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

// 模拟http并发请求 -- 秒杀并发测试
func TestSecKill(t *testing.T) {
	var wg sync.WaitGroup
	url := "http://localhost:8080/activity/secKill/start"
	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r := rand.New(rand.NewSource(time.Now().UnixNano()))
			contentType := "application/json"
			data := map[string]interface{}{
				"product_id": 11,
				"user_id":    r.Intn(1000),
				"count":      r.Intn(20),
			}
			dataJson, _ := json.Marshal(data)
			resp, err := http.Post(url, contentType, strings.NewReader(string(dataJson)))
			if err != nil {
				fmt.Println("err:", err)
				return
			}
			defer resp.Body.Close()
			fmt.Println(resp.StatusCode)
		}()
	}
	wg.Wait()
}
