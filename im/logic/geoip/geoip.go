package geoip

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

const (
	ipAPIBase = "http://ip-api.com/json"
	timeout   = 3 * time.Second
)

type ipAPIResponse struct {
	Status     string `json:"status"`
	Country    string `json:"country"`
	RegionName string `json:"regionName"`
	City       string `json:"city"`
	Isp        string `json:"isp"`
}

// Lookup 根据 IP 查询国家、城市+区域、网络服务商；失败或超时返回空字符串，不阻塞调用方
func Lookup(ip string, logger *zerolog.Logger) (country, cityRegion, isp string) {
	logger.Info().Str("ip", ip).Msg("geoip Lookup")
	if ip == "" {
		return "", "", ""
	}
	ip = strings.TrimSpace(ip)
	if isPrivateOrLocal(ip) {
		logger.Debug().Str("ip", ip).Msg("skip geo lookup for private/local IP")
		return "", "", ""
	}
	url := fmt.Sprintf("%s/%s?fields=status,country,regionName,city,isp", ipAPIBase, ip)
	logger.Info().Str("url", url).Str("ip", ip).Msg("geoip lookup request")
	client := &http.Client{Timeout: timeout}
	resp, err := client.Get(url)
	if err != nil {
		logger.Warn().Err(err).Str("ip", ip).Str("url", url).Msg("geoip lookup request failed")
		return "", "", ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logger.Warn().Int("status", resp.StatusCode).Str("ip", ip).Msg("geoip lookup non-200")
		return "", "", ""
	}
	var body ipAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		logger.Warn().Err(err).Str("ip", ip).Msg("geoip decode response failed")
		return "", "", ""
	}
	if body.Status != "success" {
		logger.Debug().Str("ip", ip).Str("status", body.Status).Msg("geoip lookup not success")
		return "", "", ""
	}
	country = strings.TrimSpace(body.Country)
	cityRegion = buildCityRegion(body.City, body.RegionName)
	isp = strings.TrimSpace(body.Isp)
	logger.Info().Str("ip", ip).Str("country", country).Str("cityRegion", cityRegion).Str("isp", isp).Msg("geoip lookup success")
	return country, cityRegion, isp
}

func buildCityRegion(city, regionName string) string {
	city = strings.TrimSpace(city)
	regionName = strings.TrimSpace(regionName)
	if city == "" && regionName == "" {
		return ""
	}
	if city == "" {
		return regionName
	}
	if regionName == "" {
		return city
	}
	return city + ", " + regionName
}

func isPrivateOrLocal(ip string) bool {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}
	return parsed.IsPrivate() || parsed.IsLoopback()
}
