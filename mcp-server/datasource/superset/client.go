package superset

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"regexp"
)

// SupersetClient 结构体，包含 Superset 连接所需的所有信息
// 现在推荐通过 config.yaml 配置文件赋值
type SupersetClient struct {
	BaseURL      string
	Username     string
	Password     string
	Client       *http.Client
	CSRFToken    string
	CookieHeader string
}

// NewSupersetClient 创建新的client结构体（对象）
func NewSupersetClient(baseURL, username, password string) *SupersetClient {
	jar, _ := cookiejar.New(nil)
	return &SupersetClient{
		BaseURL:  baseURL,
		Username: username,
		Password: password,
		Client:   &http.Client{Jar: jar},
	}
}

// FetchCSRFToken 登录并从HTML页面中提取CSRF token
func (c *SupersetClient) FetchCSRFToken() error {
	resp, err := c.Client.Get(c.BaseURL + "/login/")
	if err != nil {
		return err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			fmt.Println(err)
			return
		}
	}(resp.Body)

	bodyBytes, _ := io.ReadAll(resp.Body)
	re := regexp.MustCompile(`name="csrf_token" type="hidden" value="([^"]+)"`)
	match := re.FindSubmatch(bodyBytes)
	if len(match) < 2 {
		return fmt.Errorf("无法在登录页中找到 CSRF token")
	}
	c.CSRFToken = string(match[1])
	return nil
}

// Login 使用用户名/密码和CSRF token登录
func (c *SupersetClient) Login() error {
	form := fmt.Sprintf("username=%s&password=%s&csrf_token=%s",
		c.Username, c.Password, c.CSRFToken)

	req, _ := http.NewRequest("POST", c.BaseURL+"/login/", bytes.NewBufferString(form))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Referer", c.BaseURL+"/login/")

	resp, err := c.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 302 && resp.StatusCode != 200 {
		return fmt.Errorf("登录失败，状态码: %d", resp.StatusCode)
	}
	// 提取 Cookie：session + csrf_token
	var session string
	for _, cookie := range c.Client.Jar.Cookies(req.URL) {
		if cookie.Name == "session" {
			session = cookie.Value
		}
	}
	c.CookieHeader = fmt.Sprintf("session=%s", session)

	return nil
}
