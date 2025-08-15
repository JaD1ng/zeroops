package superset

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// FetchBySQL 通过SQL语句获取数据
func (c *SupersetClient) FetchBySQL(sql, schema string, databaseID int) ([]byte, error) {
	url := c.BaseURL + "/api/v1/sqllab/execute/"
	payload := map[string]interface{}{
		"sql":         sql,
		"database_id": databaseID,
		"schema":      schema,
	}

	body, _ := json.Marshal(payload)

	req, _ := http.NewRequest("POST", url, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRFToken", c.CSRFToken)
	req.Header.Set("Cookie", c.CookieHeader)
	req.Header.Set("Referer", c.BaseURL+"/sqllab")

	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		fmt.Println("请求失败状态码:", resp.StatusCode)
		fmt.Println("响应内容:", string(respBytes))
		return nil, fmt.Errorf("查询失败: %s", respBytes)
	}
	return respBytes, nil
}
