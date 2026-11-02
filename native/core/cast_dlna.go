package core

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

// 投屏（DLNA）：SSDP 发现设备 + SOAP 投当前视频（AVTransport SetAVTransportURI）。
// 纯本地实现，无第三方依赖：电视/盒子最常见的通用协议。

const castSSDPAddress = "239.255.255.250:1900"

const castSSDPProbe = "M-SEARCH * HTTP/1.1\r\n" +
	"HOST: 239.255.255.250:1900\r\n" +
	"MAN: \"ssdp:discover\"\r\n" +
	"MX: 2\r\n" +
	"ST: ssdp:all\r\n\r\n"

var castUSNPattern = regexp.MustCompile(`(?im)^USN:\s*(.+)$`)
var castLocationPattern = regexp.MustCompile(`(?im)^LOCATION:\s*(.+)$`)
var castServerPattern = regexp.MustCompile(`(?im)^SERVER:\s*(.+)$`)
var castSTPattern = regexp.MustCompile(`(?im)^ST:\s*(.+)$`)

// castDevice 一台可投屏的 DLNA 渲染器。
type castDevice struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Location string `json:"location"`
	Server   string `json:"server,omitempty"`
	// SOAP 控制端点（从设备描述 XML 解析）。
	ControlURL string `json:"controlUrl"`
}

func (d castDevice) describe() string {
	if d.Server != "" {
		return d.Server
	}
	return "DLNA"
}

// discoverCastDevices 在局域网内 SSDP 搜索 DLNA 媒体渲染器。
func discoverCastDevices(ctx context.Context) ([]castDevice, error) {
	conn, err := net.ListenPacket("udp4", ":0")
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(2600 * time.Millisecond))

	target, err := net.ResolveUDPAddr("udp4", castSSDPAddress)
	if err != nil {
		return nil, err
	}
	if _, err := conn.WriteTo([]byte(castSSDPProbe), target); err != nil {
		return nil, err
	}

	seen := map[string]castDevice{}
	buf := make([]byte, 4096)
	for {
		if ctx.Err() != nil {
			break
		}
		n, _, err := conn.ReadFrom(buf)
		if err != nil {
			break // 超时：发现窗口结束
		}
		response := string(buf[:n])
		if !strings.Contains(response, "HTTP/1.1 200") && !strings.Contains(response, "NOTIFY") {
			continue
		}
		location := firstMatch(castLocationPattern, response)
		if location == "" {
			continue
		}
		usn := firstMatch(castUSNPattern, response)
		if usn == "" {
			usn = location
		}
		if _, dup := seen[usn]; dup {
			continue
		}
		device := castDevice{
			ID:       usn,
			Location: location,
			Server:   firstMatch(castServerPattern, response),
		}
		seen[usn] = device
	}

	devices := make([]castDevice, 0, len(seen))
	for _, device := range seen {
		if parsed, err := loadCastDeviceDescription(ctx, device); err == nil {
			devices = append(devices, parsed)
		}
	}
	sort.Slice(devices, func(i, j int) bool { return devices[i].Name < devices[j].Name })
	return devices, nil
}

// castDeviceDescription 设备描述 XML 的相关片段。
type castDeviceDescription struct {
	XMLName     xml.Name `xml:"root"`
	Device      castDeviceDescriptionDevice `xml:"device"`
}
type castDeviceDescriptionDevice struct {
	FriendlyName string `xml:"friendlyName"`
	Services     []struct {
		ServiceType string `xml:"serviceType"`
		ControlURL  string `xml:"controlURL"`
	} `xml:"serviceList>service"`
}

// loadCastDeviceDescription 拉取设备描述 XML，取名称并解析 AVTransport 控制端点。
func loadCastDeviceDescription(ctx context.Context, device castDevice) (castDevice, error) {
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, device.Location, nil)
	if err != nil {
		return device, err
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return device, err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return device, err
	}
	if response.StatusCode != http.StatusOK {
		return device, fmt.Errorf("设备描述 HTTP %d", response.StatusCode)
	}
	var description castDeviceDescription
	if err := xml.Unmarshal(body, &description); err != nil {
		return device, err
	}
	avTransport := ""
	for _, service := range description.Device.Services {
		if strings.Contains(service.ServiceType, "AVTransport") {
			avTransport = service.ControlURL
			break
		}
	}
	if avTransport == "" {
		return device, errors.New("该设备不支持 DLNA 投屏（无 AVTransport）")
	}
	base, err := url.Parse(device.Location)
	if err != nil {
		return device, err
	}
	ref, err := url.Parse(avTransport)
	if err != nil {
		return device, err
	}
	device.ControlURL = base.ResolveReference(ref).String()
	name := strings.TrimSpace(description.Device.FriendlyName)
	if name == "" {
		name = "DLNA 设备"
	}
	device.Name = name
	return device, nil
}

// castToDevice 用 SOAP SetAVTransportURI 把地址投到设备（并自动播放）。
func castToDevice(ctx context.Context, device castDevice, mediaURL string, title, headers string) error {
	if device.ControlURL == "" {
		return errors.New("投屏设备未就绪")
	}
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()

	body := buildCastSOAPBody(mediaURL, title)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, device.ControlURL, strings.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "text/xml; charset=\"utf-8\"")
	request.Header.Set("SOAPACTION", `"urn:schemas-upnp-org:service:AVTransport:1#SetAVTransportURI"`)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	_, _ = io.ReadAll(io.LimitReader(response.Body, 1<<18))
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("投屏指令失败 HTTP %d", response.StatusCode)
	}
	// 投上后自动 Play（部分电视机需要显式 PLAY）。
	_ = sendCastPlay(ctx, device)
	return nil
}

func sendCastPlay(ctx context.Context, device castDevice) error {
	body := `<?xml version="1.0" encoding="utf-8"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/">
  <s:Body>
    <u:Play xmlns:u="urn:schemas-upnp-org:service:AVTransport:1">
      <InstanceID>0</InstanceID>
      <Speed>1</Speed>
    </u:Play>
  </s:Body>
</s:Envelope>`
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, device.ControlURL, strings.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "text/xml; charset=\"utf-8\"")
	request.Header.Set("SOAPACTION", `"urn:schemas-upnp-org:service:AVTransport:1#Play"`)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	_, _ = io.ReadAll(io.LimitReader(response.Body, 1<<18))
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("投屏播放指令失败 HTTP %d", response.StatusCode)
	}
	return nil
}

func buildCastSOAPBody(mediaURL, title string) string {
	if title == "" {
		title = "视频"
	}
	escapedURL := xmlEscape(mediaURL)
	return `<?xml version="1.0" encoding="utf-8"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/">
  <s:Body>
    <u:SetAVTransportURI xmlns:u="urn:schemas-upnp-org:service:AVTransport:1">
      <InstanceID>0</InstanceID>
      <CurrentURI>` + escapedURL + `</CurrentURI>
      <CurrentURIMetaData>&lt;DIDL-Lite xmlns=&quot;urn:schemas-upnp-org:metadata-1-0/DIDL-Lite/&quot;&gt;&lt;item&gt;&lt;dc:title xmlns:dc=&quot;http://purl.org/dc/elements/1.1/&quot;&gt;` + xmlEscape(title) + `&lt;/dc:title&gt;&lt;/item&gt;&lt;/DIDL-Lite&gt;</CurrentURIMetaData>
    </u:SetAVTransportURI>
  </s:Body>
</s:Envelope>`
}

func xmlEscape(text string) string {
	var b strings.Builder
	if err := xml.EscapeText(&b, []byte(text)); err != nil {
		return text
	}
	return b.String()
}

func firstMatch(pattern *regexp.Regexp, text string) string {
	match := pattern.FindStringSubmatch(text)
	if len(match) < 2 {
		return ""
	}
	return strings.TrimSpace(match[1])
}

// castEngine 需要把本机播放地址转成局域网可访问的（本机起临时 HTTP 服务把流转发给电视）。
var castServeOnce sync.Once
var castServeMux sync.Mutex
var castStreams = map[string]func(w http.ResponseWriter, r *http.Request){}

// registerCastStream 登记一条可被电视拉取的转发流，返回局域网 URL。
func registerCastStream(key string, handler func(w http.ResponseWriter, r *http.Request)) (string, error) {
	castServeOnce.Do(func() {
		go func() {
			listener, err := net.Listen("tcp", ":0")
			if err != nil {
				return
			}
			server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				path := strings.TrimPrefix(r.URL.Path, "/cast/")
				castServeMux.Lock()
				handler, ok := castStreams[path]
				castServeMux.Unlock()
				if ok {
					handler(w, r)
					return
				}
				http.NotFound(w, r)
			})}
			_ = server.Serve(listener)
		}()
	})
	castServeMux.Lock()
	castStreams[key] = handler
	castServeMux.Unlock()
	return "/cast/" + key, nil
}

// nativeCastDiscover 桥接：搜索局域网 DLNA 设备并返回列表。
func (engine *nativeEngine) nativeCastDiscover(ctx context.Context) (map[string]any, error) {
	devices, err := discoverCastDevices(ctx)
	if err != nil {
		return nil, err
	}
	rows := make([]map[string]any, 0, len(devices))
	for _, device := range devices {
		rows = append(rows, map[string]any{
			"id":  device.ID,
			"name": device.Name,
			"server": device.Server,
		})
	}
	return map[string]any{"items": rows}, nil
}

// nativeCastStart 桥接：把媒体地址投到选定设备。
// input.CastTarget 为 castDevice JSON；MediaURL 为可被电视直接拉取的地址。
func (engine *nativeEngine) nativeCastStart(ctx context.Context, input nativeInput) (map[string]any, error) {
	var device castDevice
	if len(input.CastTarget) > 0 {
		if err := json.Unmarshal(input.CastTarget, &device); err != nil {
			return nil, errors.New("投屏设备信息无效")
		}
	}
	if input.MediaURL == "" {
		return nil, errors.New("投屏地址无效")
	}
	// 授权：需要本机已有该媒体的播放会话或本地址可播放。
	if err := nativeAuthorizeInput(input); err != nil {
		return nil, err
	}
	if device.ControlURL == "" {
		parsed, err := loadCastDeviceDescription(ctx, device)
		if err != nil {
			return nil, fmt.Errorf("投屏设备不可用: %w", err)
		}
		device = parsed
	}
	if err := castToDevice(ctx, device, input.MediaURL, input.CastTitle, ""); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "device": device.Name}, nil
}

// NewCastDescriptionServerForTest 测试辅助：起一个最小 HTTP 服务返回给定 XML。
func NewCastDescriptionServerForTest(t interface {
	Fatalf(string, ...any)
	Logf(string, ...any)
	Cleanup(func())
}, xml string) (server struct {
	URL   string
	Close func()
}) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/dd.xml", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(xml))
	})
	httpServer := &http.Server{Handler: mux}
	go func() { _ = httpServer.Serve(ln) }()
	t.Cleanup(func() { _ = httpServer.Close() })
	server.URL = "http://" + ln.Addr().String()
	server.Close = func() { _ = httpServer.Close() }
	return server
}
