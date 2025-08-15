package prometheus

import (
	"fmt"

	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
)

type PrometheusClient struct {
	BaseURL string
	Client  v1.API
}

func NewPrometheusClient(regionCode string) (*PrometheusClient, error) {
	url, err := matchURL(regionCode)
	if err != nil {
		return nil, err
	}
	client, err := api.NewClient(api.Config{Address: url})
	if err != nil {
		return nil, fmt.Errorf("访问Prometheus失败: %w", err)
	}
	v1api := v1.NewAPI(client)
	return &PrometheusClient{
		BaseURL: url,
		Client:  v1api,
	}, nil
}
