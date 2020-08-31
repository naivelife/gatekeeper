package util

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

func HttpGET(urlString string, urlParams url.Values, msTimeout int, header http.Header) (*http.Response, []byte, error) {
	client := http.Client{
		Timeout: time.Duration(msTimeout) * time.Millisecond,
	}
	urlString = AddGetDataToUrl(urlString, urlParams)
	req, err := http.NewRequest("GET", urlString, nil)
	if err != nil {
		return nil, nil, err
	}
	if len(header) > 0 {
		req.Header = header
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}
	return resp, body, nil
}

func AddGetDataToUrl(urlString string, data url.Values) string {
	if strings.Contains(urlString, "?") {
		urlString = urlString + "&"
	} else {
		urlString = urlString + "?"
	}
	return fmt.Sprintf("%s%s", urlString, data.Encode())
}

func Encode(data string) (string, error) {
	h := md5.New()
	_, err := h.Write([]byte(data))
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func GetLocalIPs() (ips []net.IP) {
	interfaceAddr, err := net.InterfaceAddrs()
	if err != nil {
		return nil
	}
	for _, address := range interfaceAddr {
		ipNet, isValidIpNet := address.(*net.IPNet)
		if isValidIpNet && !ipNet.IP.IsLoopback() {
			if ipNet.IP.To4() != nil {
				ips = append(ips, ipNet.IP)
			}
		}
	}
	return ips
}

//AuthIPList 验证ip名单
func AuthIPList(clientIP string, whiteList []string) bool {
	return InStringList(clientIP, whiteList)
}

//InStringList 数组中是否存在某值
func InStringList(t string, list []string) bool {
	for _, s := range list {
		if s == t {
			return true
		}
	}
	return false
}

//InOrPrefixStringList 字符串在string数组 或者 字符串前缀在数组中
func InOrPrefixStringList(t string, arr []string) bool {
	for _, s := range arr {
		if t == s {
			return true
		}
		if s != "" && strings.HasPrefix(t, s) {
			return true
		}
	}
	return false
}

//Substr 字符串的截取
func Substr(str string, start int64, end int64) string {
	length := int64(len(str))
	if start < 0 || start > length {
		return ""
	}
	if end < 0 {
		return ""
	}
	if end > length {
		end = length
	}
	return string(str[start:end])
}

//MapSorter map排序，按key排序
type MapSorter []MapItem

//NewMapSorter 新排序
func NewMapSorter(m map[string]string) MapSorter {
	ms := make(MapSorter, 0, len(m))
	for k, v := range m {
		ms = append(ms, MapItem{Key: k, Val: v})
	}
	sort.Sort(ms)
	return ms
}

//MapItem 排序对象
type MapItem struct {
	Key string
	Val string
}

//Len 对象长度
func (ms MapSorter) Len() int {
	return len(ms)
}

//Swap 交换位置
func (ms MapSorter) Swap(i, j int) {
	ms[i], ms[j] = ms[j], ms[i]
}

//Less 按首字母键排序
func (ms MapSorter) Less(i, j int) bool {
	return ms[i].Key < ms[j].Key
}

//GetSign 获取签名
func GetSign(paramMap map[string]string, secret string) string {
	paramArr := NewMapSorter(paramMap)
	str := ""
	for _, v := range paramArr {
		str = str + fmt.Sprintf("%s=%s&", v.Key, url.QueryEscape(v.Val))
	}
	str = str + secret

	h := md5.New()
	h.Write([]byte(str))
	cipherStr := h.Sum(nil)
	md5Str := hex.EncodeToString(cipherStr)
	return md5Str[7:23]
}

//RemoteIP 获取远程IP
func RemoteIP(req *http.Request) string {
	var err error
	var remoteAddr = req.RemoteAddr
	if ip := req.Header.Get("X-Real-IP"); ip != "" {
		remoteAddr = ip
	} else if ip = req.Header.Get("X-Forwarded-For"); ip != "" {
		remoteAddr = ip
	} else {
		remoteAddr, _, err = net.SplitHostPort(remoteAddr)
	}
	if err != nil {
		return ""
	}
	if remoteAddr == "::1" {
		remoteAddr = "127.0.0.1"
	}
	return remoteAddr
}

func CheckConnPort(port string) error {
	ln, err := net.Listen("tcp", port)
	if err != nil {
		return err
	}
	ln.Close()
	return nil
}
